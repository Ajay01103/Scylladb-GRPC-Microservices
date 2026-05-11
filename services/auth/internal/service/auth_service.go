package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto"
	"go.uber.org/zap"

	"github.com/google/uuid"

	"github.com/Ajay01103/go-notion/auth/config"
	"github.com/Ajay01103/go-notion/auth/internal/repository"
	"github.com/Ajay01103/go-notion/auth/internal/scyllastore"
	"github.com/Ajay01103/go-notion/auth/internal/tokencache"
	"github.com/Ajay01103/go-notion/pkg/token"
)

// AuthService holds all dependencies needed by the auth business logic.
type AuthService struct {
	userRepo       *repository.UserRepo
	tokenMaker     token.TokenMaker
	sessionStore   *scyllastore.SessionStore
	revocationStore *scyllastore.SessionStore
	cache          *ristretto.Cache
	cfg            config.Config
	logger         *zap.Logger
}

// New creates an AuthService with its dependencies wired.
func New(
	userRepo *repository.UserRepo,
	tokenMaker token.TokenMaker,
	sessionStore *scyllastore.SessionStore,
	revocationStore *scyllastore.SessionStore,
	cache *ristretto.Cache,
	cfg config.Config,
	logger *zap.Logger,
) *AuthService {
	return &AuthService{
		userRepo:        userRepo,
		tokenMaker:      tokenMaker,
		sessionStore:    sessionStore,
		revocationStore: revocationStore,
		cache:           cache,
		cfg:             cfg,
		logger:          logger,
	}
}

// ─── Register ─────────────────────────────────────────────────────────────────

// RegisterResult contains the result of a successful registration
type RegisterResult struct {
	User         repository.User
	AccessToken  string
	RefreshToken string
}

