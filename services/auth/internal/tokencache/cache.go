package tokencache

import (
	"time"

	"github.com/dgraph-io/ristretto"
)

type FamilyState struct {
	FamilyID         string
	UserID           string
	CurrentToken     string
	Generation       int
	Revoked          bool
	ExpiresAt        time.Time
	AbsoluteExpiresAt time.Time
	RefreshJTI       string
	SigningKID       string
	IssuedAt         int64
	LastUsedAt       time.Time
	IPAddress        string
	UserAgent        string
	DeviceLabel      string
	Compromised      bool
	CompromisedAt    time.Time
}

type TokenCacheEntry struct {
	State    *FamilyState
	CachedAt time.Time
}

type RevokedEntry struct {
	RevokedAt time.Time
}

const (
	FamilyStateCost  = 1
	RevokedEntryCost = 1
	SessionStateCost = 1
	GlobalVerCost    = 1

	FamilyStateTTL = 5 * time.Minute
	RevokedTTL     = 10 * time.Minute
	NegativeTTL    = 30 * time.Second
	SessionStateTTL = 14 * time.Minute  // Must match session rotation window; LWT validates gen
	GlobalVerTTL    = 5 * time.Minute   // Separate from session; invalidation independent

	prefixFamily  = "fam:"
	prefixRevoked = "rev:"
	prefixMiss    = "miss:"
	prefixSess    = "sess:"
	prefixGver    = "gver:"
)

func NewRistrettoCache() (*ristretto.Cache, error) {
	return ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6,
		MaxCost:     50 << 20,
		BufferItems: 64,
		Metrics:     true,
	})
}

func FamilyKey(familyID string) string {
	return prefixFamily + familyID
}

func RevokedKey(familyID string) string {
	return prefixRevoked + familyID
}

func MissKey(familyID string) string {
	return prefixMiss + familyID
}

func SessionKey(sessionID string) string {
	return prefixSess + sessionID
}

func GlobalVerKey(userID string) string {
	return prefixGver + userID
}