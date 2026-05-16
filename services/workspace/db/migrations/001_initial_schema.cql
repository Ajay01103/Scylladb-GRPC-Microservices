CREATE KEYSPACE IF NOT EXISTS workspace_ks WITH replication = {'class': 'NetworkTopologyStrategy', 'datacenter1': 1};

CREATE TABLE IF NOT EXISTS workspaces (
    id           TEXT PRIMARY KEY,
    name         TEXT,
    slug         TEXT,
    description  TEXT,
    icon_url     TEXT,
    owner_id     TEXT,
    is_public    BOOLEAN,
    created_at   TIMESTAMP,
    updated_at   TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workspace_names (
    name         TEXT PRIMARY KEY,
    workspace_id TEXT,
    created_at   TIMESTAMP
);

-- Slug index: workspace_slug_to_id for slug-based lookups
-- Using a reverse lookup table for efficient slug queries
CREATE TABLE IF NOT EXISTS workspace_slugs (
    slug         TEXT PRIMARY KEY,
    workspace_id TEXT,
    created_at   TIMESTAMP
);

-- Workspace members: tracks user membership and roles in workspaces
-- Partitioned by workspace_id for efficient member listing
CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id  TEXT,
    user_id       TEXT,
    role          TEXT,
    invited_by    TEXT,
    joined_at     TIMESTAMP,
    PRIMARY KEY (workspace_id, user_id)
) WITH CLUSTERING ORDER BY (user_id ASC);

-- Workspace members by user: denormalized table for listing user's workspaces
-- Partitioned by user_id for efficient ListMyWorkspaces queries
CREATE TABLE IF NOT EXISTS workspace_members_by_user (
    user_id       TEXT,
    workspace_id  TEXT,
    role          TEXT,
    invited_by    TEXT,
    joined_at     TIMESTAMP,
    PRIMARY KEY (user_id, workspace_id)
) WITH CLUSTERING ORDER BY (workspace_id ASC);

-- Workspace invitations: tracks pending and accepted invitations
-- Token is the primary key for quick accept lookups
CREATE TABLE IF NOT EXISTS workspace_invitations (
    "token"      TEXT PRIMARY KEY,
    workspace_id  TEXT,
    invited_email TEXT,
    role          TEXT,
    invited_by    TEXT,
    expires_at    TIMESTAMP,
    accepted_at   TIMESTAMP,
    created_at    TIMESTAMP
);

-- Workspace invitations by workspace: for listing pending invitations
-- Useful for showing invite history and cleanup operations
CREATE TABLE IF NOT EXISTS workspace_invitations_by_workspace (
    workspace_id  TEXT,
    "token"       TEXT,
    invited_email TEXT,
    role          TEXT,
    invited_by    TEXT,
    expires_at    TIMESTAMP,
    accepted_at   TIMESTAMP,
    created_at    TIMESTAMP,
    PRIMARY KEY (workspace_id, "token")
) WITH CLUSTERING ORDER BY ("token" ASC);
