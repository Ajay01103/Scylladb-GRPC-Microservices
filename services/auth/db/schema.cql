-- Create keyspace
CREATE KEYSPACE auth_ks
    WITH replication = {'class': 'NetworkTopologyStrategy', 'datacenter1': 3};

USE auth_ks;

-- ─── Users Table ──────────────────────────────────────────────────────────────

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    email TEXT,
    name TEXT,
    password TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) WITH compaction = {'class': 'LeveledCompactionStrategy'}
  AND caching = {'keys': 'ALL', 'rows_per_partition': '1'};

-- ─── Materialized View: users_by_email_mv ────────────────────────────────────
-- Enables fast email-based lookups without intermediate index table
-- Single query instead of 2-hop lookup: users_by_email → users
CREATE MATERIALIZED VIEW users_by_email_mv AS
    SELECT id, email, name, password, created_at, updated_at
    FROM users
    WHERE email IS NOT NULL
    PRIMARY KEY (email, id);

-- DEPRECATED: Prefer users_by_email_mv for lookups. This legacy table is marked for removal.
-- TODO: Remove after verifying no external dependencies.

-- ─── Session Generation State ────────────────────────────────────────────────
-- TTL: 7 days (default on table)
-- On writes (rotation): USING TTL 604800 resets TTL, ensuring row lives exactly 7D from last use
-- Clustering order: ASC on session_id for efficient range scans on multi-session queries
-- Compaction: LeveledCompactionStrategy for better point reads/writes

CREATE TABLE user_sessions (
    user_id TEXT,
    session_id TEXT,
    gen BIGINT,
    device_fp TEXT,
    expires_at TIMESTAMP,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    PRIMARY KEY (user_id, session_id)
) WITH CLUSTERING ORDER BY (session_id ASC)
  AND default_time_to_live = 604800
  AND compaction = {'class': 'LeveledCompactionStrategy'};

-- ─── User Revocation State ───────────────────────────────────────────────────
-- TTL: 7 days (matches session lifetime)
-- On writes: USING TTL 604800 resets TTL, preventing orphaned rows

CREATE TABLE user_revocations (
    user_id TEXT PRIMARY KEY,
    global_ver INT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) WITH default_time_to_live = 604800;

-- ─── EdDSA Signing Keys Table ─────────────────────────────────────────────────
-- Stores EdDSA private keys for JWT signing
-- Keys are rotated regularly; old keys kept for token validation grace period
-- TTL: 30 days (longer than session to ensure old keys can validate old tokens)
-- Read-heavy, rarely written → LeveledCompactionStrategy

CREATE TABLE signing_keys (
    kid TEXT PRIMARY KEY,
    private_key BLOB,          -- Encoded private key bytes
    public_key BLOB,           -- Encoded public key bytes
    algorithm TEXT,            -- "EdDSA" (Ed25519)
    status TEXT,               -- "active" or "rotated"
    created_at TIMESTAMP,
    expires_at TIMESTAMP
) WITH compaction = {'class': 'LeveledCompactionStrategy'}
  AND default_time_to_live = 2592000;

-- ─── JWKS Public Keys Cache ───────────────────────────────────────────────────

-- Stores the exported JWKS format (JSON) for public key endpoints
-- Refreshed whenever new keys are generated
-- TTL: same as key retention (336 hours)
CREATE TABLE jwks_public_keys (
    id TEXT PRIMARY KEY,
    jwks_json TEXT,            -- Serialized JWKS ({"keys": [...]})
    version BIGINT,            -- Version counter for cache busting
    created_at TIMESTAMP,
    updated_at TIMESTAMP
) WITH default_time_to_live = 1209600;  -- 336 hours
