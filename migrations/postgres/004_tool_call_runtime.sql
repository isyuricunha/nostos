ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS provider_name TEXT;
ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS approval_state TEXT NOT NULL DEFAULT 'not_required';
ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS error_code TEXT;
ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS input_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS output_bytes INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_tool_calls_state ON tool_calls(state);
CREATE INDEX IF NOT EXISTS idx_tool_calls_pending_approval ON tool_calls(chat_run_id, state, approval_state);
CREATE INDEX IF NOT EXISTS idx_tool_approvals_scope ON tool_approvals(workspace_id, tool_id, agent_id, conversation_id, decision);
