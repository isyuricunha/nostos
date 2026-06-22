# Architecture

Nostos uses a single Go binary with multiple commands:

- `server`: HTTP API, SSE chat streaming, authentication, static frontend, CRUD screens.
- `worker`: database-backed task queue, schedules, retries, lease recovery, maintenance jobs.
- `migrate`: database migrations.
- `doctor`: diagnostics for container health checks.
- `version`: build metadata.

The frontend is a Svelte TypeScript SPA compiled by Vite and served by the Go server. No Node.js runtime is required in production.

Persistence is PostgreSQL-first with SQLite support behind repository interfaces. Business logic lives in domain services under `internal/*`; handlers translate HTTP into domain inputs and JSON responses.

External model providers use an OpenAI-compatible client. MCP support is isolated in `internal/mcp` so future tool and transport changes do not leak into chat, agents, or tasks.

Email is planned for Version 0.2 and can be added as a new module without rewriting chat, tasks, agents, reply intents, or authentication.
