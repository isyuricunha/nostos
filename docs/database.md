# Database

Migrations live in:

- `migrations/postgres`
- `migrations/sqlite`

PostgreSQL is the recommended production database. SQLite is supported for small single-instance installations and enables foreign keys on connection.

Important table groups:

- Identity: `users`, `sessions`, `audit_logs`, `workspaces`
- Providers: `providers`, `provider_models`
- Agents: `agents`, `agent_mcp_servers`, `agent_tool_permissions`
- Chat: `conversations`, `messages`, `message_branches`, `chat_runs`, `chat_run_memories`
- Memories: `memories`
- MCP/tools: `mcp_servers`, `mcp_tools`, `tool_calls`, `tool_approvals`
- Tasks: `tasks`, `task_schedules`, `task_runs`, `task_run_events`, `task_leases`
- Feedback/replies: `message_feedback`, `reply_presets`, `reply_drafts`
- Settings: `settings`

Identifiers are application-generated text IDs. Timestamps are stored in UTC. PostgreSQL uses JSONB where useful; SQLite stores JSON as text.
