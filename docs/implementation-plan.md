# Nostos Version 0.1 Implementation Plan

## Repository baseline

The repository starts from an initial commit with only `.gitattributes` and `LICENSE`. There are no existing application files, package manifests, migrations, local `AGENTS.md`, or implementation conventions to preserve.

## Verified external constraints

- MCP: implement stdio and current Streamable HTTP transports. Do not rely on deprecated HTTP+SSE as the primary transport.
- OpenAI-compatible providers: target `/v1/models` and `/v1/chat/completions`, including streaming chunks, usage chunks, and streamed `tool_calls`.
- Frontend: use Svelte with TypeScript and Vite. Production serves compiled static assets from the Go app.
- PostgreSQL: use pgx through `database/sql` for portable repository code.
- SQLite: use `modernc.org/sqlite` to keep the runtime cgo-free. Enable foreign keys through SQLite pragmas.
- Docker: use multi-stage builds and copy only runtime artifacts into the final image.

## Milestones

### Milestone 1: Foundation

Create the application skeleton, Go module, configuration loader, structured logging, database abstraction, migrations for both PostgreSQL and SQLite, command dispatcher, health endpoints, embedded frontend serving, Makefile, and initial docs.

Validation:

- `go test ./...`
- `pnpm --dir web check`
- `pnpm --dir web test`
- `pnpm --dir web build`
- `go build ./cmd/app`

Commit: `chore: establish application foundation`

### Milestone 2: Authentication and Security

Implement first-run setup, optional bootstrap owner creation, password hashing, persistent sessions, secure cookies, logout, session revocation, login throttling, same-origin protection, security headers, disabled-user session invalidation, and audit logs.

Validation:

- Auth unit tests.
- SQLite migration and repository tests.
- Manual setup/login/logout smoke test.

Commit: `feat(auth): add owner setup and sessions`

### Milestone 3: Providers and Chat

Implement encrypted provider secrets, environment secret references, model discovery, provider health checks, OpenAI-compatible streaming client, conversations, messages, chat runs, SSE stream endpoint, cancellation, regeneration, edit-and-branch flow, token usage capture, interrupted run recovery, summary records, and provider error persistence.

Validation:

- Mock provider tests for models, streaming content, usage, errors, and tool-call deltas.
- Chat repository tests for persistence and branching.

Commit: `feat(chat): add providers and streaming conversations`

### Milestone 4: Agents and Explicit Memories

Implement agent CRUD, default general assistant, agent snapshots for conversations, memory CRUD, scopes, tags, pinned state, relevance ranking, transparent memory injection, chat-run memory audit records, summary inspect/regenerate/clear controls, and UI surfaces.

Validation:

- Agent validation tests.
- Memory ranking tests.
- Chat prompt assembly tests.

Commit: `feat(memory): add agents and explicit memory injection`

### Milestone 5: MCP and Tool Loop

Implement MCP server configuration, stdio process lifecycle, Streamable HTTP JSON-RPC client, tool discovery, tool storage, per-tool permissions, pending approvals, audit events, bounded tool-calling loop, persisted tool calls, tool-result truncation, secret redaction, and UI tool cards.

Validation:

- Mock MCP stdio and HTTP tests.
- Tool permission evaluation tests.
- Tool-loop iteration and timeout tests.

Commit: `feat(mcp): add tool discovery and approvals`

### Milestone 6: Worker, Scheduler, and Tasks

Implement the standalone worker command, database-backed queue, leases, retries, cancellation, task schedules, manual, one-time, cron, and interval scheduling, scheduler leadership/idempotency, run history, event logs, internal maintenance tasks, and system task visibility.

Validation:

- SQLite and PostgreSQL task claiming tests.
- Lease recovery tests.
- Scheduler idempotency tests.
- Manual and scheduled task smoke tests.

Commit: `feat(tasks): add worker queue and scheduling`

### Milestone 7: Feedback and Reply Intents

Implement assistant thumbs-up/down feedback, negative-feedback reasons, regeneration with feedback context, aggregate statistics, default reply presets, custom preset CRUD/reorder/reset, reply draft generation, regeneration, editing, and insertion into chat.

Validation:

- Feedback storage tests.
- Reply preset rendering tests.
- Reply draft tests with mock provider.

Commit: `feat(replies): add feedback and reply intents`

### Milestone 8: UI Completion

Complete the Svelte SPA screens: setup, login, chat, agents, memories, tasks, MCP, providers, settings, diagnostics, session management, and admin statistics. Finalize responsive dark design, accessibility labels, focus states, empty states, destructive confirmations, loading states, and centralized user-facing strings.

Validation:

- Frontend type checking.
- Frontend unit tests.
- Playwright end-to-end flows against mock services.

Commit: `feat(ui): complete workspace interface`

### Milestone 9: Deployment, CI, and Documentation

Add production Dockerfile, `compose.yaml`, `compose.local-db.yaml`, `.dockerignore`, `.env.example`, GitHub Actions workflow, final documentation, security review, troubleshooting, and operational guides.

Validation:

- Production Docker build.
- PostgreSQL Compose readiness.
- SQLite readiness.
- Full test suite.

Commit: `chore: add deployment and ci validation`

## Architecture decisions

- Use one image with command modes: `server`, `worker`, `migrate`, `doctor`, and `version`.
- Keep web/API and worker runtime paths separate, sharing only repositories, services, config, and domain packages.
- Use `database/sql` with dialect helpers so PostgreSQL and SQLite remain feature-complete without duplicating business logic.
- Use ULID strings generated by the application for all primary identifiers.
- Store UTC timestamps as timestamptz in PostgreSQL and RFC3339 text in SQLite.
- Use JSONB in PostgreSQL and JSON text in SQLite behind repository methods.
- Use the database as the queue. PostgreSQL uses `FOR UPDATE SKIP LOCKED`; SQLite uses atomic update claims inside immediate transactions and defaults to one worker.
- Use authenticated encryption for provider secrets and MCP sensitive fields.
- Keep MCP execution bounded and permissioned. No shell tool, Docker socket, or privileged container access is included.
- Keep email out of Version 0.1 while leaving reply intents, tasks, agents, and memory modules independent enough for an email module in Version 0.2.

## Risks and controls

- Feature breadth is high. Controls: milestone commits, tests per milestone, repository interfaces, and no placeholder-only API paths.
- SQLite/PostgreSQL differences can drift. Controls: parallel migration directories, integration tests for both dialects, and repository-level abstractions.
- Provider and MCP protocols can vary by implementation. Controls: OpenAI-compatible parser with tolerant streaming handling, MCP protocol-version headers, mock providers, and mock MCP servers.
- Tool execution is security-sensitive. Controls: explicit permissions, approval persistence, bounded results, redaction, no shell interpolation, and audit logs.
- Scheduler duplication is a reliability risk. Controls: occurrence idempotency keys, leases, and scheduler heartbeat/leadership records.

## Definition-of-done mapping

The final validation will run formatting, linting, backend tests, frontend tests, integration tests for both databases, end-to-end tests, frontend build, Go build, Docker build, PostgreSQL deployment smoke, SQLite deployment smoke, log review, and security self-review. Any command that cannot run will be reported with the exact reason and replacement verification.
