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

## Task Risks

Unattended tasks default to preapproved tools only. Task runs have retries, timeouts, logs, leases, and explicit failure states.

## Deployment Recommendations

Use HTTPS, set `SECURE_COOKIES=true`, keep CORS disabled unless required, run behind a trusted reverse proxy, back up secrets separately, and use PostgreSQL for production.
