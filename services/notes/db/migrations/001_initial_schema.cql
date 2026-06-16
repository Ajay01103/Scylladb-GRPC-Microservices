CREATE KEYSPACE IF NOT EXISTS notes_ks WITH replication = {'class': 'NetworkTopologyStrategy', 'datacenter1': 1};

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

CREATE TABLE IF NOT EXISTS yjs_state (
    note_id UUID PRIMARY KEY,
    content BLOB,
    updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ws_tickets (
    ticket TEXT PRIMARY KEY,
    user_id UUID,
    created_at TIMESTAMP
) WITH default_time_to_live = 120;
