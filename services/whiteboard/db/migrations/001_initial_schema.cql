CREATE KEYSPACE IF NOT EXISTS notes_and_whiteboard WITH replication = {'class': 'NetworkTopologyStrategy', 'datacenter1': 1};

CREATE TABLE IF NOT EXISTS notes (
    workspace_id UUID,
    note_id UUID,
    title TEXT,
    created_by UUID,
    is_private BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    PRIMARY KEY (workspace_id, note_id)
) WITH CLUSTERING ORDER BY (note_id ASC);

CREATE TABLE IF NOT EXISTS notes_by_id (
    note_id UUID PRIMARY KEY,
    workspace_id UUID,
    title TEXT,
    created_by UUID,
    is_private BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS yjs_updates (
    note_id UUID,
    update_id TIMEUUID,
    data BLOB,
    user_id UUID,
    PRIMARY KEY (note_id, update_id)
) WITH CLUSTERING ORDER BY (update_id ASC)
AND compaction = {'class': 'TimeWindowCompactionStrategy', 'compaction_window_size': '1', 'compaction_window_unit': 'DAYS'};

CREATE TABLE IF NOT EXISTS yjs_snapshots (
    note_id UUID,
    snapshot_id TIMEUUID,
    state_vector BLOB,
    PRIMARY KEY (note_id, snapshot_id)
) WITH CLUSTERING ORDER BY (snapshot_id DESC);

CREATE TABLE IF NOT EXISTS boards (
    workspace_id UUID,
    board_id UUID,
    title TEXT,
    created_by UUID,
    is_private BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    PRIMARY KEY (workspace_id, board_id)
) WITH CLUSTERING ORDER BY (board_id ASC);

CREATE TABLE IF NOT EXISTS board_state (
    board_id UUID PRIMARY KEY,
    document BLOB,
    op_clock BIGINT,
    schema_version INT,
    updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS board_ops_hot (
    board_id UUID,
    op_clock BIGINT,
    op_data BLOB,
    user_id UUID,
    created_at TIMESTAMP,
    PRIMARY KEY (board_id, op_clock)
) WITH CLUSTERING ORDER BY (op_clock ASC)
AND default_time_to_live = 259200;

CREATE TABLE IF NOT EXISTS boards_by_id (
    board_id UUID PRIMARY KEY,
    workspace_id UUID,
    title TEXT,
    created_by UUID,
    is_private BOOLEAN,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS assets (
    asset_id TEXT PRIMARY KEY,
    board_id UUID,
    workspace_id UUID,
    name TEXT,
    mime_type TEXT,
    size_bytes BIGINT,
    s3_key TEXT,
    created_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ws_tickets (
    ticket TEXT PRIMARY KEY,
    user_id UUID,
    created_at TIMESTAMP
) WITH default_time_to_live = 30;
