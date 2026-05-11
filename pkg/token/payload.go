package token

import (
	"encoding/json"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenType distinguishes between access and refresh tokens in claims.
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// AccessPayload holds the JWT claims for an access token.
//
// Fields:
//   - JTI:        unique ID for this specific access token
//   - RefreshJTI: the JTI of the paired refresh token stored in Redis.
//     When the client calls RefreshToken, the server looks up
//     Redis key "refresh_token:{RefreshJTI}" to check revocation.
//   - KeyID:      the kid of the RSA key used to sign this token
type AccessPayload struct {
	JTI        uuid.UUID `json:"jti"`
	UserID     uuid.UUID `json:"sub"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	TokenType  TokenType `json:"token_type"`
	FamilyID   uuid.UUID `json:"family_id"`
	RefreshJTI uuid.UUID `json:"refresh_jti"`
	IssuedAt   time.Time `json:"iat"`
	ExpiredAt  time.Time `json:"exp"`
	KeyID      string    `json:"kid,omitempty"`
}

// AccessTokenClaims is a typed JWT claim set for access tokens.
type AccessTokenClaims struct {
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	TokenType  TokenType `json:"token_type"`
	FamilyID   string    `json:"family_id"`
	RefreshJTI string    `json:"refresh_jti"`

	jwt.RegisteredClaims
}

// Reset clears all fields before this claim object is reused from a pool.
func (c *AccessTokenClaims) Reset() {
	*c = AccessTokenClaims{}
}

// NewAccessPayload creates a new AccessPayload for the given user.
// refreshJTI must be the JTI of the refresh token this access token is paired with.
func NewAccessPayload(userID uuid.UUID, email, name string, familyID, refreshJTI uuid.UUID, duration time.Duration) (*AccessPayload, error) {
	return NewAccessPayloadAt(userID, email, name, familyID, refreshJTI, time.Now().UTC(), duration)
}

// NewAccessPayloadAt creates a new AccessPayload using a provided timestamp.
func NewAccessPayloadAt(userID uuid.UUID, email, name string, familyID, refreshJTI uuid.UUID, now time.Time, duration time.Duration) (*AccessPayload, error) {
	now = now.UTC()
	return &AccessPayload{
		JTI:        uuid.New(),
		UserID:     userID,
		Email:      email,
		Name:       name,
		TokenType:  TokenTypeAccess,
		FamilyID:   familyID,
		RefreshJTI: refreshJTI,
		IssuedAt:   now,
		ExpiredAt:  now.Add(duration),
	}, nil
}

func (p *AccessPayload) FillClaims(claims *AccessTokenClaims) {
	claims.Email = p.Email
	claims.Name = p.Name
	claims.TokenType = p.TokenType
	claims.FamilyID = p.FamilyID.String()
	claims.RefreshJTI = p.RefreshJTI.String()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ID:        p.JTI.String(),
		Subject:   p.UserID.String(),
		IssuedAt:  jwt.NewNumericDate(p.IssuedAt),
		ExpiresAt: jwt.NewNumericDate(p.ExpiredAt),
	}
}

func accessPayloadFromTokenClaims(claims *AccessTokenClaims) (*AccessPayload, error) {
	if claims == nil || claims.TokenType != TokenTypeAccess {
		return nil, ErrInvalidToken
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return nil, ErrInvalidToken
	}

	jti, err := uuid.Parse(claims.ID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, ErrInvalidToken
	}
	familyID, err := uuid.Parse(claims.FamilyID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	refreshJTI, err := uuid.Parse(claims.RefreshJTI)
	if err != nil {
		return nil, ErrInvalidToken
	}

	payload := &AccessPayload{
		JTI:        jti,
		UserID:     uid,
		Email:      claims.Email,
		Name:       claims.Name,
		TokenType:  claims.TokenType,
		FamilyID:   familyID,
		RefreshJTI: refreshJTI,
		IssuedAt:   claims.IssuedAt.Time,
		ExpiredAt:  claims.ExpiresAt.Time,
	}

	if time.Now().After(payload.ExpiredAt) {
		return nil, ErrExpiredToken
	}

	return payload, nil
}

// Valid implements the jwt.Claims interface.
func (p *AccessPayload) Valid() error {
	if time.Now().After(p.ExpiredAt) {
		return ErrExpiredToken
	}
	return nil
}

// RefreshPayload holds the JWT claims for a refresh token.
//
// Fields:
//   - JTI: unique ID stored as the Redis key "refresh_token:{JTI}".
//     Revoking a refresh token means deleting this key from Redis.
//   - KeyID: the kid of the RSA key used to sign this token
type RefreshPayload struct {
	JTI       uuid.UUID `json:"jti"`
	UserID    uuid.UUID `json:"sub"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	TokenType TokenType `json:"token_type"`
	FamilyID  uuid.UUID `json:"family_id"`
	IssuedAt  time.Time `json:"iat"`
	ExpiredAt time.Time `json:"exp"`
	KeyID     string    `json:"kid,omitempty"`
}

// RefreshTokenClaims is a typed JWT claim set for refresh tokens.
type RefreshTokenClaims struct {
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	TokenType TokenType `json:"token_type"`
	FamilyID  string    `json:"family_id"`

	jwt.RegisteredClaims
}

// Reset clears all fields before this claim object is reused from a pool.
func (c *RefreshTokenClaims) Reset() {
	*c = RefreshTokenClaims{}
}

// NewRefreshPayload creates a new RefreshPayload for the given user.
func NewRefreshPayload(userID uuid.UUID, email, name string, familyID uuid.UUID, duration time.Duration) (*RefreshPayload, error) {
	return NewRefreshPayloadAt(userID, email, name, familyID, time.Now().UTC(), duration)
}

// NewRefreshPayloadAt creates a new RefreshPayload using a provided timestamp.
func NewRefreshPayloadAt(userID uuid.UUID, email, name string, familyID uuid.UUID, now time.Time, duration time.Duration) (*RefreshPayload, error) {
	now = now.UTC()
	return &RefreshPayload{
		JTI:       uuid.New(),
		UserID:    userID,
		Email:     email,
		Name:      name,
		TokenType: TokenTypeRefresh,
		FamilyID:  familyID,
		IssuedAt:  now,
		ExpiredAt: now.Add(duration),
	}, nil
}

func (p *RefreshPayload) FillClaims(claims *RefreshTokenClaims) {
	claims.Email = p.Email
	claims.Name = p.Name
	claims.TokenType = p.TokenType
	claims.FamilyID = p.FamilyID.String()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ID:        p.JTI.String(),
		Subject:   p.UserID.String(),
		IssuedAt:  jwt.NewNumericDate(p.IssuedAt),
		ExpiresAt: jwt.NewNumericDate(p.ExpiredAt),
	}
}

