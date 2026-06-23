CREATE TABLE IF NOT EXISTS task_tool_calls (
    id TEXT PRIMARY KEY,
    task_run_id TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
    mcp_server_id TEXT,
    mcp_tool_id TEXT,
    provider_tool_call_id TEXT,
    tool_name TEXT NOT NULL,
    arguments JSONB NOT NULL DEFAULT '{}'::jsonb,
    permission_decision TEXT NOT NULL DEFAULT 'not_required',
    state TEXT NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER,
    result TEXT,
    result_truncated BOOLEAN NOT NULL DEFAULT false,
    error_category TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_tool_calls_run_created ON task_tool_calls(task_run_id, created_at);
CREATE INDEX IF NOT EXISTS idx_task_tool_calls_state ON task_tool_calls(state);
