-- ScyllaDB Schema for Auth Service
-- This schema replaces both PostgreSQL (users) and Redis (token families, JWKS)

-- Create keyspace
CREATE KEYSPACE IF NOT EXISTS auth_ks
  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

USE auth_ks;

-- ─── Users Table ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT,
    name TEXT,
    password TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) WITH CLUSTERING ORDER BY (id ASC);

-- Indices for user lookups
CREATE INDEX IF NOT EXISTS users_email_idx ON users (email);
CREATE INDEX IF NOT EXISTS users_name_idx ON users (name);

-- ─── Refresh Token Family Table ───────────────────────────────────────────────

-- Stores the active refresh token state for each family
-- TTL handles automatic expiration (same as REFRESH_TOKEN_DURATION, e.g., 168h = 604800s)
CREATE TABLE IF NOT EXISTS refresh_token_families (
    family_id TEXT PRIMARY KEY,
    user_id TEXT,
    token_hash TEXT,
    jkt TEXT,           -- DPoP key thumbprint (optional)
    expires_at TEXT,    -- ISO timestamp when token expires
    refresh_jti TEXT,   -- JTI claim from the refresh token
    signing_kid TEXT,   -- Key ID used for signing
    issued_at BIGINT,   -- Unix timestamp when issued
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) WITH default_time_to_live = 604800;  -- 7 days (refresh token duration)

-- Index for user family lookup
CREATE INDEX IF NOT EXISTS refresh_token_families_user_idx ON refresh_token_families (user_id);

-- ─── Refresh Token Blacklist Table ────────────────────────────────────────────

-- Stores revoked/rotated token hashes to prevent replay attacks
-- TTL: same as REFRESH_TOKEN_DURATION (or slightly longer for safety margin)
CREATE TABLE IF NOT EXISTS refresh_token_blacklist (
    family_id TEXT,
    token_hash TEXT,
    revoked_at TIMESTAMP,
    PRIMARY KEY (family_id, token_hash)
) WITH default_time_to_live = 604800;  -- 7 days

-- ─── Refresh Token Rotation Grace Window ──────────────────────────────────────

-- Stores the new family_id during grace period after rotation
-- Handles concurrent refresh races: if a client replays the old token within
-- the grace window, they receive the same new family as the successor.
-- TTL: 15 seconds (rotatedGraceTTL)
CREATE TABLE IF NOT EXISTS token_rotation_grace (
    family_id TEXT,
    old_token_hash TEXT,
    new_family_id TEXT,
    created_at TIMESTAMP,
    PRIMARY KEY (family_id, old_token_hash)
) WITH default_time_to_live = 15;  -- 15 seconds grace window

-- ─── User Families Mapping ────────────────────────────────────────────────────

-- Maps users to their active token families
-- Used for logout-all-devices and family enumeration
CREATE TABLE IF NOT EXISTS user_families (
    user_id TEXT,
    family_id TEXT,
    added_at TIMESTAMP,
    PRIMARY KEY (user_id, family_id)
);

-- ─── EdDSA Signing Keys Table ─────────────────────────────────────────────────

-- Stores EdDSA private keys for JWT signing
-- TTL: EDDSA_SIGNING_KEY_RETENTION_DURATION (e.g., 336 hours = 1,209,600 seconds)
-- Keys are rotated regularly; old keys kept for token validation grace period
CREATE TABLE IF NOT EXISTS signing_keys (
    kid TEXT PRIMARY KEY,
    private_key BLOB,          -- Encoded private key bytes
    public_key BLOB,           -- Encoded public key bytes
    algorithm TEXT,            -- "EdDSA" (Ed25519)
    status TEXT,               -- "active" or "rotated"
    created_at TIMESTAMP,
    expires_at TIMESTAMP
) WITH default_time_to_live = 1209600;  -- 336 hours (EDDSA key retention)

-- Index for active keys lookup
CREATE INDEX IF NOT EXISTS signing_keys_status_idx ON signing_keys (status);

-- ─── JWKS Public Keys Cache ───────────────────────────────────────────────────

-- Stores the exported JWKS format (JSON) for public key endpoints
-- Refreshed whenever new keys are generated
-- TTL: same as key retention (336 hours)
CREATE TABLE IF NOT EXISTS jwks_public_keys (
    id TEXT PRIMARY KEY,
    jwks_json TEXT,            -- Serialized JWKS ({"keys": [...]})
    version BIGINT,            -- Version counter for cache busting
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) WITH default_time_to_live = 1209600;  -- 336 hours