func refreshPayloadFromTokenClaims(claims *RefreshTokenClaims) (*RefreshPayload, error) {
	if claims == nil || claims.TokenType != TokenTypeRefresh {
		return nil, ErrInvalidToken
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return nil, ErrInvalidToken
	}

	jti, err := uuid.Parse(claims.ID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, ErrInvalidToken
	}
	familyID, err := uuid.Parse(claims.FamilyID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	payload := &RefreshPayload{
		JTI:       jti,
		UserID:    uid,
		Email:     claims.Email,
		Name:      claims.Name,
		TokenType: claims.TokenType,
		FamilyID:  familyID,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiredAt: claims.ExpiresAt.Time,
	}

	if time.Now().After(payload.ExpiredAt) {
		return nil, ErrExpiredToken
	}

	return payload, nil
}

// Valid implements the jwt.Claims interface.
func (p *RefreshPayload) Valid() error {
	if time.Now().After(p.ExpiredAt) {
		return ErrExpiredToken
	}
	return nil
}

// SessionAccessPayload is the access-token payload for the session/gen model.
type SessionAccessPayload struct {
	JTI       uuid.UUID `json:"jti"`
	UserID    uuid.UUID `json:"sub"`
	Roles     []string  `json:"roles"`
	GlobalVer int       `json:"gv"`
	TokenType TokenType `json:"token_type"`
	IssuedAt  time.Time `json:"iat"`
	ExpiredAt time.Time `json:"exp"`
}

// SessionAccessClaims is the JWT claim set for session-mode access tokens.
type SessionAccessClaims struct {
	TokenType TokenType `json:"token_type"`
	Roles     []string `json:"roles"`
	GlobalVer int      `json:"gv"`

	jwt.RegisteredClaims
}

// SessionRefreshPayload is the refresh-token payload for the session/gen model.
type SessionRefreshPayload struct {
	JTI       uuid.UUID `json:"jti"`
	UserID    uuid.UUID `json:"sub"`
	SessionID uuid.UUID `json:"sid"`
	Gen       int64     `json:"gen"`
	GlobalVer int       `json:"gv"`
	TokenType TokenType `json:"token_type"`
	IssuedAt  time.Time `json:"iat"`
	ExpiredAt time.Time `json:"exp"`
}

// SessionRefreshClaims is the JWT claim set for session-mode refresh tokens.
type SessionRefreshClaims struct {
	TokenType TokenType `json:"token_type"`
	SessionID string `json:"sid"`
	Gen       int64  `json:"gen"`
	GlobalVer int    `json:"gv"`

	jwt.RegisteredClaims
}

// Reset clears all fields before this claim object is reused from a pool.
func (c *SessionAccessClaims) Reset() {
	*c = SessionAccessClaims{}
}

// Reset clears all fields before this claim object is reused from a pool.
func (c *SessionRefreshClaims) Reset() {
	*c = SessionRefreshClaims{}
}

// NewSessionAccessPayload creates a session-mode access payload.
func NewSessionAccessPayload(userID uuid.UUID, roles []string, globalVer int, duration time.Duration) (*SessionAccessPayload, error) {
	return NewSessionAccessPayloadAt(userID, roles, globalVer, time.Now().UTC(), duration)
}

// NewSessionAccessPayloadAt creates a session-mode access payload at a provided time.
func NewSessionAccessPayloadAt(userID uuid.UUID, roles []string, globalVer int, now time.Time, duration time.Duration) (*SessionAccessPayload, error) {
	now = now.UTC()
	return &SessionAccessPayload{
		JTI:       uuid.New(),
		UserID:    userID,
		Roles:     append([]string(nil), roles...),
		GlobalVer: globalVer,
		TokenType: TokenTypeAccess,
		IssuedAt:  now,
		ExpiredAt: now.Add(duration),
	}, nil
}

// FillClaims copies session-mode access payload into JWT claims.
func (p *SessionAccessPayload) FillClaims(claims *SessionAccessClaims) {
	claims.Roles = append([]string(nil), p.Roles...)
	claims.GlobalVer = p.GlobalVer
	claims.TokenType = p.TokenType
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ID:        p.JTI.String(),
		Subject:   p.UserID.String(),
		IssuedAt:  jwt.NewNumericDate(p.IssuedAt),
		ExpiresAt: jwt.NewNumericDate(p.ExpiredAt),
	}
}

