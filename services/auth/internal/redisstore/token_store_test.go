package redisstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T) (*TokenStore, func()) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cleanup := func() {
		_ = client.Close()
		mr.Close()
	}

	return New(client), cleanup
}

func TestStoreFamilyActiveToken_UserFamiliesSetDoesNotExpire(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-1"
	familyID := "fam-1"
	activeTTL := 5 * time.Minute

	rec := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  "old-hash",
		JKT:        "thumb-1",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-1",
	}

	if err := store.StoreFamilyActiveToken(ctx, familyID, rec, activeTTL); err != nil {
		t.Fatalf("store family active token: %v", err)
	}

	activeKeyTTL, err := store.client.TTL(ctx, activeFamilyKey(familyID)).Result()
	if err != nil {
		t.Fatalf("ttl active family key: %v", err)
	}
	if activeKeyTTL <= 0 {
		t.Fatalf("expected active family key to have ttl, got %v", activeKeyTTL)
	}

	userSetTTL, err := store.client.TTL(ctx, userFamiliesKey(userID)).Result()
	if err != nil {
		t.Fatalf("ttl user family set: %v", err)
	}
	if userSetTTL != -1 {
		t.Fatalf("expected user family set ttl -1 (no expiry), got %v", userSetTTL)
	}
}

func TestRotateFamilyActiveToken_UserFamiliesSetDoesNotExpire(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-2"
	familyID := "fam-2"
	oldHash := "hash-old"
	newHash := "hash-new"
	activeTTL := 10 * time.Minute
	blacklistTTL := 15 * time.Minute

	initial := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  oldHash,
		JKT:        "thumb-2",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-old",
	}
	if err := store.StoreFamilyActiveToken(ctx, familyID, initial, activeTTL); err != nil {
		t.Fatalf("store initial family token: %v", err)
	}

	rotated := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  newHash,
		JKT:        "thumb-2",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-new",
	}
	outcome, err := store.RotateFamilyActiveToken(
		ctx,
		familyID,
		userID,
		oldHash,
		"thumb-2",
		"kid-1",
		rotated,
		activeTTL,
		blacklistTTL,
	)
	if err != nil {
		t.Fatalf("rotate family active token: %v", err)
	}

	if outcome != RotateSuccess {
		t.Fatalf("expected outcome %s, got %s", RotateSuccess, outcome)
	}

	userSetTTL, err := store.client.TTL(ctx, userFamiliesKey(userID)).Result()
	if err != nil {
		t.Fatalf("ttl user family set: %v", err)
	}
	if userSetTTL != -1 {
		t.Fatalf("expected user family set ttl -1 (no expiry), got %v", userSetTTL)
	}

	blacklisted, err := store.IsTokenHashBlacklisted(ctx, familyID, oldHash)
	if err != nil {
		t.Fatalf("check old token hash blacklist: %v", err)
	}
	if !blacklisted {
		t.Fatalf("expected old token hash to be blacklisted")
	}
}

func TestRotateFamilyActiveToken_SecondReplayHitsGraceWindow(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-replay"
	familyID := "fam-replay"
	oldHash := "hash-old-replay"
	activeTTL := 10 * time.Minute
	blacklistTTL := 15 * time.Minute

	initial := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  oldHash,
		JKT:        "thumb-replay",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-old-replay",
		SigningKID: "kid-old-replay",
	}
	if err := store.StoreFamilyActiveToken(ctx, familyID, initial, activeTTL); err != nil {
		t.Fatalf("store initial family token: %v", err)
	}

	firstRotated := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  "hash-new-replay",
		JKT:        "thumb-replay",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-new-replay",
		SigningKID: "kid-new-replay",
	}

	firstOutcome, err := store.RotateFamilyActiveToken(
		ctx,
		familyID,
		userID,
		oldHash,
		"thumb-replay",
		"kid-old-replay",
		firstRotated,
		activeTTL,
		blacklistTTL,
	)
	if err != nil {
		t.Fatalf("first rotate family active token: %v", err)
	}
	if firstOutcome != RotateSuccess {
		t.Fatalf("expected first outcome %s, got %s", RotateSuccess, firstOutcome)
	}

	secondOutcome, err := store.RotateFamilyActiveToken(
		ctx,
		familyID,
		userID,
		oldHash,
		"thumb-replay",
		"kid-old-replay",
		firstRotated,
		activeTTL,
		blacklistTTL,
	)
	if err != nil {
		t.Fatalf("second rotate family active token: %v", err)
	}
	if secondOutcome != RotateGraceHit {
		t.Fatalf("expected second outcome %s, got %s", RotateGraceHit, secondOutcome)
	}
}

