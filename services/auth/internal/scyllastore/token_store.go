package scyllastore

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"

	"github.com/Ajay01103/go-notion/auth/internal/service"
)

const (
	refreshPrefix        = "rt:"
	refreshUserSetPrefix = "rt:user:"
)

// rotatedGraceTTL covers concurrent refresh races long enough for the
// successor token to be minted and persisted, but stays short to limit replay
// exposure.
const rotatedGraceTTL = 15 * time.Second

const maxRevokeTries = 5

// TokenStore manages refresh token lifecycle in ScyllaDB using:
// - allowlist: refresh_token_families table
// - blacklist: refresh_token_blacklist table
// - user families: user_families table
// - grace window: token_rotation_grace table
type TokenStore struct {
	session *gocql.Session
}

// Errors
var (
	ErrFamilyNotFound = errors.New("token family not found")
	ErrGraceNotFound  = errors.New("grace window not found")
	ErrBlacklisted    = errors.New("token hash is blacklisted")
)

// New creates a TokenStore from a connected ScyllaDB session
func New(session *gocql.Session) *TokenStore {
	return &TokenStore{session: session}
}

// RefreshTokenState captures the refresh-token state needed by the auth service
// in a single ScyllaDB round-trip.
type RefreshTokenState struct {
	Blacklisted   bool
	GraceFamilyID string
	FamilyKID     string
	ActiveRecord  *service.ActiveRefreshTokenRecord
}

// LoadRefreshTokenState fetches blacklist, rotated-grace, and active family data
func (s *TokenStore) LoadRefreshTokenState(ctx context.Context, familyID, tokenHash string) (*RefreshTokenState, error) {
	state := &RefreshTokenState{}

	// Check blacklist
	var blacklistVal string
	err := s.session.Query(
		"SELECT revoked_at FROM refresh_token_blacklist WHERE family_id = ? AND token_hash = ? LIMIT 1",
		familyID, tokenHash,
	).WithContext(ctx).Scan(&blacklistVal)
	if err == nil {
		state.Blacklisted = true
	} else if err != gocql.ErrNotFound {
		return nil, fmt.Errorf("check blacklist: %w", err)
	}

	// Check grace window
	var graceFamilyID string
	err = s.session.Query(
		"SELECT new_family_id FROM token_rotation_grace WHERE family_id = ? AND old_token_hash = ? LIMIT 1",
		familyID, tokenHash,
	).WithContext(ctx).Scan(&graceFamilyID)
	if err == nil {
		state.GraceFamilyID = graceFamilyID
	} else if err != gocql.ErrNotFound {
		return nil, fmt.Errorf("check grace window: %w", err)
	}

	// Get active family record
	var (
		userID     string
		tokenHashVal string
		jkt        string
		expiresAt  string
		refreshJTI string
		signingKID string
		issuedAt   int64
	)
	err = s.session.Query(
		`SELECT user_id, token_hash, jkt, expires_at, refresh_jti, signing_kid, issued_at
		FROM refresh_token_families WHERE family_id = ? LIMIT 1`,
		familyID,
	).WithContext(ctx).Scan(&userID, &tokenHashVal, &jkt, &expiresAt, &refreshJTI, &signingKID, &issuedAt)
	if err == gocql.ErrNotFound {
		return nil, ErrFamilyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get active family: %w", err)
	}

	state.ActiveRecord = &service.ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  tokenHashVal,
		JKT:        jkt,
		ExpiresAt:  expiresAt,
		RefreshJTI: refreshJTI,
		SigningKID: signingKID,
		IssuedAt:   issuedAt,
	}
	state.FamilyKID = signingKID

	return state, nil
}

