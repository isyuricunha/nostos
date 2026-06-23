ALTER TABLE providers ADD COLUMN model_refresh_state TEXT NOT NULL DEFAULT 'idle';
ALTER TABLE providers ADD COLUMN model_refresh_started_at TEXT;
ALTER TABLE providers ADD COLUMN model_refresh_completed_at TEXT;
ALTER TABLE providers ADD COLUMN model_refresh_duration_ms INTEGER;
ALTER TABLE providers ADD COLUMN model_refresh_error_category TEXT;
ALTER TABLE providers ADD COLUMN model_refresh_error_message TEXT;

ALTER TABLE provider_models ADD COLUMN workspace_id TEXT;
UPDATE provider_models
SET workspace_id = (SELECT providers.workspace_id FROM providers WHERE providers.id = provider_models.provider_id)
WHERE workspace_id IS NULL;

ALTER TABLE provider_models ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE provider_models ADD COLUMN manually_added INTEGER NOT NULL DEFAULT 0;
ALTER TABLE provider_models ADD COLUMN available INTEGER NOT NULL DEFAULT 1;
ALTER TABLE provider_models ADD COLUMN first_seen_at TEXT;
ALTER TABLE provider_models ADD COLUMN last_seen_at TEXT;
ALTER TABLE provider_models ADD COLUMN last_successful_probe_at TEXT;
ALTER TABLE provider_models ADD COLUMN last_failed_probe_at TEXT;
ALTER TABLE provider_models ADD COLUMN last_error_category TEXT;
ALTER TABLE provider_models ADD COLUMN last_safe_error_message TEXT;
ALTER TABLE provider_models ADD COLUMN capabilities TEXT NOT NULL DEFAULT '[]';
ALTER TABLE provider_models ADD COLUMN capability_source TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE provider_models ADD COLUMN search_text TEXT NOT NULL DEFAULT '';

UPDATE provider_models
SET first_seen_at = COALESCE(first_seen_at, created_at),
    last_seen_at = COALESCE(last_seen_at, refreshed_at, updated_at),
    enabled = active,
    manually_added = CASE WHEN source = 'manual' THEN 1 ELSE manually_added END,
    available = active,
    search_text = lower(COALESCE(display_name, '') || ' ' || model_id)
WHERE first_seen_at IS NULL
   OR last_seen_at IS NULL
   OR search_text = '';

INSERT OR IGNORE INTO provider_models (id, workspace_id, provider_id, model_id, display_name, source, active, metadata, refreshed_at, enabled, manually_added, available, first_seen_at, last_seen_at, capabilities, capability_source, search_text, created_at, updated_at)
SELECT lower(hex(randomblob(16))),
       p.workspace_id,
       p.id,
       p.default_model,
       p.default_model,
       'manual',
       1,
       '{}',
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       1,
       1,
       1,
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       '["chat"]',
       'manual',
       lower(p.default_model || ' ' || p.name),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM providers p
WHERE COALESCE(trim(p.default_model), '') <> '';

INSERT OR IGNORE INTO provider_models (id, workspace_id, provider_id, model_id, display_name, source, active, metadata, refreshed_at, enabled, manually_added, available, first_seen_at, last_seen_at, capabilities, capability_source, search_text, created_at, updated_at)
SELECT lower(hex(randomblob(16))),
       p.workspace_id,
       p.id,
       p.fallback_model,
       p.fallback_model,
       'manual',
       1,
       '{}',
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       1,
       1,
       1,
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       '["chat"]',
       'manual',
       lower(p.fallback_model || ' ' || p.name),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM providers p
WHERE COALESCE(trim(p.fallback_model), '') <> '';

CREATE TABLE IF NOT EXISTS model_role_bindings (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('chat', 'utility', 'vision')),
    position INTEGER NOT NULL CHECK (position >= 0),
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    model_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (workspace_id, role, position)
);

INSERT INTO model_role_bindings (id, workspace_id, role, position, provider_id, model_id, created_at, updated_at)
SELECT lower(hex(randomblob(16))),
       p.workspace_id,
       'chat',
       0,
       p.id,
       p.default_model,
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM providers p
WHERE COALESCE(trim(p.default_model), '') <> ''
  AND NOT EXISTS (
      SELECT 1 FROM model_role_bindings existing
      WHERE existing.workspace_id = p.workspace_id AND existing.role = 'chat'
  );

INSERT INTO model_role_bindings (id, workspace_id, role, position, provider_id, model_id, created_at, updated_at)
SELECT lower(hex(randomblob(16))),
       p.workspace_id,
       'utility',
       0,
       p.id,
       p.default_model,
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
       strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM providers p
WHERE COALESCE(trim(p.default_model), '') <> ''
  AND NOT EXISTS (
      SELECT 1 FROM model_role_bindings existing
      WHERE existing.workspace_id = p.workspace_id AND existing.role = 'utility'
  );

CREATE INDEX IF NOT EXISTS idx_provider_models_workspace_provider ON provider_models(workspace_id, provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_models_workspace_available ON provider_models(workspace_id, available, enabled);
CREATE INDEX IF NOT EXISTS idx_provider_models_model_id ON provider_models(model_id);
CREATE INDEX IF NOT EXISTS idx_provider_models_updated ON provider_models(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_provider_models_search ON provider_models(workspace_id, search_text);
CREATE INDEX IF NOT EXISTS idx_model_role_bindings_workspace_role ON model_role_bindings(workspace_id, role, position);
