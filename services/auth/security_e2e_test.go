package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/Ajay01103/go-notion/auth/gen/pb"
	"github.com/Ajay01103/go-notion/auth/gen/pb/pbconnect"
	"github.com/redis/go-redis/v9"
)

func TestReplayGraceWindowThenNukeAfterGraceExpiry(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client, rdb := setupE2E(t)

	email, name, password := uniqueIdentity("replay")
	regResp, err := client.Register(ctx, connect.NewRequest(&pb.RegisterRequest{
		Email:    email,
		Name:     name,
		Password: password,
	}))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	baseRefresh := regResp.Msg.GetRefreshToken()
	claims, err := parseJWTClaims(baseRefresh)
	if err != nil {
		t.Fatalf("parse refresh claims: %v", err)
	}
	familyID, _ := claims["family_id"].(string)
	if familyID == "" {
		t.Fatalf("family_id missing in refresh token claims")
	}

	firstRefreshResp, err := client.RefreshToken(ctx, connect.NewRequest(&pb.RefreshTokenRequest{
		RefreshToken: baseRefresh,
	}))
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	rotatedRefresh := firstRefreshResp.Msg.GetRefreshToken()
	if rotatedRefresh == "" {
		t.Fatalf("first refresh did not return rotated token")
	}

	_, err = client.RefreshToken(ctx, connect.NewRequest(&pb.RefreshTokenRequest{RefreshToken: baseRefresh}))
	if !connectErrorContains(err, connect.CodeUnauthenticated, "token has expired") &&
		!connectErrorContains(err, connect.CodeUnauthenticated, "refresh token reuse detected") {
		requireConnectErrorContains(t, err, connect.CodeUnauthenticated, "token has expired")
	}

	activeKey := fmt.Sprintf("rt:{%s}:active", familyID)
	active, err := rdb.Get(ctx, activeKey).Result()
	if err != nil && err != redis.Nil {
		t.Fatalf("redis get active key failed: %v", err)
	}
	if active == "" {
		_, err = client.RefreshToken(ctx, connect.NewRequest(&pb.RefreshTokenRequest{RefreshToken: rotatedRefresh}))
		requireConnectErrorContains(t, err, connect.CodeUnauthenticated, "refresh token reuse detected")
		return
	}

	_, err = client.RefreshToken(ctx, connect.NewRequest(&pb.RefreshTokenRequest{RefreshToken: rotatedRefresh}))
	if err != nil {
		t.Fatalf("refresh with rotated token failed during grace window: %v", err)
	}

	time.Sleep(16 * time.Second)

	_, err = client.RefreshToken(ctx, connect.NewRequest(&pb.RefreshTokenRequest{RefreshToken: baseRefresh}))
	requireConnectErrorContains(t, err, connect.CodeUnauthenticated, "refresh token reuse detected")

	active, err = rdb.Get(ctx, activeKey).Result()
	if err != nil && err != redis.Nil {
		t.Fatalf("redis get active key after grace expiry replay failed: %v", err)
	}
	if active != "" {
		t.Fatalf("expected active family key to be removed after grace expiry replay, got value: %s", active)
	}

	_, err = client.RefreshToken(ctx, connect.NewRequest(&pb.RefreshTokenRequest{RefreshToken: rotatedRefresh}))
	requireConnectErrorContains(t, err, connect.CodeUnauthenticated, "refresh token reuse detected")
}

func TestMissingFamilyDoesNotNukeOnRefresh(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, rdb := setupE2E(t)

	email, name, password := uniqueIdentity("missing-family")
	regResp, err := client.Register(ctx, connect.NewRequest(&pb.RegisterRequest{
		Email:    email,
		Name:     name,
		Password: password,
	}))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	refreshToken := regResp.Msg.GetRefreshToken()
	claims, err := parseJWTClaims(refreshToken)
	if err != nil {
		t.Fatalf("parse refresh claims: %v", err)
	}
	familyID, _ := claims["family_id"].(string)
	if familyID == "" {
		t.Fatalf("family_id missing in refresh token claims")
	}

	activeKey := fmt.Sprintf("rt:{%s}:active", familyID)
	if err := rdb.Del(ctx, activeKey).Err(); err != nil {
		t.Fatalf("delete active family key failed: %v", err)
	}

	_, err = client.RefreshToken(ctx, connect.NewRequest(&pb.RefreshTokenRequest{RefreshToken: refreshToken}))
	requireConnectErrorContains(t, err, connect.CodeUnauthenticated, "refresh token family missing")

	blacklistKey := "rt:blacklist:" + hashSHA256(refreshToken)
	blacklisted, err := rdb.Exists(ctx, blacklistKey).Result()
	if err != nil {
		t.Fatalf("check blacklist key failed: %v", err)
	}
	if blacklisted != 0 {
		t.Fatalf("expected refresh token to remain unblacklisted, got blacklist key %s", blacklistKey)
	}

	userFamiliesKey := fmt.Sprintf("rt:user:%s:families", regResp.Msg.GetUserId())
	member, err := rdb.SIsMember(ctx, userFamiliesKey, familyID).Result()
	if err != nil {
		t.Fatalf("check user family membership failed: %v", err)
	}
	if !member {
		t.Fatalf("expected family %s to remain in user family set %s", familyID, userFamiliesKey)
	}
}

func setupE2E(t *testing.T) (pbconnect.AuthServiceClient, *redis.Client) {
	t.Helper()

	baseURL := os.Getenv("AUTH_E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:50051"
	}

	redisURL := os.Getenv("AUTH_E2E_REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://:redispass@localhost:6379/0"
	}

	resp, err := http.Get(baseURL + "/.well-known/jwks.json")
	if err != nil {
		t.Skipf("auth service not reachable at %s: %v", baseURL, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("auth service jwks endpoint unhealthy at %s: status=%d", baseURL, resp.StatusCode)
	}

	rdbOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("invalid redis url %s: %v", redisURL, err)
	}
	rdb := redis.NewClient(rdbOpts)
	if pingErr := rdb.Ping(context.Background()).Err(); pingErr != nil {
		t.Skipf("redis not reachable at %s: %v", redisURL, pingErr)
	}

	client := pbconnect.NewAuthServiceClient(http.DefaultClient, baseURL)
	return client, rdb
}

func uniqueIdentity(prefix string) (email, name, password string) {
	ns := time.Now().UnixNano()
	email = fmt.Sprintf("%s.%d@example.com", prefix, ns)
	name = fmt.Sprintf("%s-%d", prefix, ns)
	password = "Passw0rd!234"
	return
}

func parseJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid jwt format")
	}
	payload := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode jwt payload: %w", err)
	}
	claims := make(map[string]any)
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal jwt payload: %w", err)
	}
	return claims, nil
}

func hashSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func requireConnectErrorContains(t *testing.T, err error, code connect.Code, contains string) {
	t.Helper()
	if connectErrorContains(err, code, contains) {
		return
	}

	if err == nil {
		t.Fatalf("expected error containing %q, got nil", contains)
	}
	cerr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T (%v)", err, err)
	}
	if cerr.Code() != code {
		t.Fatalf("expected connect code %s, got %s (%v)", code, cerr.Code(), cerr)
	}
	t.Fatalf("expected error message to contain %q, got %q", contains, cerr.Message())
}

func connectErrorContains(err error, code connect.Code, contains string) bool {
	if err == nil {
		return false
	}

	cerr, ok := err.(*connect.Error)
	if !ok {
		return false
	}
	if cerr.Code() != code {
		return false
	}

	return strings.Contains(strings.ToLower(cerr.Message()), strings.ToLower(contains))
}
