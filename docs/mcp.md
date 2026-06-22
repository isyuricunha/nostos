# MCP

Nostos supports:

- Local stdio MCP server discovery.
- HTTP JSON-RPC discovery compatible with current Streamable HTTP deployments.

Configured fields include command, arguments, working directory, environment variables, HTTP URL, HTTP headers, timeouts, enabled state, health state, and last error.

Sensitive environment values and headers are encrypted at rest when `APP_ENCRYPTION_KEY` is set.

Version 0.1 does not include a generic unrestricted shell tool and never mounts the Docker socket.
