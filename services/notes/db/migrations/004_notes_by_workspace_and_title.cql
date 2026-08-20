CREATE TABLE IF NOT EXISTS notes_by_workspace_and_title (
    workspace_id UUID,
    title        TEXT,
    note_id      UUID,
    created_by   UUID,
    is_private   BOOLEAN,
    created_at   TIMESTAMP,
    updated_at   TIMESTAMP,
    PRIMARY KEY (workspace_id, title)
) WITH CLUSTERING ORDER BY (title ASC);
