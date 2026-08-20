-- Lookup table for GetBoardBySlug RPC.
-- Mirrors notes_by_workspace_and_title (notes migration 004).
-- Partition by workspace_id, cluster by title for O(1) slug lookups.

CREATE TABLE IF NOT EXISTS boards_by_workspace_and_title (
    workspace_id UUID,
    title        TEXT,
    board_id     UUID,
    created_by   UUID,
    is_private   BOOLEAN,
    created_at   TIMESTAMP,
    updated_at   TIMESTAMP,
    PRIMARY KEY (workspace_id, title)
) WITH CLUSTERING ORDER BY (title ASC);
