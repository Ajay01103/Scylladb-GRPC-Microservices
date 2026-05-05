package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
)

// ActiveRefreshTokenRecord describes a stored refresh token family
type ActiveRefreshTokenRecord struct {
	UserID     string `json:"user_id"`
	TokenHash  string `json:"token_hash"`
	JKT        string `json:"jkt,omitempty"`
	ExpiresAt  string `json:"expires_at"`
	RefreshJTI string `json:"refresh_jti"`
	SigningKID string `json:"signing_kid"`
	IssuedAt   int64  `json:"issued_at"`
}

// RotateOutcome describes the result of a token rotation attempt
type RotateOutcome string

const (
	RotateSuccess        RotateOutcome = "OK"
	RotateFamilyNotFound RotateOutcome = "FAMILY_NOT_FOUND"
	RotateUserMismatch   RotateOutcome = "USER_MISMATCH"
	RotateHashMismatch   RotateOutcome = "HASH_MISMATCH"
	RotateJKTMismatch    RotateOutcome = "JKT_MISMATCH"
	RotateKIDMismatch    RotateOutcome = "KID_MISMATCH"
	RotateBlacklisted    RotateOutcome = "BLACKLISTED"
	RotateGraceHit       RotateOutcome = "GRACE_HIT"
	RotateBadArgs        RotateOutcome = "BAD_ARGS"
)

// HashTokenSHA256 returns the SHA256 hash of a token as a hex string
func HashTokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:])
}

// TokenStore defines the interface for token persistence and rotation.
type TokenStore interface {
	StoreFamilyActiveToken(ctx context.Context, familyID string, rec ActiveRefreshTokenRecord, ttl time.Duration) error
	RotateFamilyActiveToken(
		ctx context.Context,
		familyID, userID, oldTokenHash, oldJKT, signingKID string,
		newRecord ActiveRefreshTokenRecord,
		activeTTL, blacklistTTL time.Duration,
	) (RotateOutcome, error)
	RevokeFamily(ctx context.Context, userID, familyID string, blacklistTTL time.Duration) error
	LogoutFamily(ctx context.Context, userID, familyID, tokenHash string, blacklistTTL time.Duration) error
	RevokeAllUserFamilies(ctx context.Context, userID string, blacklistTTL time.Duration) error
	ListUserFamilies(ctx context.Context, userID string) ([]string, error)
}

