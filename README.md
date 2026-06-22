# Nostos

Nostos is a lightweight self-hosted AI workspace for chat, agents, explicit memories, MCP tools, scheduled tasks, response feedback, and reply-intent drafts.

Version 0.1 is container-first, Go-backed, Svelte-powered, and designed to run with PostgreSQL in production or SQLite for small single-instance installations.

## Version 0.1 Features

- Local owner setup, login, logout, secure sessions, CSRF checks, session revocation, login throttling, and audit logs.
- PostgreSQL and SQLite persistence through explicit migrations.
- OpenAI-compatible provider configuration, encrypted provider secrets, model refresh, and streaming chat.
- Persistent conversations, messages, chat runs, cancellation, regeneration branches, and visible memory injection.
- Configurable agents and explicit user-created memories.
- MCP server configuration for stdio and HTTP transports, tool discovery, per-tool permissions, and encrypted MCP secrets.
- Database-backed worker queue with manual tasks, interval/cron/one-time schedules, retries, leases, recovery, and visible run logs.
- Assistant-message thumbs up/down feedback and aggregate feedback statistics.
- Reply-intent presets and AI-generated editable reply drafts.
- Dark Svelte SPA served by the Go application.
- Docker, Compose, Makefile, and CI foundations.

Email integration is planned for Version 0.2. Version 0.1 intentionally does not include IMAP, SMTP, Gmail OAuth, calendars, contacts, or fake email pages.

## Quick Start: PostgreSQL

Create `.env` from `.env.example`, set strong secrets, and provide a PostgreSQL URL:

```sh
cp .env.example .env
docker build -t nostos:latest .
docker compose -f compose.yaml up -d
docker compose -f compose.yaml exec app /nostos migrate
```

Required production values:

```env
DATABASE_DRIVER=postgres
DATABASE_URL=postgresql://user:password@host:5432/database
APP_ENCRYPTION_KEY=base64-encoded-32-byte-key
APP_SESSION_SECRET=at-least-32-characters
```

Open `http://localhost:7000` and create the owner account.

## Quick Start: SQLite

SQLite is supported for simple single-instance deployments:

```sh
cp .env.example .env
docker build -t nostos:latest .
docker compose -f compose.yaml -f compose.sqlite.yaml up -d
```

Set:

```env
DATABASE_DRIVER=sqlite
DATABASE_URL=/data/nostos.db
WORKER_CONCURRENCY=1
```

## Local PostgreSQL

For local evaluation with bundled PostgreSQL:

```sh
POSTGRES_PASSWORD=change-me docker compose -f compose.yaml -f compose.local-db.yaml up -d
```

## Bifrost Configuration

Add a provider in the Providers screen:

- Name: `Bifrost`
- Base URL: your Bifrost OpenAI-compatible base URL
- API key: direct secret or `env:BIFROST_API_KEY`
- Default model: a model served by Bifrost

Nostos calls `/v1/models` and `/v1/chat/completions` and supports streaming Server-Sent Events.

## Worker

Run one web container and one worker container from the same image:

```sh
docker compose -f compose.yaml up -d app worker
```

The worker handles scheduled tasks, queued manual runs, lease recovery, retry execution, and maintenance task placeholders.

## MCP

MCP servers are configured in the MCP screen. Version 0.1 supports stdio server discovery and HTTP JSON-RPC discovery against the current Streamable HTTP style endpoint. Secrets in headers and environment variables are encrypted at rest.

## Tasks

Tasks can be manual, one-time, cron, or interval based. Schedules are timezone-aware and use IANA timezone names. Task runs are stored with events, retry state, timeout, lease ownership, result, and error fields.

## Development

```sh
pnpm --dir web install
make dev
make test
make lint
make build
```

SQLite quick start:

```sh
APP_ENV=development DATABASE_DRIVER=sqlite DATABASE_URL=data/dev.db make migrate
APP_ENV=development DATABASE_DRIVER=sqlite DATABASE_URL=data/dev.db make dev
```

## Upgrade Procedure

1. Back up PostgreSQL or `/data/nostos.db`.
2. Pull/build the new image.
3. Run `nostos migrate`.
4. Restart `app` and `worker`.
5. Check `/health/ready` and `/api/v1/diagnostics`.

## Backup Recommendations

- PostgreSQL: use native `pg_dump` or managed database snapshots.
- SQLite: stop the app and worker or use SQLite online backup tooling before copying `/data/nostos.db`.
- Back up `.env` secrets separately. Losing `APP_ENCRYPTION_KEY` makes encrypted provider and MCP secrets unrecoverable.

## Security Warnings

Do not expose Nostos without HTTPS and secure cookies in production. Do not mount the Docker socket. Do not configure untrusted MCP servers with broad permissions. Provider and MCP secrets are never returned by the API after storage.

## Screenshots

Screenshots are not included yet.
