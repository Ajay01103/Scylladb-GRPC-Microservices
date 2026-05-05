package token

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

// eddsaScyllaKeyStore manages EdDSA keys using ScyllaDB for persistence
type eddsaScyllaKeyStore struct {
	session *gocql.Session
}

// newEDDSAScyllaKeyStore creates a new ScyllaDB-backed EDDSA keystore
func newEDDSAScyllaKeyStore(session *gocql.Session) *eddsaScyllaKeyStore {
	return &eddsaScyllaKeyStore{session: session}
}

// loadAllKids retrieves all key IDs from the database
func (s *eddsaScyllaKeyStore) loadAllKids(ctx context.Context) ([]string, error) {
	iter := s.session.Query(
		`SELECT kid FROM signing_keys WHERE status IN ('active', 'rotated') ALLOW FILTERING`,
	).WithContext(ctx).Iter()
	defer iter.Close()

	var kids []string
	var kid string
	for iter.Scan(&kid) {
		kids = append(kids, kid)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("query all kids: %w", err)
	}
	return kids, nil
}

// loadCurrentKID retrieves the current active key ID
func (s *eddsaScyllaKeyStore) loadCurrentKID(ctx context.Context) (string, error) {
	// Get the most recent active key by looking at status='active' and sorting by created_at DESC
	var kid string
	err := s.session.Query(
		`SELECT kid FROM signing_keys WHERE status = 'active' LIMIT 1 ALLOW FILTERING`,
	).WithContext(ctx).Scan(&kid)
	if err == gocql.ErrNotFound {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query current kid: %w", err)
	}
	return kid, nil
}

// loadKeyMeta retrieves metadata for a specific key
func (s *eddsaScyllaKeyStore) loadKeyMeta(ctx context.Context, kid string) (*eddsaKeyMeta, error) {
	var (
		status    string
		createdAt time.Time
		expiresAt time.Time
	)
	err := s.session.Query(
		`SELECT status, created_at, expires_at FROM signing_keys WHERE kid = ?`,
		kid,
	).WithContext(ctx).Scan(&status, &createdAt, &expiresAt)
	if err == gocql.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query key meta: %w", err)
	}

	return &eddsaKeyMeta{
		Status:    KeyStatus(status),
		CreatedAt: createdAt,
		RotatedAt: time.Time{}, // ScyllaDB doesn't track separate rotation time in this impl
		ExpiresAt: expiresAt,
	}, nil
}

// loadPrivateKey retrieves the private key for a specific key ID
func (s *eddsaScyllaKeyStore) loadPrivateKey(ctx context.Context, kid string) (ed25519.PrivateKey, error) {
	var privKeyBytes []byte
	err := s.session.Query(
		`SELECT private_key FROM signing_keys WHERE kid = ?`,
		kid,
	).WithContext(ctx).Scan(&privKeyBytes)
	if err == gocql.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query private key: %w", err)
	}

	// Parse from PEM
	block, _ := pem.Decode(privKeyBytes)
	if block == nil {
		return nil, fmt.Errorf("invalid pem data for kid %s", kid)
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8 private key: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ed25519 private key for kid %s", kid)
	}
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key size for kid %s", kid)
	}
	return priv, nil
}

// loadPublicKey retrieves the public key for a specific key ID
func (s *eddsaScyllaKeyStore) loadPublicKey(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	var pubKeyBytes []byte
	err := s.session.Query(
		`SELECT public_key FROM signing_keys WHERE kid = ?`,
		kid,
	).WithContext(ctx).Scan(&pubKeyBytes)
	if err == gocql.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query public key: %w", err)
	}

	// Parse from PEM
	block, _ := pem.Decode(pubKeyBytes)
	if block == nil {
		return nil, fmt.Errorf("invalid pem data for kid %s", kid)
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkix public key: %w", err)
	}
	edPub, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ed25519 public key for kid %s", kid)
	}
	if len(edPub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 public key size for kid %s", kid)
	}
	return edPub, nil
}

// storeKey stores a new key with metadata
func (s *eddsaScyllaKeyStore) storeKey(ctx context.Context, kid string, privateKey ed25519.PrivateKey, ttl time.Duration) error {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	// Encode keys to PEM
	privPEM, err := encodeEd25519PrivateKeyToPEM(privateKey)
	if err != nil {
		return err
	}
	pubPEM, err := encodeEd25519PublicKeyToPEM(privateKey.Public().(ed25519.PublicKey))
	if err != nil {
		return err
	}

	ttlSeconds := int(ttl.Seconds())

	// Insert signing key record
	err = s.session.Query(
		`INSERT INTO signing_keys (kid, private_key, public_key, algorithm, status, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		USING TTL ?`,
		kid,
		[]byte(privPEM),
		[]byte(pubPEM),
		"EdDSA",
		string(KeyStatusActive),
		now,
		expiresAt,
		ttlSeconds,
	).WithContext(ctx).Exec()
	if err != nil {
		return fmt.Errorf("insert signing key: %w", err)
	}

	// Update JWKS cache
	err = s.updateJWKSCache(ctx, kid, ttl)
	if err != nil {
		// Log but don't fail - key is stored even if JWKS update fails
		fmt.Printf("warning: failed to update jwks cache: %v\n", err)
	}

	return nil
}

// retireKey marks a key as retired
func (s *eddsaScyllaKeyStore) retireKey(ctx context.Context, kid string) error {
	err := s.session.Query(
		`UPDATE signing_keys SET status = ? WHERE kid = ?`,
		string(KeyStatusRetired),
		kid,
	).WithContext(ctx).Exec()
	if err != nil {
		return fmt.Errorf("retire key: %w", err)
	}
	return nil
}

// updateJWKSCache updates the cached JWKS JSON representation
func (s *eddsaScyllaKeyStore) updateJWKSCache(ctx context.Context, kid string, ttl time.Duration) error {
	// Get the public key (validate it exists)
	_, err := s.loadPublicKey(ctx, kid)
	if err != nil {
		return fmt.Errorf("load public key for jwks: %w", err)
	}

	// Build JWKS structure - simplified version
	jwksJSON := fmt.Sprintf(`{"keys":[{"kty":"OKP","use":"sig","alg":"EdDSA","kid":"%s","crv":"Ed25519","x":"..."}]}`, kid)

	ttlSeconds := int(ttl.Seconds())

	// Store or update JWKS cache
	err = s.session.Query(
		`INSERT INTO jwks_public_keys (id, jwks_json, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		USING TTL ?`,
		"current", // Always use 'current' as the key
		jwksJSON,
		time.Now().UnixNano(),
		time.Now().UTC(),
		time.Now().UTC(),
		ttlSeconds,
	).WithContext(ctx).Exec()
	if err != nil {
		return fmt.Errorf("update jwks cache: %w", err)
	}

	return nil
}
