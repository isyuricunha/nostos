CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    owner_user_id TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE users (
    id TEXT PRIMARY KEY,
    workspace_id TEXT REFERENCES workspaces(id) ON DELETE RESTRICT,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'member')),
    disabled_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    csrf_token_hash TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE audit_logs (
    id TEXT PRIMARY KEY,
    workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_audit_logs_workspace_created ON audit_logs(workspace_id, created_at DESC);
CREATE INDEX idx_audit_logs_event_type ON audit_logs(event_type);

CREATE TABLE providers (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    encrypted_api_key TEXT,
    api_key_env_ref TEXT,
    organization_header TEXT,
    project_header TEXT,
    custom_headers TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER NOT NULL DEFAULT 1,
    request_timeout_ms INTEGER NOT NULL DEFAULT 60000,
    default_model TEXT,
    fallback_model TEXT,
    health_status TEXT NOT NULL DEFAULT 'unknown',
    last_health_check_at TEXT,
    last_error TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (workspace_id, name)
);
CREATE INDEX idx_providers_workspace ON providers(workspace_id);

CREATE TABLE provider_models (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    display_name TEXT,
    source TEXT NOT NULL CHECK (source IN ('api', 'manual')),
    active INTEGER NOT NULL DEFAULT 1,
    metadata TEXT NOT NULL DEFAULT '{}',
    refreshed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (provider_id, model_id)
);

CREATE TABLE mcp_servers (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    transport_type TEXT NOT NULL CHECK (transport_type IN ('stdio', 'http')),
    command TEXT,
    arguments TEXT NOT NULL DEFAULT '[]',
    working_directory TEXT,
    encrypted_environment TEXT NOT NULL DEFAULT '{}',
    http_url TEXT,
    encrypted_http_headers TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER NOT NULL DEFAULT 0,
    startup_timeout_ms INTEGER NOT NULL DEFAULT 10000,
    request_timeout_ms INTEGER NOT NULL DEFAULT 30000,
    health_status TEXT NOT NULL DEFAULT 'unknown',
    last_error TEXT,
    last_connected_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (workspace_id, name)
);

CREATE TABLE mcp_tools (
    id TEXT PRIMARY KEY,
    server_id TEXT NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    input_schema TEXT NOT NULL DEFAULT '{}',
    permission_mode TEXT NOT NULL DEFAULT 'ask' CHECK (permission_mode IN ('deny', 'ask', 'allow')),
    discovered_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (server_id, name)
);

CREATE TABLE agents (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    avatar TEXT NOT NULL DEFAULT 'sparkles',
    system_prompt TEXT NOT NULL,
    default_provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL,
    default_model TEXT,
    fallback_model TEXT,
    temperature REAL NOT NULL DEFAULT 0.7,
    max_tool_iterations INTEGER NOT NULL DEFAULT 8,
    memory_access_mode TEXT NOT NULL DEFAULT 'pinned_only' CHECK (memory_access_mode IN ('none', 'pinned_only', 'relevant', 'all')),
    tool_permission_default TEXT NOT NULL DEFAULT 'ask' CHECK (tool_permission_default IN ('deny', 'ask', 'allow')),
    active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (workspace_id, name)
);

CREATE TABLE agent_mcp_servers (
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    server_id TEXT NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (agent_id, server_id)
);

CREATE TABLE agent_tool_permissions (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tool_id TEXT NOT NULL REFERENCES mcp_tools(id) ON DELETE CASCADE,
    permission_mode TEXT NOT NULL CHECK (permission_mode IN ('deny', 'ask', 'allow')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (agent_id, tool_id)
);

CREATE TABLE conversations (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id TEXT REFERENCES agents(id) ON DELETE SET NULL,
    agent_snapshot TEXT NOT NULL DEFAULT '{}',
    provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL,
    model TEXT,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    summary_updated_at TEXT,
    archived_at TEXT,
    deleted_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_conversations_workspace_updated ON conversations(workspace_id, updated_at DESC);
CREATE INDEX idx_conversations_title ON conversations(title);

CREATE TABLE message_branches (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    parent_message_id TEXT,
    source_message_id TEXT,
    name TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    branch_id TEXT REFERENCES message_branches(id) ON DELETE SET NULL,
    parent_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    role TEXT NOT NULL CHECK (role IN ('system', 'user', 'assistant', 'tool')),
    content TEXT NOT NULL DEFAULT '',
    markdown TEXT NOT NULL DEFAULT '',
    tool_call_id TEXT,
    provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL,
    model TEXT,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_messages_conversation_created ON messages(conversation_id, created_at);

CREATE TABLE chat_runs (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    assistant_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    branch_id TEXT REFERENCES message_branches(id) ON DELETE SET NULL,
    provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL,
    model TEXT,
    state TEXT NOT NULL CHECK (state IN ('pending', 'streaming', 'waiting_for_tool_approval', 'completed', 'failed', 'cancelled')),
    error_code TEXT,
    error_message TEXT,
    cancellation_requested_at TEXT,
    started_at TEXT,
    completed_at TEXT,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_chat_runs_conversation_created ON chat_runs(conversation_id, created_at DESC);
CREATE INDEX idx_chat_runs_state ON chat_runs(state);

CREATE TABLE memories (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    owner_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id TEXT REFERENCES agents(id) ON DELETE CASCADE,
    conversation_id TEXT REFERENCES conversations(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '[]',
    scope TEXT NOT NULL CHECK (scope IN ('global', 'agent', 'conversation', 'workspace')),
    importance INTEGER NOT NULL DEFAULT 50,
    pinned INTEGER NOT NULL DEFAULT 0,
    active INTEGER NOT NULL DEFAULT 1,
    source TEXT NOT NULL CHECK (source IN ('manual', 'message', 'task', 'import')),
    source_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    last_used_at TEXT,
    use_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_memories_workspace_scope ON memories(workspace_id, scope);
CREATE INDEX idx_memories_title ON memories(title);

CREATE TABLE chat_run_memories (
    chat_run_id TEXT NOT NULL REFERENCES chat_runs(id) ON DELETE CASCADE,
    memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    rank_score REAL NOT NULL,
    removed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    PRIMARY KEY (chat_run_id, memory_id)
);

CREATE TABLE tool_calls (
    id TEXT PRIMARY KEY,
    chat_run_id TEXT REFERENCES chat_runs(id) ON DELETE CASCADE,
    message_id TEXT REFERENCES messages(id) ON DELETE SET NULL,
    tool_id TEXT REFERENCES mcp_tools(id) ON DELETE SET NULL,
    provider_tool_call_id TEXT,
    name TEXT NOT NULL,
    input TEXT NOT NULL DEFAULT '{}',
    output TEXT,
    output_truncated INTEGER NOT NULL DEFAULT 0,
    state TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    started_at TEXT,
    completed_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_tool_calls_run ON tool_calls(chat_run_id);

CREATE TABLE tool_approvals (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    tool_call_id TEXT REFERENCES tool_calls(id) ON DELETE CASCADE,
    tool_id TEXT REFERENCES mcp_tools(id) ON DELETE CASCADE,
    agent_id TEXT REFERENCES agents(id) ON DELETE CASCADE,
    conversation_id TEXT REFERENCES conversations(id) ON DELETE CASCADE,
    actor_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    decision TEXT NOT NULL CHECK (decision IN ('approve_once', 'approve_conversation', 'allow_agent', 'deny', 'deny_disable_tool')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    task_type TEXT NOT NULL CHECK (task_type IN ('agent', 'system')),
    state TEXT NOT NULL CHECK (state IN ('draft', 'enabled', 'disabled')),
    system_managed INTEGER NOT NULL DEFAULT 0,
    agent_id TEXT REFERENCES agents(id) ON DELETE SET NULL,
    provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL,
    model TEXT,
    prompt TEXT NOT NULL DEFAULT '',
    tool_policy TEXT NOT NULL DEFAULT 'use_preapproved_tools_only' CHECK (tool_policy IN ('fail_if_approval_required', 'use_preapproved_tools_only')),
    max_retries INTEGER NOT NULL DEFAULT 3,
    timeout_ms INTEGER NOT NULL DEFAULT 600000,
    concurrency_policy TEXT NOT NULL DEFAULT 'skip' CHECK (concurrency_policy IN ('allow', 'skip', 'replace')),
    result TEXT,
    last_error TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (workspace_id, name)
);

CREATE TABLE task_schedules (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    mode TEXT NOT NULL CHECK (mode IN ('manual', 'one_time', 'cron', 'interval')),
    cron_expression TEXT,
    interval_seconds INTEGER,
    run_at TEXT,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    enabled INTEGER NOT NULL DEFAULT 1,
    next_run_at TEXT,
    last_enqueued_occurrence TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_task_schedules_next_run ON task_schedules(enabled, next_run_at);

CREATE TABLE task_runs (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    schedule_id TEXT REFERENCES task_schedules(id) ON DELETE SET NULL,
    idempotency_key TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL CHECK (state IN ('queued', 'claimed', 'running', 'waiting', 'succeeded', 'failed', 'cancelled', 'timed_out')),
    attempt INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    timeout_ms INTEGER NOT NULL DEFAULT 600000,
    lease_owner TEXT,
    lease_expires_at TEXT,
    queued_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    started_at TEXT,
    completed_at TEXT,
    result TEXT,
    error_message TEXT,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_task_runs_state_lease ON task_runs(state, lease_expires_at);
CREATE INDEX idx_task_runs_task_created ON task_runs(task_id, created_at DESC);

CREATE TABLE task_run_events (
    id TEXT PRIMARY KEY,
    task_run_id TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
    level TEXT NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error')),
    message TEXT NOT NULL,
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_task_run_events_run_created ON task_run_events(task_run_id, created_at);

CREATE TABLE task_leases (
    id TEXT PRIMARY KEY,
    task_run_id TEXT NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
    worker_id TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    released_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_task_leases_expires ON task_leases(expires_at);

CREATE TABLE message_feedback (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating TEXT NOT NULL CHECK (rating IN ('positive', 'negative')),
    reason TEXT,
    comment TEXT,
    provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL,
    model TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (message_id, user_id)
);

CREATE TABLE reply_presets (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    prompt_instruction TEXT NOT NULL,
    icon TEXT NOT NULL DEFAULT 'message-circle',
    sort_order INTEGER NOT NULL DEFAULT 0,
    active INTEGER NOT NULL DEFAULT 1,
    system_default INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (workspace_id, name)
);

CREATE TABLE reply_drafts (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    source_message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    preset_id TEXT REFERENCES reply_presets(id) ON DELETE SET NULL,
    preset_name TEXT NOT NULL,
    custom_instruction TEXT NOT NULL DEFAULT '',
    generated_draft TEXT NOT NULL,
    provider_id TEXT REFERENCES providers(id) ON DELETE SET NULL,
    model TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE settings (
    id TEXT PRIMARY KEY,
    workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '{}',
    updated_by_user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (workspace_id, key)
);
