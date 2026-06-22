# Changelog

## 0.1.0 - Unreleased

### Added

- Canonical Go module path `github.com/isyuricunha/nostos`.
- Branch-aware chat context construction with persisted multi-turn history, summaries, memories, tool calls, and tool results.
- Automatic conversation summary queueing, summary metadata, worker generation, manual regeneration, and clearing APIs.
- Typed internal maintenance handlers for expired sessions, task leases, task events, summaries, provider health, MCP health, abandoned chat runs, generated titles, duplicate scheduling events, and temporary files.
- Persisted MCP tool-call runtime state, pending approvals, approve/deny APIs, and chat-run resume.
- Bounded multi-iteration tool execution loop for chat and agent tasks.
- Agent task execution through agent runtime settings, selected memories, provider/model resolution, and unattended tool policies.
- Real worker concurrency with lease renewal and atomic scheduled-occurrence claiming.
- Frontend controls for conversation summaries, tool approvals, provider editing, agent editing, task editing, MCP server editing, and memory editing.
- Docker and Compose smoke validation for SQLite and bundled PostgreSQL.

### Fixed

- PostgreSQL migration checks no longer issue SQLite-style placeholder queries before retrying.
- `doctor` reports actual migration status instead of a hardcoded value.
- README worker documentation no longer describes maintenance tasks as placeholders.

### Not Yet Released

- GitHub Actions passed for the pushed remediation commits.
- The release tag has not been created.