// StoreFamilyActiveToken stores a new active refresh token family
func (s *TokenStore) StoreFamilyActiveToken(ctx context.Context, familyID string, rec service.ActiveRefreshTokenRecord, ttl time.Duration) error {
	now := time.Now()
	ttlSeconds := int(ttl.Seconds())

	// Insert into refresh_token_families
	if err := s.session.Query(
		`INSERT INTO refresh_token_families (family_id, user_id, token_hash, jkt, expires_at, refresh_jti, signing_kid, issued_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		USING TTL ?`,
		familyID, rec.UserID, rec.TokenHash, rec.JKT, rec.ExpiresAt, rec.RefreshJTI,
		rec.SigningKID, rec.IssuedAt, now, now, ttlSeconds,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("insert active family: %w", err)
	}

	// Add family to user_families
	if err := s.session.Query(
		`INSERT INTO user_families (user_id, family_id, added_at) VALUES (?, ?, ?)`,
		rec.UserID, familyID, now,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("add user family: %w", err)
	}

	return nil
}

// GetFamilyActiveToken retrieves an active refresh token family record
func (s *TokenStore) GetFamilyActiveToken(ctx context.Context, familyID string) (*service.ActiveRefreshTokenRecord, error) {
	var (
		userID     string
		tokenHash  string
		jkt        string
		expiresAt  string
		refreshJTI string
		signingKID string
		issuedAt   int64
	)
	err := s.session.Query(
		`SELECT user_id, token_hash, jkt, expires_at, refresh_jti, signing_kid, issued_at
		FROM refresh_token_families WHERE family_id = ? LIMIT 1`,
		familyID,
	).WithContext(ctx).Scan(&userID, &tokenHash, &jkt, &expiresAt, &refreshJTI, &signingKID, &issuedAt)
	if err == gocql.ErrNotFound {
		return nil, ErrFamilyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get active family: %w", err)
	}

	return &service.ActiveRefreshTokenRecord{
		UserID:     userID,
		TokenHash:  tokenHash,
		JKT:        jkt,
		ExpiresAt:  expiresAt,
		RefreshJTI: refreshJTI,
		SigningKID: signingKID,
		IssuedAt:   issuedAt,
	}, nil
}

// IsTokenHashBlacklisted checks if a token hash is blacklisted
func (s *TokenStore) IsTokenHashBlacklisted(ctx context.Context, familyID, tokenHash string) (bool, error) {
	var val string
	err := s.session.Query(
		"SELECT revoked_at FROM refresh_token_blacklist WHERE family_id = ? AND token_hash = ? LIMIT 1",
		familyID, tokenHash,
	).WithContext(ctx).Scan(&val)
	if err == gocql.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check blacklist: %w", err)
	}
	return true, nil
}

// BlacklistTokenHash adds a token hash to the blacklist
func (s *TokenStore) BlacklistTokenHash(ctx context.Context, familyID, tokenHash string, ttl time.Duration) error {
	ttlSeconds := int(ttl.Seconds())
	if err := s.session.Query(
		`INSERT INTO refresh_token_blacklist (family_id, token_hash, revoked_at) VALUES (?, ?, ?)
		USING TTL ?`,
		familyID, tokenHash, time.Now(), ttlSeconds,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("blacklist token hash: %w", err)
	}
	return nil
}