// Valid implements the jwt.Claims interface.
func (p *SessionAccessPayload) Valid() error {
	if time.Now().After(p.ExpiredAt) {
		return ErrExpiredToken
	}
	return nil
}

// NewSessionRefreshPayload creates a session-mode refresh payload.
func NewSessionRefreshPayload(userID, sessionID uuid.UUID, gen int64, globalVer int, duration time.Duration) (*SessionRefreshPayload, error) {
	return NewSessionRefreshPayloadAt(userID, sessionID, gen, globalVer, time.Now().UTC(), duration)
}

// NewSessionRefreshPayloadAt creates a session-mode refresh payload at a provided time.
func NewSessionRefreshPayloadAt(userID, sessionID uuid.UUID, gen int64, globalVer int, now time.Time, duration time.Duration) (*SessionRefreshPayload, error) {
	now = now.UTC()
	return &SessionRefreshPayload{
		JTI:       uuid.New(),
		UserID:    userID,
		SessionID: sessionID,
		Gen:       gen,
		GlobalVer: globalVer,
		TokenType: TokenTypeRefresh,
		IssuedAt:  now,
		ExpiredAt: now.Add(duration),
	}, nil
}

// FillClaims copies session-mode refresh payload into JWT claims.
func (p *SessionRefreshPayload) FillClaims(claims *SessionRefreshClaims) {
	claims.SessionID = p.SessionID.String()
	claims.Gen = p.Gen
	claims.GlobalVer = p.GlobalVer
	claims.TokenType = p.TokenType
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ID:        p.JTI.String(),
		Subject:   p.UserID.String(),
		IssuedAt:  jwt.NewNumericDate(p.IssuedAt),
		ExpiresAt: jwt.NewNumericDate(p.ExpiredAt),
	}
}

// Valid implements the jwt.Claims interface.
func (p *SessionRefreshPayload) Valid() error {
	if time.Now().After(p.ExpiredAt) {
		return ErrExpiredToken
	}
	return nil
}

func sessionAccessPayloadFromClaims(claims *SessionAccessClaims) (*SessionAccessPayload, error) {
	if claims == nil || claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return nil, ErrInvalidToken
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return &SessionAccessPayload{
		JTI:       uuid.MustParse(claims.ID),
		UserID:    uid,
		Roles:     append([]string(nil), claims.Roles...),
		GlobalVer: claims.GlobalVer,
		TokenType: claims.TokenType,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiredAt: claims.ExpiresAt.Time,
	}, nil
}

func sessionRefreshPayloadFromClaims(claims *SessionRefreshClaims) (*SessionRefreshPayload, error) {
	if claims == nil || claims.ExpiresAt == nil || claims.IssuedAt == nil {
		return nil, ErrInvalidToken
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, ErrInvalidToken
	}
	sid, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return nil, ErrInvalidToken
	}
	return &SessionRefreshPayload{
		JTI:       uuid.MustParse(claims.ID),
		UserID:    uid,
		SessionID: sid,
		Gen:       claims.Gen,
		GlobalVer: claims.GlobalVer,
		TokenType: claims.TokenType,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiredAt: claims.ExpiresAt.Time,
	}, nil
}

// encodeSessionRoles returns a JSON-compatible copy for MapClaims fallback.
func encodeSessionRoles(roles []string) []string {
	if len(roles) == 0 {
		return nil
	}
	return append([]string(nil), roles...)
}

func decodeStringSliceClaim(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	if typed, ok := raw.([]string); ok {
		return append([]string(nil), typed...), nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, ErrInvalidToken
	}
	roles := make([]string, 0, len(items))
	for _, item := range items {
		role, ok := item.(string)
		if !ok {
			return nil, ErrInvalidToken
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// JSON helper kept local so session-mode claims can use either typed claims or MapClaims.
func marshalSessionRoles(roles []string) (any, error) {
	if len(roles) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(roles)
	if err != nil {
		return nil, err
	}
	var decoded []string
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}
