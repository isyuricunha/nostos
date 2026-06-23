# Deployment

Nostos ships as one image with multiple commands:

```sh
server
worker
migrate
doctor
version
```

The normal deployment runs one `app` container and one `worker` container from the same image.

## Required Secrets

Set these before starting production containers:

```env
APP_ENCRYPTION_KEY=base64-encoded-32-byte-key
APP_SESSION_SECRET=at-least-32-characters
```

Generate an encryption key with:

```sh
openssl rand -base64 32
```

Back up `APP_ENCRYPTION_KEY`. Losing it makes encrypted provider and MCP secrets unrecoverable.

## External PostgreSQL

Create `.env` from `.env.example` and set:

```env
DATABASE_DRIVER=postgres
DATABASE_URL=postgresql://user:password@postgres-host:5432/nostos
APP_ENCRYPTION_KEY=...
APP_SESSION_SECRET=...
APP_BASE_URL=https://nostos.example.com
SECURE_COOKIES=true
MODEL_REFRESH_TIMEOUT=60s
```

Start:

```sh
docker compose up -d --build
```

The app container applies migrations during startup. The worker waits for the app health check and verifies that migrations are current before processing jobs.

For Bifrost or other large provider catalogs, keep `MODEL_REFRESH_TIMEOUT` at `60s` or raise it up to `300s`. A failed model refresh does not erase the previous cached catalog.

## Local PostgreSQL Evaluation

Use the bundled PostgreSQL 17 override:

```sh
POSTGRES_PASSWORD=change-me \
APP_ENCRYPTION_KEY="$(openssl rand -base64 32)" \
APP_SESSION_SECRET="$(openssl rand -base64 32)" \
docker compose -f compose.yaml -f compose.local-db.yaml up -d --build
```

This is suitable for local evaluation. For production, prefer an externally managed PostgreSQL service and regular backups.

## SQLite Quick Start

SQLite is supported for single-instance installations:

```sh
APP_ENCRYPTION_KEY="$(openssl rand -base64 32)" \
APP_SESSION_SECRET="$(openssl rand -base64 32)" \
docker compose -f compose.yaml -f compose.sqlite.yaml up -d --build
```

The SQLite database is stored in the Docker-managed `nostos-data` volume at `/data/nostos.db`.

## Health Checks

Readiness:

```sh
curl -fsS http://localhost:7000/health/ready
```

Diagnostics:

```sh
curl -fsS http://localhost:7000/api/v1/diagnostics
```

## Updates

1. Back up PostgreSQL or the SQLite `/data` volume.
2. Preserve `.env`, especially `APP_ENCRYPTION_KEY`.
3. Pull or build the new image.
4. Run:

   ```sh
   docker compose up -d --build
   ```

5. Check `/health/ready` and the Settings diagnostics screen.

## Rollback Considerations

Database migrations are forward-only. Before upgrading, take a database backup that can be restored if you need to roll back the application image.

## Container Security

The Compose files run the application as a non-root distroless container with:

- read-only root filesystem;
- writable `/data` volume;
- temporary `/tmp` tmpfs;
- dropped Linux capabilities;
- `no-new-privileges`;
- no Docker socket mount;
- no privileged mode.