// RotateFamilyActiveToken atomically rotates the active token using Lightweight Transactions
func (s *TokenStore) RotateFamilyActiveToken(
	ctx context.Context,
	familyID, userID, oldTokenHash, oldJKT, signingKID string,
	newRecord service.ActiveRefreshTokenRecord,
	activeTTL, blacklistTTL time.Duration,
) (service.RotateOutcome, error) {
	if blacklistTTL < activeTTL {
		return "", fmt.Errorf("blacklist ttl must be >= active ttl")
	}

	now := time.Now()
	activeTTLSeconds := int(activeTTL.Seconds())
	blacklistTTLSeconds := int(blacklistTTL.Seconds())
	graceTTLSeconds := int(rotatedGraceTTL.Seconds())

	// Step 1: Check blacklist first (without IF condition)
	isBlacklisted, err := s.IsTokenHashBlacklisted(ctx, familyID, oldTokenHash)
	if err != nil {
		return "", fmt.Errorf("check blacklist: %w", err)
	}

	if isBlacklisted {
		// Check if we're in grace window for this family
		var newFamilyID string
		err := s.session.Query(
			"SELECT new_family_id FROM token_rotation_grace WHERE family_id = ? AND old_token_hash = ? LIMIT 1",
			familyID, oldTokenHash,
		).WithContext(ctx).Scan(&newFamilyID)
		if err == nil {
			return service.RotateGraceHit, nil
		}
		if err != gocql.ErrNotFound {
			return "", fmt.Errorf("check grace: %w", err)
		}
		return service.RotateBlacklisted, nil
	}

	// Step 2: Try to update active family with IF condition (CAS - Compare And Set)
	// First get current state to validate
	var (
		storedUserID   string
		storedHash     string
		storedJKT      string
		storedSignKID  string
	)
	err = s.session.Query(
		`SELECT user_id, token_hash, jkt, signing_kid FROM refresh_token_families WHERE family_id = ? LIMIT 1`,
		familyID,
	).WithContext(ctx).Scan(&storedUserID, &storedHash, &storedJKT, &storedSignKID)
	if err == gocql.ErrNotFound {
		return service.RotateFamilyNotFound, nil
	}
	if err != nil {
		return "", fmt.Errorf("get current family state: %w", err)
	}

	// Validate conditions
	if storedUserID != userID {
		return service.RotateUserMismatch, nil
	}
	if storedHash != oldTokenHash {
		return service.RotateHashMismatch, nil
	}
	if oldJKT != "" && storedJKT != oldJKT && storedJKT != "" {
		return service.RotateJKTMismatch, nil
	}
	if signingKID != "" && storedSignKID != signingKID && storedSignKID != "" {
		return service.RotateKIDMismatch, nil
	}

	// Step 3: Perform atomic rotation using Lightweight Transaction
	// Update active family
	applied, err := s.session.Query(
		`UPDATE refresh_token_families
		SET user_id = ?, token_hash = ?, jkt = ?, expires_at = ?, refresh_jti = ?, signing_kid = ?, issued_at = ?, updated_at = ?
		WHERE family_id = ?
		IF user_id = ? AND token_hash = ?
		USING TTL ?`,
		newRecord.UserID, newRecord.TokenHash, newRecord.JKT, newRecord.ExpiresAt,
		newRecord.RefreshJTI, newRecord.SigningKID, newRecord.IssuedAt, now,
		familyID,
		storedUserID, storedHash,
		activeTTLSeconds,
	).WithContext(ctx).ScanCAS(&storedUserID, &storedHash)
	if err != nil {
		return "", fmt.Errorf("rotate family: %w", err)
	}

	if !applied {
		// Condition failed - family state changed concurrently
		return service.RotateHashMismatch, nil
	}

	// Step 4: Add old token hash to blacklist
	if err := s.session.Query(
		`INSERT INTO refresh_token_blacklist (family_id, token_hash, revoked_at) VALUES (?, ?, ?)
		USING TTL ?`,
		familyID, oldTokenHash, now, blacklistTTLSeconds,
	).WithContext(ctx).Exec(); err != nil {
		return "", fmt.Errorf("blacklist old hash: %w", err)
	}

	// Step 5: Add grace window entry
	if err := s.session.Query(
		`INSERT INTO token_rotation_grace (family_id, old_token_hash, new_family_id, created_at) VALUES (?, ?, ?, ?)
		USING TTL ?`,
		familyID, oldTokenHash, familyID, now, graceTTLSeconds,
	).WithContext(ctx).Exec(); err != nil {
		return "", fmt.Errorf("add grace window: %w", err)
	}

	// Step 6: Update user families
	if err := s.session.Query(
		`INSERT INTO user_families (user_id, family_id, added_at) VALUES (?, ?, ?)`,
		newRecord.UserID, familyID, now,
	).WithContext(ctx).Exec(); err != nil {
		// Log but don't fail - family is already updated
		return service.RotateSuccess, nil
	}

	return service.RotateSuccess, nil
}

