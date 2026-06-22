# Frontend Redesign

Date: 2026-06-22

## Goals

The redesign moves Nostos away from a raw administration-panel feel and toward a premium dark AI workspace. The interface should feel calm, dense, intentional, and suitable for a self-hosted developer command center.

The work preserved the existing Svelte SPA and API integration. It did not add a production Node.js runtime and did not copy another product's code, assets, or layout source.

## Visual System

The UI now uses a near-black foundation with very dark elevated panels, soft separators, rounded surfaces, and restrained amber accents. The accent color is used for active navigation, primary calls to action, focus states, status highlights, and selected cards.

Core tokens live in:

- `web/src/styles/tokens.css`
- `web/src/styles/theme.css`
- `web/src/styles/motion.css`
- `web/src/styles/utilities.css`

Token groups include:

- background and panel colors;
- muted, soft, and strong text colors;
- amber accent colors;
- success, warning, danger, and info status colors;
- radius sizes;
- spacing scale;
- type scale;
- shadow styles;
- transition durations and easing.

## Component Structure

`web/src/App.svelte` is now a thin entry component that renders `WorkspaceApp`.

The previous monolithic app markup was split into focused views:

- `web/src/views/AuthView.svelte`
- `web/src/views/WorkspaceApp.svelte`
- `web/src/views/ChatView.svelte`
- `web/src/views/AgentsView.svelte`
- `web/src/views/MemoriesView.svelte`
- `web/src/views/TasksView.svelte`
- `web/src/views/MCPView.svelte`
- `web/src/views/ProvidersView.svelte`
- `web/src/views/SettingsView.svelte`

The application shell now lives in:

- `web/src/app/shell/AppShell.svelte`

Reusable primitives were added under:

- `web/src/components/common/`
- `web/src/components/forms/`

Shared primitives include button, icon button, badge, card, section container, sidebar item, tabs, dropdown, modal, empty state, status pill, notice, skeleton, input, textarea, select, checkbox, and toggle components.

## Screen Changes

Chat:

- Added a premium two-column workbench with a conversation rail and message workspace.
- Added conversation search styling, selected conversation emphasis, and cleaner timestamps.
- Added a structured provider/agent/model toolbar.
- Added context status pills for active provider, active agent, streaming, and tool usage.
- Improved message cards, assistant/user contrast, markdown/code styling, feedback controls, reply draft panel, tool cards, pending approvals, summary panel, memory panel, and composer.

Agents:

- Split the long form into identity, provider/model, memory/tools, and runtime sections.
- Added agent status badges and cleaner assistant cards.

Memories:

- Reworked memories into knowledge cards with title, content, tags, scope, importance, pinned state, active state, source, and usage metadata.

Tasks:

- Reworked tasks into an operations panel.
- System-managed tasks have clear status badges and visual treatment.
- Schedule, retry, timeout, tool policy, concurrency policy, run history, and event log display remain available.

MCP:

- Split server configuration into identity, HTTP transport, stdio transport, and timeout sections.
- Server health, transport, secret-key metadata, errors, discovery, and tool permission controls are clearer.

Providers:

- Split provider setup into connection, secrets, and model defaults.
- Added clearer health indicators, write-only secret treatment, and infrastructure-style provider cards.

Settings:

- Grouped profile, sessions, diagnostics, feedback statistics, and reply presets into deliberate cards.
- Diagnostics now read as a runtime status dashboard rather than a data dump.

## Motion Approach

Motion is restrained and utility-focused:

- panels animate in with a short upward fade;
- skeleton placeholders use a subtle sweep;
- streaming state uses a small pulsing indicator;
- hover states use short border/background/translate transitions;
- reduced-motion preferences are respected.

## Validation

Commands executed during the redesign:

```sh
pnpm --dir web lint
pnpm --dir web check
pnpm --dir web test
pnpm --dir web build
go test ./...
docker build -t nostos:latest .
docker compose -p nostos-redesign up -d --build
curl -fsS http://localhost:17020/health/ready
docker compose -p nostos-redesign logs --tail=120 app worker
```

Browser validation was performed with temporary Playwright tooling and Chromium against the Docker Compose app at `http://localhost:17020`.

Validated browser flow:

- first-run owner setup;
- Providers screen;
- provider creation against a mock OpenAI-compatible server;
- model refresh;
- Chat screen;
- streamed chat response from the mock provider;
- Agents screen;
- Memories screen;
- Tasks screen;
- MCP screen;
- Settings screen.

Representative screenshots were generated under `/tmp/nostos-redesign-*.png` during validation.

## Remaining Polish Ideas for v0.1.1

- Add persistent screen-level filter state for conversations, memories, tasks, and MCP tools.
- Add a compact mode for very small laptop screens.
- Move more orchestration state out of `WorkspaceApp.svelte` into domain stores.
- Add dedicated visual-regression screenshots to CI after deciding on a stable browser test strategy.
- Add inline tooltips for dense controls such as task policies and MCP permissions.
