# Providers

Providers are OpenAI-compatible endpoints with:

- Base URL
- API key or `env:NAME` reference
- Optional organization/project headers
- Custom headers
- Default and fallback models
- Health status

Nostos uses:

- `GET /v1/models`
- `POST /v1/chat/completions`

Streaming chat completions are consumed through Server-Sent Events. Tool-call deltas are parsed by the provider client for MCP/tool integration paths.