// RevokeFamily revokes a specific family
func (s *TokenStore) RevokeFamily(ctx context.Context, userID, familyID string, blacklistTTL time.Duration) error {
	ttlSeconds := int(blacklistTTL.Seconds())

	// Get current active record
	var tokenHash string
	err := s.session.Query(
		"SELECT token_hash FROM refresh_token_families WHERE family_id = ? LIMIT 1",
		familyID,
	).WithContext(ctx).Scan(&tokenHash)
	if err == gocql.ErrNotFound {
		// Already revoked
		return nil
	}
	if err != nil {
		return fmt.Errorf("get family: %w", err)
	}

	// Delete family
	if err := s.session.Query(
		"DELETE FROM refresh_token_families WHERE family_id = ?",
		familyID,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete family: %w", err)
	}

	// Blacklist the token hash
	if tokenHash != "" {
		if err := s.session.Query(
			`INSERT INTO refresh_token_blacklist (family_id, token_hash, revoked_at) VALUES (?, ?, ?)
			USING TTL ?`,
			familyID, tokenHash, time.Now(), ttlSeconds,
		).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("blacklist token: %w", err)
		}
	}

	// Remove from user families
	if err := s.session.Query(
		"DELETE FROM user_families WHERE user_id = ? AND family_id = ?",
		userID, familyID,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("remove user family: %w", err)
	}

	return nil
}

// LogoutFamily logs out a specific family
func (s *TokenStore) LogoutFamily(ctx context.Context, userID, familyID, tokenHash string, blacklistTTL time.Duration) error {
	ttlSeconds := int(blacklistTTL.Seconds())

	// Delete active family
	if err := s.session.Query(
		"DELETE FROM refresh_token_families WHERE family_id = ?",
		familyID,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete family: %w", err)
	}

	// Blacklist token hash if provided
	if tokenHash != "" {
		if err := s.session.Query(
			`INSERT INTO refresh_token_blacklist (family_id, token_hash, revoked_at) VALUES (?, ?, ?)
			USING TTL ?`,
			familyID, tokenHash, time.Now(), ttlSeconds,
		).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("blacklist token: %w", err)
		}
	}

	// Remove from user families
	_ = s.session.Query(
		"DELETE FROM user_families WHERE user_id = ? AND family_id = ?",
		userID, familyID,
	).WithContext(ctx).Exec()

	return nil
}

// RevokeAllUserFamilies revokes all families for a user
func (s *TokenStore) RevokeAllUserFamilies(ctx context.Context, userID string, blacklistTTL time.Duration) error {
	ttlSeconds := int(blacklistTTL.Seconds())

	// Get all families for user
	families, err := s.ListUserFamilies(ctx, userID)
	if err != nil {
		return fmt.Errorf("list families: %w", err)
	}

	if len(families) == 0 {
		return nil
	}

	// For each family, get its active record and blacklist the token hash
	for _, familyID := range families {
		var tokenHash string
		err := s.session.Query(
			"SELECT token_hash FROM refresh_token_families WHERE family_id = ? LIMIT 1",
			familyID,
		).WithContext(ctx).Scan(&tokenHash)
		if err == gocql.ErrNotFound {
			continue
		}
		if err != nil {
			return fmt.Errorf("get family token hash: %w", err)
		}

		// Delete family
		if err := s.session.Query(
			"DELETE FROM refresh_token_families WHERE family_id = ?",
			familyID,
		).WithContext(ctx).Exec(); err != nil {
			return fmt.Errorf("delete family: %w", err)
		}

		// Blacklist token hash
		if tokenHash != "" {
			if err := s.session.Query(
				`INSERT INTO refresh_token_blacklist (family_id, token_hash, revoked_at) VALUES (?, ?, ?)
				USING TTL ?`,
				familyID, tokenHash, time.Now(), ttlSeconds,
			).WithContext(ctx).Exec(); err != nil {
				return fmt.Errorf("blacklist token: %w", err)
			}
		}
	}

	// Delete all user family mappings
	if err := s.session.Query(
		"DELETE FROM user_families WHERE user_id = ?",
		userID,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("delete user families: %w", err)
	}

	return nil
}

