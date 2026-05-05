package redisstore

import "errors"

var (
	ErrTokenRevoked       = errors.New("refresh token has been revoked")
	ErrFamilyNotFound     = errors.New("refresh token family not found")
	ErrTokenReuseDetected = errors.New("refresh token reuse detected")
	ErrGraceNotFound      = errors.New("refresh token grace marker not found")
	ErrKeyBindingMismatch = errors.New("refresh token key binding mismatch")
)
