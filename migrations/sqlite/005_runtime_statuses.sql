CREATE TABLE IF NOT EXISTS runtime_statuses (
    component TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    instance_id TEXT,
    message TEXT,
    metadata TEXT NOT NULL DEFAULT '{}',
    last_seen_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_runtime_statuses_last_seen ON runtime_statuses(last_seen_at);