func TestRotateFamilyActiveToken_RejectsShortBlacklistTTL(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-short-ttl"
	familyID := "fam-short-ttl"
	oldHash := "hash-old-short"
	activeTTL := 10 * time.Minute
	blacklistTTL := 5 * time.Minute

	initial := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  oldHash,
		JKT:        "thumb-short",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-old-short",
	}
	if err := store.StoreFamilyActiveToken(ctx, familyID, initial, activeTTL); err != nil {
		t.Fatalf("store initial family token: %v", err)
	}

	rotated := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  "hash-new-short",
		JKT:        "thumb-short",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-new-short",
	}

	_, err := store.RotateFamilyActiveToken(
		ctx,
		familyID,
		userID,
		oldHash,
		"thumb-short",
		"kid-short",
		rotated,
		activeTTL,
		blacklistTTL,
	)
	if err == nil {
		t.Fatalf("expected short blacklist ttl to be rejected")
	}
}

func TestListUserFamiliesPruned_RemovesOnlyStaleFamilies(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-prune"
	activeFamilyID := "fam-active"
	staleFamilyID := "fam-stale"
	activeTTL := 30 * time.Minute

	activeRecord := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  "hash-active",
		JKT:        "thumb-prune",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-active",
	}
	if err := store.StoreFamilyActiveToken(ctx, activeFamilyID, activeRecord, activeTTL); err != nil {
		t.Fatalf("store active family: %v", err)
	}

	if err := store.AddFamilyToUser(ctx, userID, staleFamilyID); err != nil {
		t.Fatalf("add stale family membership: %v", err)
	}

	families, err := store.ListUserFamilies(ctx, userID)
	if err != nil {
		t.Fatalf("list user families: %v", err)
	}
	if len(families) != 1 || families[0] != activeFamilyID {
		t.Fatalf("expected only active family %q after pruning, got %v", activeFamilyID, families)
	}

	activeStillMember, err := store.client.SIsMember(ctx, userFamiliesKey(userID), activeFamilyID).Result()
	if err != nil {
		t.Fatalf("check active family membership: %v", err)
	}
	if !activeStillMember {
		t.Fatalf("expected active family %q to remain in user set", activeFamilyID)
	}

	staleStillMember, err := store.client.SIsMember(ctx, userFamiliesKey(userID), staleFamilyID).Result()
	if err != nil {
		t.Fatalf("check stale family membership: %v", err)
	}
	if staleStillMember {
		t.Fatalf("expected stale family %q to be removed from user set", staleFamilyID)
	}
}

func TestRevokeFamily_BlacklistsActiveTokenAndDeletesFamily(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-revoke"
	familyID := "fam-revoke"
	tokenHash := "hash-revoke"
	activeTTL := 20 * time.Minute
	blacklistTTL := 25 * time.Minute

	rec := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  tokenHash,
		JKT:        "thumb-revoke",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-revoke",
	}
	if err := store.StoreFamilyActiveToken(ctx, familyID, rec, activeTTL); err != nil {
		t.Fatalf("store family active token: %v", err)
	}

	if err := store.RevokeFamily(ctx, userID, familyID, blacklistTTL); err != nil {
		t.Fatalf("revoke family: %v", err)
	}

	blacklisted, err := store.IsTokenHashBlacklisted(ctx, familyID, tokenHash)
	if err != nil {
		t.Fatalf("check blacklist: %v", err)
	}
	if !blacklisted {
		t.Fatalf("expected token hash %q to be blacklisted", tokenHash)
	}

	activeExists, err := store.client.Exists(ctx, activeFamilyKey(familyID)).Result()
	if err != nil {
		t.Fatalf("check active family key: %v", err)
	}
	if activeExists != 0 {
		t.Fatalf("expected active family key to be deleted, got exists=%d", activeExists)
	}

	member, err := store.client.SIsMember(ctx, userFamiliesKey(userID), familyID).Result()
	if err != nil {
		t.Fatalf("check user family membership: %v", err)
	}
	if member {
		t.Fatalf("expected family %q to be removed from the user set", familyID)
	}
}

