CREATE TABLE IF NOT EXISTS runtime_statuses (
    component TEXT PRIMARY KEY,
    status TEXT NOT NULL,
    instance_id TEXT,
    message TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_runtime_statuses_last_seen ON runtime_statuses(last_seen_at);
