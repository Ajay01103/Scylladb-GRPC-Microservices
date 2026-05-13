package service

import (
	"context"
	"testing"

	"github.com/dgraph-io/ristretto"
	"go.uber.org/zap"

	"github.com/Ajay01103/go-notion/auth/internal/tokencache"
	"github.com/google/uuid"
)

func TestGetCurrentUserProfile_UsesCacheHit(t *testing.T) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,
		MaxCost:     1 << 20,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		t.Fatalf("create cache: %v", err)
	}

	userID := uuid.New()
	entry := tokencache.CurrentUserEntry{
		UserID: userID.String(),
		Email:  "cached@example.com",
		Name:   "Cached User",
	}
	cache.SetWithTTL(tokencache.CurrentUserKey(userID.String()), entry, tokencache.CurrentUserCost, tokencache.CurrentUserTTL)
	cache.Wait()

	svc := &AuthService{
		cache:  cache,
		logger: zap.NewNop(),
	}

	got, err := svc.GetCurrentUserProfile(context.Background(), userID)
	if err != nil {
		t.Fatalf("get current user profile: %v", err)
	}

	if got == nil {
		t.Fatal("expected cached current user profile")
	}
	if got.UserID != entry.UserID || got.Email != entry.Email || got.Name != entry.Name {
		t.Fatalf("unexpected cached profile: %#v", got)
	}
}
