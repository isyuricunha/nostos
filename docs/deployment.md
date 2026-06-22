# Deployment

Build:

```sh
docker build -t nostos:latest .
```

External PostgreSQL:

```sh
docker compose -f compose.yaml up -d
```

Local PostgreSQL:

```sh
POSTGRES_PASSWORD=change-me docker compose -f compose.yaml -f compose.local-db.yaml up -d
```

SQLite:

```sh
docker compose -f compose.yaml -f compose.sqlite.yaml up -d
```

Use one `app` container and one `worker` container. Both use the same image and different commands.
