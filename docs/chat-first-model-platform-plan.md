# Chat-First Model Platform Plan

## Current Architecture

Nostos v0.1.0 is a single Go binary with `server`, `worker`, `migrate`, `doctor`, and `version` commands. The Svelte SPA is compiled into the runtime image and served by the Go server. PostgreSQL and SQLite share explicit migrations under `migrations/`.

The backend has domain services for authentication, providers, chat, agents, memories, MCP, tasks, feedback, replies, health, and workers. Provider models are currently stored in `provider_models`, keyed by `provider_id + model_id`, and refreshed synchronously by `POST /api/v1/providers/{id}/models/refresh`.

The frontend already has a redesigned dark shell and reusable components, but `web/src/views/WorkspaceApp.svelte` still orchestrates most workspace state. `ChatView.svelte` still owns a permanent conversation rail and direct provider/model/agent selects.

## Identified UX Problems

- The primary shell still treats Chat, Agents, Memories, Tasks, MCP, Providers, and Settings as equivalent sections.
- Conversations are nested inside the Chat screen instead of being the core navigation surface.
- The chat toolbar exposes provider/model/agent as plain form controls and makes configuration feel more important than the conversation.
- Assistant-message secondary actions are always visible, including feedback controls, reply drafting, memory creation, and regeneration.
- There is no single reusable model picker. Provider, agent, task, and chat forms all use text inputs or provider-local selects.
- Settings lacks a focused model defaults area for chat, utility, and vision roles.
- Large provider catalogs are not handled as a cached platform resource.

## Current Model-Resolution Paths

- Chat runs resolve model in this order: run input, conversation model, agent default model, provider default model.
- Branch/regeneration runs use the same model fallback path.
- Conversation summaries resolve to conversation provider/model, provider default model, provider fallback model, then first enabled provider.
- Reply drafts use source message provider/model unless explicit input is supplied.
- Agent tasks resolve task provider/model, agent default provider/model, provider default model, and agent fallback only after an initial provider request failure.
- Provider default and fallback models are raw model ID strings, not provider-scoped references.

## Proposed Database Migrations

Add forward-only migrations for PostgreSQL and SQLite.

- Extend `providers` with model-refresh metadata:
  - `model_refresh_state`
  - `model_refresh_started_at`
  - `model_refresh_completed_at`
  - `model_refresh_duration_ms`
  - `model_refresh_error_category`
  - `model_refresh_error_message`
- Extend `provider_models` with catalog metadata:
  - `workspace_id`
  - `enabled`
  - `manually_added`
  - `available`
  - `first_seen_at`
  - `last_seen_at`
  - `last_successful_probe_at`
  - `last_failed_probe_at`
  - `last_error_category`
  - `last_safe_error_message`
  - `capabilities`
  - `capability_source`
  - `search_text`
- Add indexes for workspace, provider, model ID, availability, enabled state, and updated time.
- Add `model_role_bindings` for ordered global role fallbacks:
  - `workspace_id`
  - `role` (`chat`, `utility`, `vision`)
  - `position`
  - `provider_id`
  - `model_id`
  - `created_at`
  - `updated_at`

Migration compatibility:

- Existing `provider_models` rows are preserved.
- Existing provider default models are converted to primary `chat` and `utility` role bindings where possible.
- Existing raw model references remain valid as manual model IDs instead of being discarded.

## Proposed API Changes

- Keep existing provider routes compatible.
- Change `POST /api/v1/providers/{id}/models/refresh` to start an asynchronous refresh and return `202 Accepted`.
- Add `GET /api/v1/providers/{id}/models/refresh-status`.
- Add `GET /api/v1/models` for cached model catalog queries across providers.
- Add `POST /api/v1/models` for manual model entries.
- Add `PATCH /api/v1/models/{id}` for enabled/capability/display metadata.
- Add `POST /api/v1/providers/{id}/models/cleanup-unavailable`.
- Add `GET /api/v1/model-roles`.
- Add `PUT /api/v1/model-roles/{role}`.

## Frontend Component Plan

- Add a reusable `ModelPicker.svelte` component with provider grouping, search, full ID display, keyboard navigation, manual entry, capability badges, and unavailable markers.
- Add Settings model defaults and catalog management sections.
- Refactor the app shell so the primary sidebar contains New Chat, conversation search, recent conversations, compact shortcuts, and the owner menu.
- Remove the permanent conversation rail from `ChatView`.
- Move chat provider/model/agent controls into a compact conversation toolbar/composer control row.
- Move message secondary actions into a compact menu and add message details display.
- Keep providers, agents, tasks, and settings functional while progressively replacing raw model inputs with the model picker.

## Backward Compatibility Plan

- Existing conversations, agents, providers, tasks, reply drafts, feedback, and tool calls continue to store `provider_id + model`.
- New model roles are used only when existing explicit overrides are empty.
- Provider-scoped full model IDs remain unchanged.
- Provider default and fallback model fields continue to work for v0.1 clients and are used as migration seeds.
- Failed catalog refreshes keep existing cached models available.

## Implementation Phases

1. Add model-platform migrations, config, provider catalog fields, refresh metadata, and repository support.
2. Add model role bindings, resolver service, and API routes.
3. Wire role resolution into chat summaries, reply drafts, and task defaults without breaking explicit overrides.
4. Add asynchronous stale-while-revalidate model refresh and large-catalog tests.
5. Add the reusable frontend model picker and settings model defaults.
6. Convert chat to a chat-first shell with conversations in the primary sidebar.
7. Move message secondary actions into menus and add message details.
8. Add E2E coverage for large catalogs and chat-first model selection.
9. Update documentation and run validation.

## Test Plan

Backend:

- Provider-scoped model uniqueness and full ID preservation.
- Large catalog import with at least 800 models.
- Refresh failure preserves existing cache.
- Missing refresh models are marked unavailable, not deleted.
- Concurrent refresh prevention.
- Manual model addition and cleanup of unavailable models.
- Role model resolution and fallback order.
- Utility model use for summaries and reply drafts.
- Fresh and upgrade migrations for PostgreSQL and SQLite.

Frontend:

- Model picker search, grouping, full ID display, unavailable state, manual entry, and keyboard navigation.
- Settings model-role selection and fallback editing.
- Chat composer model selection.
- Message action menu without permanent feedback form clutter.

Browser E2E:

- Mock provider returns at least 800 models.
- Async refresh completes and cached models load from DB.
- Search finds full model IDs.
- Chat uses selected Chat Model.
- Summary or title uses Utility Model.
- Refresh failure preserves cache.
- No console errors and no permanent thumbs-up/down form under assistant messages.