// GetFamilyKID returns the signing_kid stored in the active family record
func (s *TokenStore) GetFamilyKID(ctx context.Context, familyID string) (string, error) {
	var signingKID string
	err := s.session.Query(
		"SELECT signing_kid FROM refresh_token_families WHERE family_id = ? LIMIT 1",
		familyID,
	).WithContext(ctx).Scan(&signingKID)
	if err == gocql.ErrNotFound {
		return "", ErrFamilyNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get family kid: %w", err)
	}
	return signingKID, nil
}

// GetRotatedTokenGraceFamilyID returns the grace window family ID
func (s *TokenStore) GetRotatedTokenGraceFamilyID(ctx context.Context, familyID, oldTokenHash string) (string, error) {
	var newFamilyID string
	err := s.session.Query(
		"SELECT new_family_id FROM token_rotation_grace WHERE family_id = ? AND old_token_hash = ? LIMIT 1",
		familyID, oldTokenHash,
	).WithContext(ctx).Scan(&newFamilyID)
	if err == gocql.ErrNotFound {
		return "", ErrGraceNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get grace family: %w", err)
	}
	return newFamilyID, nil
}

// AddFamilyToUser adds a family to a user's family list
func (s *TokenStore) AddFamilyToUser(ctx context.Context, userID, familyID string) error {
	if err := s.session.Query(
		`INSERT INTO user_families (user_id, family_id, added_at) VALUES (?, ?, ?)`,
		userID, familyID, time.Now(),
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("add family to user: %w", err)
	}
	return nil
}

// RemoveFamilyFromUser removes a family from a user's family list
func (s *TokenStore) RemoveFamilyFromUser(ctx context.Context, userID, familyID string) error {
	if err := s.session.Query(
		"DELETE FROM user_families WHERE user_id = ? AND family_id = ?",
		userID, familyID,
	).WithContext(ctx).Exec(); err != nil {
		return fmt.Errorf("remove family from user: %w", err)
	}
	return nil
}

// ListUserFamilies returns all active families for a user
func (s *TokenStore) ListUserFamilies(ctx context.Context, userID string) ([]string, error) {
	// Get all families from user_families mapping
	iter := s.session.Query(
		"SELECT family_id FROM user_families WHERE user_id = ?",
		userID,
	).WithContext(ctx).Iter()
	defer iter.Close()

	var families []string
	var familyID string
	for iter.Scan(&familyID) {
		families = append(families, familyID)
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("list user families: %w", err)
	}

	if len(families) == 0 {
		return nil, nil
	}

	// Prune stale families (where active record doesn't exist)
	var staleFamilies []string
	for _, fid := range families {
		var dummy string
		err := s.session.Query(
			"SELECT family_id FROM refresh_token_families WHERE family_id = ? LIMIT 1",
			fid,
		).WithContext(ctx).Scan(&dummy)
		if err == gocql.ErrNotFound {
			staleFamilies = append(staleFamilies, fid)
		} else if err != nil {
			return nil, fmt.Errorf("check family existence: %w", err)
		}
	}

	// Remove stale entries
	for _, staleFid := range staleFamilies {
		_ = s.session.Query(
			"DELETE FROM user_families WHERE user_id = ? AND family_id = ?",
			userID, staleFid,
		).WithContext(ctx).Exec()
	}

	// Return only active families
	activeFamilies := make([]string, 0, len(families)-len(staleFamilies))
	for _, fid := range families {
		isStale := false
		for _, staleFid := range staleFamilies {
			if fid == staleFid {
				isStale = true
				break
			}
		}
		if !isStale {
			activeFamilies = append(activeFamilies, fid)
		}
	}

	return activeFamilies, nil
}

// HashTokenSHA256 computes SHA256 hash of a token
func HashTokenSHA256(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum[:])
}
