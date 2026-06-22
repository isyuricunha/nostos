# Troubleshooting

## Frontend unavailable

Run `pnpm --dir web build` locally or rebuild the Docker image.

## Database unavailable

Check `DATABASE_DRIVER`, `DATABASE_URL`, `/health/ready`, and `nostos doctor`.

## Encrypted secrets fail

Verify `APP_ENCRYPTION_KEY` is a base64-encoded 32-byte value and has not changed since secrets were created.

## Provider calls fail

Use the Providers screen test action. Confirm the base URL exposes `/v1/models` or manually configure a model.

## Worker is idle

Check task run logs, worker logs, schedule `next_run_at`, and lease recovery events.
