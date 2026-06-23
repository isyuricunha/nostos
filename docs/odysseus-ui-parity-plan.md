# Odysseus UI Parity Plan

Reference audited: `pewdiepie-archdaemon/odysseus` `origin/dev` at `7e5db9a3c6346caaabf1f0f49bab4be39be2ffe7`.

This is a clean-room implementation plan. The reference was inspected for interaction grammar, information density, and workflow shape only. Nostos will keep its name, backend architecture, API routes, PostgreSQL and SQLite support, amber accent token, and current product capabilities.

## Reference Observations

### Application shell

- The desktop experience is a persistent AI workspace, not a route-by-route administration dashboard.
- Chat stays mounted as the primary canvas while secondary tools open as windows above it.
- Navigation is compact, mostly row-based, with a fixed sidebar and optional icon-only collapsed state.
- The sidebar is full height, with top actions, dense conversation rows, tool launchers, and a bottom profile/settings area.
- Technical health and provider administration are not primary chrome.

### Dimensions

- Sidebar rows are roughly native-app dense: small icon, 10-13px supporting text, 24-32px row height.
- Tool windows typically sit in the 600-720px width range, with larger settings windows using a left nav and scrollable panel.
- The main conversation column is centered, with assistant messages wider than user messages and the composer near the lower center.
- Corners are modest, usually 4-8px for controls and 8-12px for windows/messages.

### Visual hierarchy

- The chat canvas carries the page. Secondary data appears through compact windows.
- The top chat label is subtle metadata, not a page title.
- Action affordances are mostly icons and hover/focus menus, with text labels only where needed.
- Status is visible but quiet: dots, small labels, or low-saturation pills.

### Density

- Lists are single-row or two-line rows by default.
- Forms are not permanently visible. Creation and editing happen after an explicit action.
- Cards are used sparingly for repeated items, not as full page containers.
- Spacing is tight, with thin borders and restrained contrast.

### Navigation

- Primary navigation exposes chat, conversations, tools, profile, and settings.
- Tool concepts open windows. They do not replace the chat canvas.
- Provider and model administration live under settings/model areas instead of primary navigation.
- Conversation actions are grouped behind row menus.

### Window behavior

- Tool surfaces are dark, bordered, compact windows with title bars.
- Windows can be brought forward, minimized, closed, dragged on desktop, and adapted to full-screen sheets on mobile.
- State preservation on minimize matters more than complex multi-window tiling for Nostos.
- Escape should close transient menus first, then windows where appropriate.

### Message anatomy

- User messages align right, use a compact bubble/card, and reveal actions on hover/focus.
- Assistant messages align left, are wider, show compact model identity and time, and carry Markdown/code content.
- Message footers expose compact metrics and actions only when data exists.
- Details are popovers/panels, not permanent blocks under every message.

### Composer anatomy

- The composer is a fixed, centered, bounded input bar.
- It contains multiline input, add/tool controls, mode selection, model selection, and send/stop.
- Model selection belongs inside the composer; catalog management belongs in settings.
- Enter sends, Shift+Enter inserts a newline, and Escape closes menus.

### Settings organization

- Settings is a large workspace window with a compact title bar, a left navigation column, and a scrollable active panel.
- Model administration is organized into add provider, providers, and AI defaults.
- Appearance and shortcuts are settings panels, not separate primary pages.
- System and session details belong under administration.

### Model management

- The model picker is a searchable popover with grouping, full model IDs, provider information, offline state, and keyboard navigation.
- Large catalogs need either virtual rendering or explicit incremental rendering.
- Refreshing model catalogs is a settings/provider action, not chat toolbar chrome.

### Memories organization

- Memories are a Brain-style window with tabs for browse, add, and settings.
- The browse state prioritizes search, filters, counts, compact rows/cards, pinning, source, use count, and last-used metadata.
- Create/edit is not permanently displayed.
- Nostos keeps explicit memories only.

### Tasks organization

- Tasks are shown as compact operational rows with status, schedule, next run, last run, run-now, and menu actions.
- Run history is dense and expandable.
- Create/edit is a dedicated editor panel or modal.

### Tools organization

- The primary label is `Tools`.
- MCP remains visible in technical details through tabs for MCP servers, discovered tools, permissions, and approvals.
- Rows and tabs replace full-page empty cards.

### Responsive behavior

- Desktop is primary.
- Tablet collapses the sidebar and bounds windows to the viewport.
- Mobile uses a drawer sidebar, pinned composer, full-screen or bottom-sheet windows, and avoids horizontal overflow.

### Interaction states and accessibility

