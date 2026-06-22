# Security

## Threat Model

Nostos assumes a trusted owner, untrusted browsers, potentially unreliable model providers, and high-risk MCP servers. The app must not leak secrets to logs, API responses, diagnostics, or frontend storage.

## Secret Storage

Provider API keys and MCP secrets can be encrypted in the database with `APP_ENCRYPTION_KEY`. Environment references such as `env:BIFROST_API_KEY` avoid database storage.

Losing `APP_ENCRYPTION_KEY` makes encrypted secrets unrecoverable.

## Sessions

Sessions are stored in the database, hashed using `APP_SESSION_SECRET`, scoped to secure SameSite cookies, and revocable from the UI. Disabled users lose access during authentication checks.

## MCP Risks

MCP stdio servers are spawned without shell interpolation, receive bounded inherited environment variables, and should be configured only from trusted sources. Do not mount the Docker socket or grant broad host access.

Tools configured as `ask` create persisted approval records and pause chat runs until the owner approves or denies the call. Tool calls are stored with state, bounded input/output, error text, timing, and truncation status. Provider-facing tool names are mapped back to internal tool IDs to avoid display-name ambiguity.

## Task Risks

Unattended tasks default to preapproved tools only. Task runs have retries, timeouts, logs, leases, and explicit failure states.

Agent tasks use the selected agent runtime settings, but they do not grant interactive tool approval automatically. A task that needs an approval-only tool fails according to its tool policy instead of waiting forever.

## Deployment Recommendations

Use HTTPS, set `SECURE_COOKIES=true`, keep CORS disabled unless required, run behind a trusted reverse proxy, back up secrets separately, and use PostgreSQL for production.

The provided containers run as non-root, drop Linux capabilities, use read-only root filesystems in Compose, avoid privileged mode, and do not mount the Docker socket.