func TestRevokeFamily_AfterRotateBlacklistsLatestActiveToken(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-revoke-rotate"
	familyID := "fam-revoke-rotate"
	oldHash := "hash-old-revoke-rotate"
	newHash := "hash-new-revoke-rotate"
	activeTTL := 20 * time.Minute
	blacklistTTL := 25 * time.Minute

	initial := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  oldHash,
		JKT:        "thumb-revoke-rotate",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-old-revoke-rotate",
		SigningKID: "kid-old-revoke-rotate",
	}
	if err := store.StoreFamilyActiveToken(ctx, familyID, initial, activeTTL); err != nil {
		t.Fatalf("store initial family token: %v", err)
	}

	rotated := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  newHash,
		JKT:        "thumb-revoke-rotate",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-new-revoke-rotate",
		SigningKID: "kid-new-revoke-rotate",
	}
	outcome, err := store.RotateFamilyActiveToken(
		ctx,
		familyID,
		userID,
		oldHash,
		"thumb-revoke-rotate",
		"kid-old-revoke-rotate",
		rotated,
		activeTTL,
		blacklistTTL,
	)
	if err != nil {
		t.Fatalf("rotate family active token: %v", err)
	}
	if outcome != RotateSuccess {
		t.Fatalf("expected rotate outcome %s, got %s", RotateSuccess, outcome)
	}

	if err := store.RevokeFamily(ctx, userID, familyID, blacklistTTL); err != nil {
		t.Fatalf("revoke family after rotate: %v", err)
	}

	blacklistedNew, err := store.IsTokenHashBlacklisted(ctx, familyID, newHash)
	if err != nil {
		t.Fatalf("check new hash blacklist: %v", err)
	}
	if !blacklistedNew {
		t.Fatalf("expected new active token hash %q to be blacklisted", newHash)
	}

	activeExists, err := store.client.Exists(ctx, activeFamilyKey(familyID)).Result()
	if err != nil {
		t.Fatalf("check active family key: %v", err)
	}
	if activeExists != 0 {
		t.Fatalf("expected active family key to be deleted, got exists=%d", activeExists)
	}
}

func TestLogoutFamily_BlacklistsActiveTokenAndRemovesUserMembership(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-logout"
	familyID := "fam-logout"
	tokenHash := "hash-logout"
	activeTTL := 20 * time.Minute
	blacklistTTL := 25 * time.Minute

	rec := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  tokenHash,
		JKT:        "thumb-logout",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-logout",
	}
	if err := store.StoreFamilyActiveToken(ctx, familyID, rec, activeTTL); err != nil {
		t.Fatalf("store family active token: %v", err)
	}

	if err := store.LogoutFamily(ctx, userID, familyID, tokenHash, blacklistTTL); err != nil {
		t.Fatalf("logout family: %v", err)
	}

	blacklisted, err := store.IsTokenHashBlacklisted(ctx, familyID, tokenHash)
	if err != nil {
		t.Fatalf("check blacklist: %v", err)
	}
	if !blacklisted {
		t.Fatalf("expected token hash %q to be blacklisted", tokenHash)
	}

	activeExists, err := store.client.Exists(ctx, activeFamilyKey(familyID)).Result()
	if err != nil {
		t.Fatalf("check active family key: %v", err)
	}
	if activeExists != 0 {
		t.Fatalf("expected active family key to be deleted, got exists=%d", activeExists)
	}

	member, err := store.client.SIsMember(ctx, userFamiliesKey(userID), familyID).Result()
	if err != nil {
		t.Fatalf("check user family membership: %v", err)
	}
	if member {
		t.Fatalf("expected family %q to be removed from the user set", familyID)
	}
}

func TestRevokeAllUserFamilies_BlacklistsAllActiveTokensAndDeletesUserSet(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-revoke-all"
	activeTTL := 20 * time.Minute
	blacklistTTL := 25 * time.Minute

	families := []struct {
		familyID  string
		tokenHash string
	}{
		{familyID: "fam-revoke-all-1", tokenHash: "hash-revoke-all-1"},
		{familyID: "fam-revoke-all-2", tokenHash: "hash-revoke-all-2"},
	}

	for _, family := range families {
		rec := ActiveRefreshTokenRecord{
			UserID:     userID,
			TokenHash:  family.tokenHash,
			JKT:        "thumb-revoke-all",
			ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
			RefreshJTI: "jti-" + family.familyID,
		}
		if err := store.StoreFamilyActiveToken(ctx, family.familyID, rec, activeTTL); err != nil {
			t.Fatalf("store family active token %s: %v", family.familyID, err)
		}
	}

	if err := store.RevokeAllUserFamilies(ctx, userID, blacklistTTL); err != nil {
		t.Fatalf("revoke all user families: %v", err)
	}

	for _, family := range families {
		blacklisted, err := store.IsTokenHashBlacklisted(ctx, family.familyID, family.tokenHash)
		if err != nil {
			t.Fatalf("check blacklist for %s: %v", family.familyID, err)
		}
		if !blacklisted {
			t.Fatalf("expected token hash %q to be blacklisted", family.tokenHash)
		}

		activeExists, err := store.client.Exists(ctx, activeFamilyKey(family.familyID)).Result()
		if err != nil {
			t.Fatalf("check active family key %s: %v", family.familyID, err)
		}
		if activeExists != 0 {
			t.Fatalf("expected active family key %s to be deleted, got exists=%d", family.familyID, activeExists)
		}
	}

	for _, family := range families {
		member, err := store.client.SIsMember(ctx, userFamiliesKey(userID), family.familyID).Result()
		if err != nil {
			t.Fatalf("check user family membership %s: %v", family.familyID, err)
		}
		if member {
			t.Fatalf("expected family %q to be removed from the user set", family.familyID)
		}
	}
}

