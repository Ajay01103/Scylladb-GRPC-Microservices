package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestRemoteValidator_VerifyAccessToken_WithEdDSA(t *testing.T) {
	edPub, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	edJWK := JWKSKey{
		KTY: "OKP",
		Use: "sig",
		KID: "eddsa-1",
		CRV: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(edPub),
		Alg: jwt.SigningMethodEdDSA.Alg(),
	}

	srv := newJWKSHTTPServer(t, JWKSResponse{Keys: []JWKSKey{edJWK}})
	defer srv.Close()

	validator := NewRemoteValidator(srv.URL)
	validator.SetCacheTTL(time.Hour)

	edToken := mustSignedAccessToken(t, jwt.SigningMethodEdDSA, edPriv, "eddsa-1")
	edPayload, err := validator.VerifyAccessToken(edToken)
	if err != nil {
		t.Fatalf("verify eddsa access token: %v", err)
	}
	if edPayload.KeyID != "eddsa-1" {
		t.Fatalf("expected eddsa kid eddsa-1, got %s", edPayload.KeyID)
	}
}

func TestRemoteValidator_VerifyRefreshToken_WithEdDSA(t *testing.T) {
	edPub, edPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	edJWK := JWKSKey{
		KTY: "OKP",
		Use: "sig",
		KID: "eddsa-refresh",
		CRV: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(edPub),
		Alg: jwt.SigningMethodEdDSA.Alg(),
	}

	srv := newJWKSHTTPServer(t, JWKSResponse{Keys: []JWKSKey{edJWK}})
	defer srv.Close()

	validator := NewRemoteValidator(srv.URL)
	validator.SetCacheTTL(time.Hour)

	refreshToken := mustSignedRefreshToken(t, jwt.SigningMethodEdDSA, edPriv, "eddsa-refresh")
	payload, err := validator.VerifyRefreshToken(refreshToken)
	if err != nil {
		t.Fatalf("verify refresh token: %v", err)
	}
	if payload.KeyID != "eddsa-refresh" {
		t.Fatalf("expected refresh kid eddsa-refresh, got %s", payload.KeyID)
	}
}

func TestRemoteValidator_RejectsRS256Token_WhenEdDSAOnlyJWKS(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	edPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	edJWK := JWKSKey{
		KTY: "OKP",
		Use: "sig",
		KID: "eddsa-only",
		CRV: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(edPub),
		Alg: jwt.SigningMethodEdDSA.Alg(),
	}

	srv := newJWKSHTTPServer(t, JWKSResponse{Keys: []JWKSKey{edJWK}})
	defer srv.Close()

	validator := NewRemoteValidator(srv.URL)
	rsaToken := mustSignedAccessToken(t, jwt.SigningMethodRS256, rsaKey, "rsa-legacy")

	_, err = validator.VerifyAccessToken(rsaToken)
	if err == nil {
		t.Fatal("expected RS256 token to be rejected when validator is EdDSA-only")
	}
}

func mustSignedAccessToken(t *testing.T, method jwt.SigningMethod, signer interface{}, kid string) string {
	t.Helper()

	now := time.Now().UTC()
	payload, err := NewAccessPayloadAt(
		uuid.New(),
		"test@example.com",
		"Test User",
		uuid.New(),
		uuid.New(),
		now,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("new access payload: %v", err)
	}

	claims := &AccessTokenClaims{}
	payload.FillClaims(claims)

	tok := jwt.NewWithClaims(method, claims)
	tok.Header["kid"] = kid

	signed, err := tok.SignedString(signer)
	if err != nil {
		t.Fatalf("sign access token: %v", err)
	}
	return signed
}

func mustSignedRefreshToken(t *testing.T, method jwt.SigningMethod, signer interface{}, kid string) string {
	t.Helper()

	now := time.Now().UTC()
	payload, err := NewRefreshPayloadAt(
		uuid.New(),
		"test@example.com",
		"Test User",
		uuid.New(),
		now,
		24*time.Hour,
	)
	if err != nil {
		t.Fatalf("new refresh payload: %v", err)
	}

	claims := &RefreshTokenClaims{}
	payload.FillClaims(claims)

	tok := jwt.NewWithClaims(method, claims)
	tok.Header["kid"] = kid

	signed, err := tok.SignedString(signer)
	if err != nil {
		t.Fatalf("sign refresh token: %v", err)
	}
	return signed
}

func newJWKSHTTPServer(t *testing.T, response JWKSResponse) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("encode jwks: %v", err)
		}
	}))
}
