package tokencache

import (
	"time"

	"github.com/dgraph-io/ristretto"
)

const (
	SessionStateCost = 1
	CurrentUserCost  = 1
	GlobalVerCost    = 1

	SessionStateTTL = 14 * time.Minute // Must match session rotation window; LWT validates gen
	CurrentUserTTL  = 15 * time.Minute
	GlobalVerTTL    = 5 * time.Minute // Separate from session; invalidation independent

	prefixSess = "sess:"
	prefixCur  = "cur:"
	prefixGver = "gver:"
)

type CurrentUserEntry struct {
	UserID string
	Email  string
	Name   string
}

func NewRistrettoCache() (*ristretto.Cache, error) {
	return ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6,
		MaxCost:     50 << 20,
		BufferItems: 64,
		Metrics:     true,
	})
}

func SessionKey(sessionID string) string {
	return prefixSess + sessionID
}

func CurrentUserKey(userID string) string {
	return prefixCur + userID
}

func GlobalVerKey(userID string) string {
	return prefixGver + userID
}
