ALTER TABLE providers ADD COLUMN IF NOT EXISTS model_refresh_state TEXT NOT NULL DEFAULT 'idle';
ALTER TABLE providers ADD COLUMN IF NOT EXISTS model_refresh_started_at TIMESTAMPTZ;
ALTER TABLE providers ADD COLUMN IF NOT EXISTS model_refresh_completed_at TIMESTAMPTZ;
ALTER TABLE providers ADD COLUMN IF NOT EXISTS model_refresh_duration_ms INTEGER;
ALTER TABLE providers ADD COLUMN IF NOT EXISTS model_refresh_error_category TEXT;
ALTER TABLE providers ADD COLUMN IF NOT EXISTS model_refresh_error_message TEXT;

ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS workspace_id TEXT;
UPDATE provider_models
SET workspace_id = providers.workspace_id
FROM providers
WHERE provider_models.provider_id = providers.id
  AND provider_models.workspace_id IS NULL;
ALTER TABLE provider_models ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE provider_models ADD CONSTRAINT provider_models_workspace_id_fkey FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE;

ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS manually_added BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS available BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS first_seen_at TIMESTAMPTZ;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS last_successful_probe_at TIMESTAMPTZ;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS last_failed_probe_at TIMESTAMPTZ;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS last_error_category TEXT;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS last_safe_error_message TEXT;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS capabilities JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS capability_source TEXT NOT NULL DEFAULT 'unknown';
ALTER TABLE provider_models ADD COLUMN IF NOT EXISTS search_text TEXT NOT NULL DEFAULT '';

UPDATE provider_models
SET first_seen_at = COALESCE(first_seen_at, created_at),
    last_seen_at = COALESCE(last_seen_at, refreshed_at, updated_at),
    enabled = COALESCE(enabled, active),
    manually_added = CASE WHEN source = 'manual' THEN true ELSE manually_added END,
    available = COALESCE(available, active),
    search_text = lower(COALESCE(display_name, '') || ' ' || model_id)
WHERE first_seen_at IS NULL
   OR last_seen_at IS NULL
   OR search_text = '';

INSERT INTO provider_models (id, workspace_id, provider_id, model_id, display_name, source, active, metadata, refreshed_at, enabled, manually_added, available, first_seen_at, last_seen_at, capabilities, capability_source, search_text, created_at, updated_at)
SELECT 'pm_' || substr(md5(p.id || ':default:' || p.default_model), 1, 24),
       p.workspace_id,
       p.id,
       p.default_model,
       p.default_model,
       'manual',
       true,
       '{}'::jsonb,
       NOW(),
       true,
       true,
       true,
       NOW(),
       NOW(),
       '["chat"]'::jsonb,
       'manual',
       lower(p.default_model || ' ' || p.name),
       NOW(),
       NOW()
FROM providers p
WHERE COALESCE(trim(p.default_model), '') <> ''
ON CONFLICT (provider_id, model_id) DO NOTHING;

INSERT INTO provider_models (id, workspace_id, provider_id, model_id, display_name, source, active, metadata, refreshed_at, enabled, manually_added, available, first_seen_at, last_seen_at, capabilities, capability_source, search_text, created_at, updated_at)
SELECT 'pm_' || substr(md5(p.id || ':fallback:' || p.fallback_model), 1, 24),
       p.workspace_id,
       p.id,
       p.fallback_model,
       p.fallback_model,
       'manual',
       true,
       '{}'::jsonb,
       NOW(),
       true,
       true,
       true,
       NOW(),
       NOW(),
       '["chat"]'::jsonb,
       'manual',
       lower(p.fallback_model || ' ' || p.name),
       NOW(),
       NOW()
FROM providers p
WHERE COALESCE(trim(p.fallback_model), '') <> ''
ON CONFLICT (provider_id, model_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS model_role_bindings (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('chat', 'utility', 'vision')),
    position INTEGER NOT NULL CHECK (position >= 0),
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    model_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, role, position)
);

INSERT INTO model_role_bindings (id, workspace_id, role, position, provider_id, model_id, created_at, updated_at)
SELECT 'mr_' || substr(md5(p.workspace_id || ':chat:0'), 1, 24),
       p.workspace_id,
       'chat',
       0,
       p.id,
       p.default_model,
       NOW(),
       NOW()
FROM providers p
WHERE COALESCE(trim(p.default_model), '') <> ''
  AND NOT EXISTS (
      SELECT 1 FROM model_role_bindings existing
      WHERE existing.workspace_id = p.workspace_id AND existing.role = 'chat'
  );

INSERT INTO model_role_bindings (id, workspace_id, role, position, provider_id, model_id, created_at, updated_at)
SELECT 'mr_' || substr(md5(p.workspace_id || ':utility:0'), 1, 24),
       p.workspace_id,
       'utility',
       0,
       p.id,
       p.default_model,
       NOW(),
       NOW()
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
