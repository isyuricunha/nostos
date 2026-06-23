# Odysseus UI Fidelity Plan

Reference: `pewdiepie-archdaemon/odysseus` `dev` at `8f5e36a0793e733ddaa14015a4844464251dcc62`.

This pass is a clean-room Svelte implementation. The reference was inspected for structure, density, state behavior, and interaction models only. Nostos keeps its Go backend, lightweight data model, cached model catalog, full model IDs, and existing API surfaces.

## Implementation Approach

1. Establish one compact dark token system matching the Odysseus surface hierarchy: charcoal canvas, slightly elevated panels, thin neutral borders, coral configurable accent, readable metadata, and consistent shadows.
2. Add one reusable feedback layer for toasts and async action states, then replace scattered `notice`/`errorMessage` handling with typed user feedback where practical.
3. Tighten the existing Svelte components in place: `AppShell.svelte`, `ChatView.svelte`, `ModelPicker.svelte`, `WorkspaceWindow.svelte`, and the workspace views.
4. Preserve immediate local state mutations that already exist, fill gaps for deletes, tests, refreshes, runs, and long-running provider/model/MCP/task states.
5. Validate with component tests first, then build/check, browser screenshots, and click-through.

## Screen-By-Screen Gap Analysis

| Screen | Current Nostos behavior | Odysseus reference behavior | Intended Nostos implementation | Likely files | Validation |
| --- | --- | --- | --- | --- | --- |
| Sidebar | Compact structure exists, but action contrast is weak and row/menu overflow can create horizontal pressure. | Fixed compact sidebar with persistent menu button, wordmark, New/Search, dense chat rows, hover-only kebab, sections, profile/settings/sign out; no x-scroll. | Keep the existing structure, reduce overflow, strengthen icon/button states, make row menu keyboard/outside-click aware, keep archived/recent and show more dense. | `AppShell.svelte`, `workspace.css`, sidebar tests | E2E sidebar overflow screenshot and no horizontal scroll assertion. |
| Chat header | Conversation menu is centered and subtle, but it mostly exposes summary only. | Small title/count control with compact menu; no status clutter. | Keep centered header, show `title · count`, add compact actions for rename, summary, archive, fork/delete where supported by Nostos APIs. | `ChatView.svelte`, `WorkspaceApp.svelte`, `workspace.css` | Component test for menu actions; screenshot. |
| Messages | Basic right/left cards exist, but assistant/user geometry is not close enough and actions are too generic. | User messages right/narrow; assistant messages left/wider; compact metadata, hover actions, model emphasis, clean Markdown. | Rebuild message card sizing, footer hierarchy, hover/focus action strip, streaming indicator, compact model label with full tooltip. | `ChatView.svelte`, `workspace.css` | Message layout tests and screenshot comparison. |
| Message footer | Shows a few metrics but sparse and inconsistent. | Compact stats/pills and small detail popover. | Show only present values, compact tokens/memories/tools/branch, keep copy/details/kebab as icon buttons. | `ChatView.svelte`, `workspace.css` | Unit test empty values hidden and details visible. |
| Composer | Centered and mostly correct, but controls clip with long model/agent names. | Bottom rounded input bar with multiline text, compact controls, always-visible send/stop, stable long labels. | Use grid layout with fixed send lane, bounded model/agent labels, full ID tooltip, stable mobile stacking, visible disabled/send states. | `ChatView.svelte`, `ModelPicker.svelte`, `workspace.css` | Long model/agent tests; screenshot; no overflow assertion. |
| Model picker | Popover search and load-more exist, but keyboard support is incomplete and selected/offline states are weak. | Compact searchable grouped picker with autofocus, health/manual/capability badges, copy ID, result counts, smooth large lists. | Add Home/End, clearer selected/offline/manual badges, action feedback, stable large result rendering, viewport-bounded popover. | `ModelPicker.svelte`, `workspace.css`, tests | Existing `ModelPicker.test.ts` plus keyboard/large-list tests. |
| Agent picker | Native select in composer can fight for space. | Compact selector integrated with chat mode controls. | Replace visual treatment with constrained dark select; keep full names in title and avoid pushing send button. | `ChatView.svelte`, `workspace.css` | Long agent name test. |
| Conversation menu | Sidebar row menu exists; header menu incomplete. | Kebab menus close on outside click/Escape and stay in viewport. | Add shared menu behavior for outside click/Escape, make conversation menus fixed/anchored enough to avoid viewport escape. | `AppShell.svelte`, `ChatView.svelte`, `workspace.css` | E2E menu close behavior. |
| Message menu | Kebab exists, but no outside-click handling and not all requested actions are represented. | Compact overflow menu anchored to message actions. | Add valid assistant/user actions, outside click/Escape, viewport-safe placement where feasible. Unsupported backend actions remain disabled/hidden, not fake. | `ChatView.svelte`, `WorkspaceApp.svelte`, `workspace.css` | Component and E2E tests. |
| Message statistics | Inline `<dl>` currently consumes card width. | Small anchored stats popover. | Convert details into compact popover with provider/model/tokens/memories/tools/branch/run state fields when available. | `ChatView.svelte`, `workspace.css` | Screenshot and unit test. |
| Workspace windows | Basic draggable modal exists; active z-index is static and controls are small. | Modal windows with clear title controls, active stacking, Esc close, mobile full-sheet. | Add active window state, stronger titlebar controls, bounded stored positions, focus restore, full-screen mobile. | `WorkspaceWindow.svelte`, `WorkspaceApp.svelte`, tests | Window tests and mobile screenshot. |
| Settings | Structure is close but width is oversized and controls can look generic. | Settings shell with left nav, compact active states, dense forms. | Tighten width, nav contrast, form states, provider workflow feedback, no fake modules. | `SettingsView.svelte`, `workspace.css` | Settings screenshots and form tests. |
| Providers | Compact list exists but refresh/test progress is global and lists can feel stale. | Provider rows show health/model counts, live test/refresh state, action menus. | Add per-provider async action state, show testing/refreshing/succeeded/failed, poll refresh status while keeping cached list visible. | `WorkspaceApp.svelte`, `SettingsView.svelte`, `types.ts` | Provider test/refresh tests. |
| AI Defaults | Three sections exist but save feedback and unsaved state are weak. | Compact vertical model chains with clear primary/fallback and save state. | Keep vertical sections, add dirty/saving/saved state and shared picker. | `SettingsView.svelte`, `WorkspaceApp.svelte` | Model-default save feedback test. |
| Memories | Tabs exist, but cards are larger than the Brain-style dense rows and edit consumes too much. | Brain modal with tabs, toolbar, compact memory list/cards, pin/active/menu states. | Tighten cards/rows, visible enabled/count/filter/sort, immediate local mutation on create/edit/delete/pin/disable, feedback toasts. | `MemoriesView.svelte`, `workspace.css`, `WorkspaceApp.svelte` | Memory reactivity test and screenshot. |
| Agents | List/editor exists, but empty space and row density still need polish. | Dense agent rows with count, search/filter, status, model, memory/tool policy, overflow. | Keep split editor only when needed, reduce blank area, make rows selected/dense, immediate mutation and feedback. | `AgentsView.svelte`, `workspace.css`, `WorkspaceApp.svelte` | Agent reactivity test and screenshot. |
| Tasks | Rows/tabs exist but live run polling is missing after queue. | Dense task log rows with running/queued/succeeded/failed indicators. | Add lightweight polling while active runs exist; show queued/claimed/running/succeeded/failed in rows and toast state. | `TasksView.svelte`, `WorkspaceApp.svelte`, `workspace.css` | Task status polling test. |
| Tools/MCP | Basic servers/tools/permissions tabs exist; no Approvals tab and tests lack visible progress. | Tools window exposes MCP servers, tools, permissions, approvals, compact rows, test/discover state. | Add Approvals tab using pending tool approvals already loaded, add server action state for testing/discovering, immediate updates. | `MCPView.svelte`, `WorkspaceApp.svelte`, `workspace.css` | MCP feedback test and screenshot. |
| Account | Identity exists; sign out is in sidebar only. | Account settings show identity, sign out, supported account operations. | Add sign-out action to Account if API supports current auth; no fake password change if unsupported. | `SettingsView.svelte`, `WorkspaceApp.svelte` | Click-through account flow. |
| Sessions | List/revoke exists but current-session distinction is absent. | Session list with client/IP/time/current and immediate revoke. | Keep available fields, add created/expires/current marker if backend exposes it, remove revoked session locally. | `SettingsView.svelte`, `WorkspaceApp.svelte` | Session revoke test. |
| Appearance | Local settings exist and preview immediately. | Theme controls alter accent, density, scale, motion, background. | Keep local persistence, extend tokens to use accent consistently, add sidebar collapsed preference to Appearance where practical. | `SettingsView.svelte`, `workspace.css`, `tokens.css` | Appearance screenshot and localStorage assertions. |
| Mobile interface | Mobile drawer and full-screen windows exist, but menus/picker need stronger bounds. | Sidebar drawer, full-sheet modals, input above safe area, no horizontal scroll. | Tighten mobile composer, windows, model picker sheet behavior, always visible menu controls. | `workspace.css`, `WorkspaceWindow.svelte`, `ModelPicker.svelte` | Mobile screenshots and no-x-scroll E2E. |

## Reference Behaviors to Preserve Independently

- Compact modal surfaces with thin borders and clear titlebar controls.
- Top-right non-blocking toast feedback with optional persistent failure and dismissal.
- Single-action overflow menus that stay inside the viewport and close predictably.
- Immediate local list mutation after successful create/edit/delete/toggle.
- Polling for long-running refresh/run operations where no live stream exists.
- Dense list rows over giant cards for agents, providers, tasks, tools, and memories.

## Non-Goals

- Copying Odysseus source code, markup, CSS rules, SVGs, prompts, database schema, or exact proprietary text.
- Adding a component library, Redis, Docker socket access, ChromaDB, SearXNG, or any heavy required service.
- Reintroducing Version 0.1.0 release work.