- Hover states reveal secondary actions; focus states expose the same controls for keyboard users.
- Menus need Escape handling, outside-click dismissal, visible focus, and role/label semantics.
- Dialog windows need focus management and keyboard close/minimize controls.
- Rows should be button-accessible and keep labels readable when truncated.

## Nostos Implementation Map

### Shell

- Replace `AppShell.svelte` with a fixed workspace layout:
  - compact sidebar;
  - persistent chat canvas;
  - overlay window layer;
  - mobile drawer state;
  - collapsed state persisted in `localStorage`.
- Remove providers from primary sidebar. Add `Tools`, `Agents`, `Memories`, `Tasks`, and `Settings` as window launchers.
- Keep `WorkspaceApp.svelte` as the state owner for API calls in this milestone.

### Icons

- Add an original internal SVG icon component with a small fixed icon set.
- Use icons for buttons, conversation rows, tool shortcuts, message actions, and window title bars.
- Avoid emoji and one-letter fake icons.

### Chat canvas

- Convert `ChatView.svelte` into a mounted canvas:
  - subtle centered conversation metadata;
  - summary in a compact title popover;
  - fixed centered composer;
  - message rail with bottom padding for composer;
  - optional low-contrast Nostos trace background.

### Messages

- Add compact `MessageActions` and details popover behavior inside `ChatView.svelte`.
- Keep Markdown rendering through existing `marked` and `DOMPurify`.
- Use available Nostos fields now and render absent metrics conditionally.
- Move report feedback behind `Report response`.

### Composer

- Move agent and model controls into the composer.
- Remove chat-level provider badges and catalog refresh.
- Support Enter, Shift+Enter, Escape, send, and stop.
- Reuse the shared `ModelPicker`.

### Workspace windows

- Add reusable Svelte components:
  - `WorkspaceWindow.svelte`;
  - `WindowLayer.svelte` if useful;
  - lightweight z-index/position state in `WorkspaceApp.svelte`.
- Support one reliable active window with minimize and close, plus remembered desktop position/size where practical.
- On mobile, render windows as full-screen panels.

### Settings

- Rebuild `SettingsView.svelte` as dense settings window content:
  - `Models / Add Provider`;
  - `Models / Providers`;
  - `Models / AI Defaults`;
  - Search;
  - Integrations;
  - Appearance;
  - Shortcuts;
  - Account;
  - Administration / Agent Tools;
  - Administration / Sessions;
  - Administration / System.
- Move provider management into the settings window while preserving provider routes and actions.

### Model picker

- Restyle `ModelPicker.svelte` as a compact searchable popover.
- Remove the fixed 120-result cap.
- Implement incremental rendering with a visible result count and load-more control for large catalogs.
- Add keyboard arrow navigation, Enter selection, Escape close, manual model option, and copy full ID.

### Memories

- Rebuild `MemoriesView.svelte` as Brain-style dense window content.
- Tabs: Memories, Add, Settings.
- Add search, scope/tag filters, sort, active toggle, compact cards, and row menus.

### Agents

- Rebuild `AgentsView.svelte` as dense rows with a create/edit modal.
- Rows show icon, name, active state, model, memory mode, tool policy, description, and overflow actions.

### Tasks

- Rebuild `TasksView.svelte` as dense rows plus run history/details.
- Add user/system filters, search, status filters, run now, and row menus.

### Tools

- Rebuild `MCPView.svelte` as a `Tools` window with MCP Servers, Discovered Tools, Permissions, and Approvals tabs.
- Keep MCP labels inside technical details and form labels.

### Appearance

- Replace the current dashboard theme with near-black canvas, dark sidebar/window/message surfaces, thin neutral borders, compact spacing, and amber accent.
- Add appearance controls for background pattern, density, UI scale, accent color, message width, and reduced motion.
- Store appearance preferences in `localStorage`.

## Milestone Checklist

- [x] Audit Nostos current frontend and Odysseus reference.
- [x] Record Odysseus reference commit SHA.
- [x] Replace shell with persistent chat workspace.
- [x] Add internal SVG icon set.
- [x] Add workspace window manager.
- [x] Rebuild chat messages and composer.
- [x] Move settings and providers into settings window.
- [x] Rebuild Memories, Agents, Tasks, and Tools windows.
- [x] Restyle and harden ModelPicker for large catalogs.
- [x] Add responsive sidebar/drawer/window behavior.
- [x] Add focused component and e2e coverage.
- [ ] Capture and inspect required screenshots.
- [ ] Run required Go, pnpm, Docker, PostgreSQL, and SQLite validation.
- [ ] Commit milestone-sized changes and push `feat/odysseus-ui-parity`.