func TestRotateFamilyActiveToken_SetsRotatedGraceMarker(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	userID := "user-3"
	familyID := "fam-3"
	oldHash := "hash-old-3"
	newHash := "hash-new-3"
	activeTTL := 10 * time.Minute
	blacklistTTL := 15 * time.Minute

	initial := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  oldHash,
		JKT:        "thumb-3",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-old-3",
	}
	if err := store.StoreFamilyActiveToken(ctx, familyID, initial, activeTTL); err != nil {
		t.Fatalf("store initial family token: %v", err)
	}

	rotated := ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  newHash,
		JKT:        "thumb-3",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-new-3",
	}

	outcome, err := store.RotateFamilyActiveToken(
		ctx,
		familyID,
		userID,
		oldHash,
		"thumb-3",
		"kid-1",
		rotated,
		activeTTL,
		blacklistTTL,
	)
	if err != nil {
		t.Fatalf("rotate family active token: %v", err)
	}
	if outcome != RotateSuccess {
		t.Fatalf("expected outcome %s, got %s", RotateSuccess, outcome)
	}

	graceFamilyID, err := store.GetRotatedTokenGraceFamilyID(ctx, familyID, oldHash)
	if err != nil {
		t.Fatalf("get rotated grace family id: %v", err)
	}
	if graceFamilyID != familyID {
		t.Fatalf("expected grace family id %q, got %q", familyID, graceFamilyID)
	}

	graceTTL, err := store.client.TTL(ctx, rotatedGraceKey(familyID, oldHash)).Result()
	if err != nil {
		t.Fatalf("ttl rotated grace key: %v", err)
	}
	if graceTTL <= 0 {
		t.Fatalf("expected rotated grace ttl > 0, got %v", graceTTL)
	}
}

func TestGetRotatedTokenGraceFamilyID_NotFound(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	_, err := store.GetRotatedTokenGraceFamilyID(context.Background(), "fam-missing", "missing-hash")
	if !errors.Is(err, ErrGraceNotFound) {
		t.Fatalf("expected ErrGraceNotFound, got %v", err)
	}
}

func TestLoadRefreshTokenState_ReturnsBlacklistedGraceAndActiveRecord(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	familyID := "fam-state"
	tokenHash := "token-hash-state"
	activeTTL := 20 * time.Minute

	rec := ActiveRefreshTokenRecord{
		UserID:     "user-state",
		TokenHash:  "current-hash",
		JKT:        "thumb-state",
		ExpiresAt:  time.Now().Add(activeTTL).UTC().Format(time.RFC3339),
		RefreshJTI: "jti-state",
		SigningKID: "kid-state",
	}
	if err := store.StoreFamilyActiveToken(ctx, familyID, rec, activeTTL); err != nil {
		t.Fatalf("store family active token: %v", err)
	}
	if err := store.BlacklistTokenHash(ctx, familyID, tokenHash, activeTTL); err != nil {
		t.Fatalf("blacklist token hash: %v", err)
	}
	if err := store.client.Set(ctx, rotatedGraceKey(familyID, tokenHash), familyID, rotatedGraceTTL).Err(); err != nil {
		t.Fatalf("set rotated grace key: %v", err)
	}

	state, err := store.LoadRefreshTokenState(ctx, familyID, tokenHash)
	if err != nil {
		t.Fatalf("load refresh token state: %v", err)
	}
	if !state.Blacklisted {
		t.Fatalf("expected token hash to be blacklisted")
	}
	if state.GraceFamilyID != familyID {
		t.Fatalf("expected grace family id %q, got %q", familyID, state.GraceFamilyID)
	}
	if state.FamilyKID != rec.SigningKID {
		t.Fatalf("expected family kid %q, got %q", rec.SigningKID, state.FamilyKID)
	}
	if state.ActiveRecord == nil || state.ActiveRecord.TokenHash != rec.TokenHash {
		t.Fatalf("expected active record token hash %q, got %#v", rec.TokenHash, state.ActiveRecord)
	}
}
