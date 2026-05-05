package token

import (
	"errors"
	"time"
)

// Sentinel errors for token validation
var (
	ErrExpiredToken = errors.New("token has expired")
	ErrInvalidToken = errors.New("token is invalid")
)

// TokenMaker is the interface for creating and verifying JWT tokens.
type TokenMaker interface {
	// CreateRefreshToken mints a refresh token. The returned RefreshPayload
	// contains the JTI that must be stored in Redis by the caller.
	CreateRefreshToken(userID, email, name, familyID string, duration time.Duration) (string, *RefreshPayload, error)

	// CreateAccessToken mints an access token paired with a refresh token.
	// refreshJTI must be the JTI of the already-created refresh token.
	CreateAccessToken(userID, email, name, familyID, refreshJTI string, duration time.Duration) (string, *AccessPayload, error)

	// VerifyAccessToken parses and validates an access token string.
	VerifyAccessToken(token string) (*AccessPayload, error)

	// VerifyRefreshToken parses and validates a refresh token string.
	VerifyRefreshToken(token string) (*RefreshPayload, error)

	// GetCurrentKeyID returns the current active signing key ID.
	GetCurrentKeyID() string
}
