CREATE TABLE IF NOT EXISTS note_assets (
    asset_id TEXT PRIMARY KEY,
    note_id UUID,
    workspace_id UUID,
    name TEXT,
    mime_type TEXT,
    size_bytes BIGINT,
    s3_key TEXT,
    created_at TIMESTAMP
);