func (s *AuthService) Register(ctx context.Context, email, name, password string) (*RegisterResult, error) {
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.CreateUser(ctx, email, name, hashed)
	if err != nil {
		if errors.Is(err, repository.ErrEmailTaken) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	accessToken, refreshToken, err := s.mintSessionTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}

	return &RegisterResult{User: user, AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// ─── Login ────────────────────────────────────────────────────────────────────

// LoginResult contains the result of a successful login
type LoginResult struct {
	User         repository.User
	AccessToken  string
	RefreshToken string
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if err := VerifyPassword(user.Password, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, refreshToken, err := s.mintSessionTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}

	return &LoginResult{User: user, AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

// ─── RefreshToken ─────────────────────────────────────────────────────────────

type RefreshResult struct {
	AccessToken  string
	RefreshToken string
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenStr string) (*RefreshResult, error) {
	// Parse session-mode refresh token
	payload, err := s.tokenMaker.VerifySessionRefreshToken(refreshTokenStr)
	if err != nil {
		if errors.Is(err, token.ErrExpiredToken) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	uid := payload.UserID.String()
	sid := payload.SessionID.String()

	// ── L1: Ristretto session cache ────────────────────────────────────────
	var sessionGen int64
	var minGlobalVer int
	if cached, ok := s.cache.Get(tokencache.SessionKey(sid)); ok {
		if entry, ok := cached.(*CachedSession); ok {
			sessionGen = entry.Gen
			minGlobalVer = entry.MinGlobalVer
			// Verify generation counter (replay protection)
			if payload.Gen != sessionGen {
				s.logger.Warn("replay detected: gen mismatch",
					zap.String("userID", uid),
					zap.String("sessionID", sid),
					zap.Int64("expected", sessionGen),
					zap.Int64("got", payload.Gen),
				)
				go s.handleTheftDetected(ctx, uid, sid)
				return nil, ErrReplayDetected
			}
			// Check user revocation from cache
			if payload.GlobalVer < minGlobalVer {
				s.logger.Info("globally revoked refresh attempt",
					zap.String("userID", uid),
					zap.Int("storedGv", minGlobalVer),
					zap.Int("tokenGv", payload.GlobalVer),
				)
				return nil, ErrTokenRevoked
			}
			// Cache hit: mint and update cache
			return s.issueAndUpdateSessionCache(ctx, uid, sid, sessionGen, minGlobalVer)
		}
	}

	// ── L2: ScyllaDB (cache miss only) ─────────────────────────────────────
	sessionRec, err := s.sessionStore.GetSession(ctx, uid, sid)
	if err != nil || sessionRec == nil {
		s.logger.Warn("session not found during refresh",
			zap.String("userID", uid),
			zap.String("sessionID", sid),
		)
		return nil, ErrSessionNotFound
	}

	// Verify generation counter
	if payload.Gen != sessionRec.Gen {
		s.logger.Warn("replay detected: gen mismatch (from Scylla)",
			zap.String("userID", uid),
			zap.String("sessionID", sid),
		)
		go s.handleTheftDetected(ctx, uid, sid)
		return nil, ErrReplayDetected
	}

	// Get user global version
	userRev, err := s.revocationStore.GetUserRevocation(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("get user revocation: %w", err)
	}
	globalVer := 0
	if userRev != nil {
		globalVer = userRev.GlobalVer
	}

	// Check revocation
	if payload.GlobalVer < globalVer {
		return nil, ErrTokenRevoked
	}

	// Atomic CAS update: bump generation
	newGen := sessionRec.Gen + 1
	ok, err := s.sessionStore.AtomicBumpGen(ctx, uid, sid, sessionRec.Gen, newGen)
	if err != nil {
		return nil, fmt.Errorf("bump session gen: %w", err)
	}
	if !ok {
		s.logger.Warn("concurrent refresh detected: gen mismatch on CAS",
			zap.String("userID", uid),
			zap.String("sessionID", sid),
		)
		return nil, ErrConcurrentRotation
	}

	// Warm cache with new generation
	s.cache.SetWithTTL(
		tokencache.SessionKey(sid),
		&CachedSession{Gen: newGen, MinGlobalVer: globalVer},
		tokencache.SessionStateCost,
		tokencache.SessionStateTTL,
	)

	// Get user for token claims (optional for session-mode tokens)
	userID, err := uuid.Parse(uid)
	if err != nil {
		return nil, fmt.Errorf("parse user id: %w", err)
	}
	_, err = s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Mint new token pair
	newRefreshStr, _, err := s.tokenMaker.CreateSessionRefreshToken(
		uid, sid, newGen, globalVer, s.cfg.RefreshTokenDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("create session refresh token: %w", err)
	}

	newAccessStr, _, err := s.tokenMaker.CreateSessionAccessToken(
		uid, []string{"user"}, globalVer, s.cfg.AccessTokenDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("create session access token: %w", err)
	}

	return &RefreshResult{AccessToken: newAccessStr, RefreshToken: newRefreshStr}, nil
}

// ─── Logout ───────────────────────────────────────────────────────────────────

func (s *AuthService) Logout(ctx context.Context, refreshTokenStr string) error {
	payload, err := s.tokenMaker.VerifySessionRefreshToken(refreshTokenStr)
	if err != nil {
		if errors.Is(err, token.ErrExpiredToken) {
			return ErrTokenExpired
		}
		return ErrInvalidToken
	}

	sid := payload.SessionID.String()

	// Delete session from cache
	s.cache.Del(tokencache.SessionKey(sid))

	// Delete session from Scylla (TTL will eventually clean it up, but we can accelerate)
	// For now, we rely on TTL; explicit delete is optional

	return nil
}

func (s *AuthService) LogoutAllDevices(ctx context.Context, userID string) error {
	if _, err := uuid.Parse(userID); err != nil {
		return ErrInvalidToken
	}

	// Bump user global version (invalidates all tokens)
	newGv, err := s.revocationStore.BumpUserGlobalVer(ctx, userID)
	if err != nil {
		return fmt.Errorf("bump global version: %w", err)
	}

	// Warm cache with new global version
	s.cache.SetWithTTL(
		tokencache.GlobalVerKey(userID),
		newGv,
		tokencache.GlobalVerCost,
		tokencache.GlobalVerTTL,
	)

	s.logger.Info("user logged out all devices", zap.String("userID", userID), zap.Int("newGv", newGv))
	return nil
}

// ─── ValidateToken ────────────────────────────────────────────────────────────

type ValidateResult struct {
	UserID uuid.UUID
	Email  string
	Name   string
}

func (s *AuthService) ValidateToken(ctx context.Context, accessTokenStr string) (*ValidateResult, error) {
	// Pure crypto — no DB call, no cache required for AT signature validation
	payload, err := s.tokenMaker.VerifySessionAccessToken(accessTokenStr)
	if err != nil {
		if errors.Is(err, token.ErrExpiredToken) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	// ── Optional: Check global version from cache only (no Scylla) ─────────
	// If global_ver in token is less than cached global_ver, token is revoked
	if cached, ok := s.cache.Get(tokencache.GlobalVerKey(payload.UserID.String())); ok {
		if storedGv, ok := cached.(int); ok {
			if payload.GlobalVer < storedGv {
				return nil, ErrTokenRevoked
			}
		}
	}
	// Cache miss: token passes (stale window accepted per design)

	return &ValidateResult{
		UserID: payload.UserID,
		Email:  "",        // session tokens don't carry email
		Name:   "",        // session tokens don't carry name
	}, nil
}

// GetUserByID fetches user details by ID (used by GetCurrentUser endpoint)
func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*repository.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// ─── Private helpers ──────────────────────────────────────────────────────────

// CachedSession stores session state in Ristretto for fast rotation checks.
type CachedSession struct {
	Gen          int64
	MinGlobalVer int
}

func (s *AuthService) mintSessionTokenPair(ctx context.Context, user repository.User) (accessToken, refreshToken string, err error) {
	// Create a new session with gen=1
	sessionID := uuid.NewString()
	sessionRec := scyllastore.SessionRecord{
		UserID:    user.ID,
		SessionID: sessionID,
		Gen:       1,
		DeviceFP:  "",
		ExpiresAt: time.Now().UTC().Add(s.cfg.RefreshTokenDuration),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.sessionStore.StoreSession(ctx, sessionRec, s.cfg.RefreshTokenDuration); err != nil {
		return "", "", fmt.Errorf("store session: %w", err)
	}

	// Get or get user global version (will be 0 if not yet created)
	userRev, err := s.revocationStore.GetUserRevocation(ctx, user.ID)
	if err != nil {
		return "", "", fmt.Errorf("get user revocation: %w", err)
	}
	globalVer := 0
	if userRev != nil {
		globalVer = userRev.GlobalVer
	}

	// Mint session-mode refresh token
	refreshStr, _, err := s.tokenMaker.CreateSessionRefreshToken(
		user.ID, sessionID, 1, globalVer, s.cfg.RefreshTokenDuration,
	)
	if err != nil {
		return "", "", fmt.Errorf("create session refresh token: %w", err)
	}

	// Mint session-mode access token
	accessStr, _, err := s.tokenMaker.CreateSessionAccessToken(
		user.ID, []string{"user"}, globalVer, s.cfg.AccessTokenDuration,
	)
	if err != nil {
		return "", "", fmt.Errorf("create session access token: %w", err)
	}

	// Cache the session state
	s.cache.SetWithTTL(
		tokencache.SessionKey(sessionID),
		&CachedSession{Gen: 1, MinGlobalVer: globalVer},
		tokencache.SessionStateCost,
		tokencache.SessionStateTTL,
	)

	return accessStr, refreshStr, nil
}

func (s *AuthService) issueAndUpdateSessionCache(
	ctx context.Context,
	userID, sessionID string,
	currentGen int64,
	currentGv int,
) (*RefreshResult, error) {
	// Bump generation via CAS
	newGen := currentGen + 1
	ok, err := s.sessionStore.AtomicBumpGen(ctx, userID, sessionID, currentGen, newGen)
	if err != nil {
		return nil, fmt.Errorf("bump session gen: %w", err)
	}
	if !ok {
		s.logger.Warn("concurrent refresh during cache hit",
			zap.String("userID", userID),
			zap.String("sessionID", sessionID),
		)
		return nil, ErrConcurrentRotation
	}

	// Mint new tokens
	newRefreshStr, _, err := s.tokenMaker.CreateSessionRefreshToken(
		userID, sessionID, newGen, currentGv, s.cfg.RefreshTokenDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("create session refresh token: %w", err)
	}

	newAccessStr, _, err := s.tokenMaker.CreateSessionAccessToken(
		userID, []string{"user"}, currentGv, s.cfg.AccessTokenDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("create session access token: %w", err)
	}

	// Refresh cache
	s.cache.SetWithTTL(
		tokencache.SessionKey(sessionID),
		&CachedSession{Gen: newGen, MinGlobalVer: currentGv},
		tokencache.SessionStateCost,
		tokencache.SessionStateTTL,
	)

	return &RefreshResult{AccessToken: newAccessStr, RefreshToken: newRefreshStr}, nil
}

func (s *AuthService) handleTheftDetected(ctx context.Context, userID, sessionID string) {
	// Non-negotiable: kill the session immediately on replay detection
	s.logger.Warn("TOKEN REPLAY DETECTED - KILLING SESSION",
		zap.String("userID", userID),
		zap.String("sessionID", sessionID),
	)

	// Delete session from Scylla (best effort, async cleanup)
	// This is fire-and-forget; logging only on error
	if err := s.sessionStore.DeleteSession(ctx, userID, sessionID); err != nil {
		s.logger.Error("failed to delete compromised session from DB",
			zap.String("userID", userID),
			zap.String("sessionID", sessionID),
			zap.Error(err),
		)
	}

	// Evict from cache immediately (must succeed)
	s.cache.Del(tokencache.SessionKey(sessionID))

	// TODO: emit audit event for security team
	// TODO: optionally bump global_ver to revoke all user sessions on repeated theft
}
