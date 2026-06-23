CREATE TABLE IF NOT EXISTS task_tool_calls (
    id TEXT PRIMARY KEY,
    task_run_id TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
    mcp_server_id TEXT,
    mcp_tool_id TEXT,
    provider_tool_call_id TEXT,
    tool_name TEXT NOT NULL,
    arguments TEXT NOT NULL DEFAULT '{}',
    permission_decision TEXT NOT NULL DEFAULT 'not_required',
    state TEXT NOT NULL DEFAULT 'pending',
    started_at TEXT,
    completed_at TEXT,
    duration_ms INTEGER,
    result TEXT,
    result_truncated INTEGER NOT NULL DEFAULT 0,
    error_category TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_task_tool_calls_run_created ON task_tool_calls(task_run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_task_tool_calls_state ON task_tool_calls(state);
