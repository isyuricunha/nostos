ALTER TABLE conversations ADD COLUMN summary_status TEXT NOT NULL DEFAULT 'idle';
ALTER TABLE conversations ADD COLUMN summary_error TEXT;
ALTER TABLE conversations ADD COLUMN summary_source_start_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE conversations ADD COLUMN summary_source_end_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE conversations ADD COLUMN summary_provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL;
ALTER TABLE conversations ADD COLUMN summary_model TEXT;
ALTER TABLE conversations ADD COLUMN summary_generated_at TEXT;
ALTER TABLE conversations ADD COLUMN summary_estimated_input_tokens INTEGER;
ALTER TABLE conversations ADD COLUMN summary_version INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_conversations_summary_status ON conversations(summary_status, updated_at);
