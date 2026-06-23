# Chat-First Experience

The next Nostos interface direction is:

```text
Chat is the product. Everything else supports the chat.
```

## Shell

The primary sidebar contains:

- Nostos identity;
- New Conversation;
- conversation search;
- recent conversations;
- conversation actions;
- compact shortcuts to Chat, Agents, Memories, Tasks, MCP, Providers, and Settings;
- owner/session controls.

Conversations no longer live in a second permanent rail inside the Chat screen. Selecting a conversation switches the workspace to Chat and loads its messages.

## Chat Workspace

The main chat panel prioritizes message history and the composer. Agent and model controls are compact and live near the conversation controls rather than as a large configuration block.

The model selector uses the shared Model Picker, so chat uses the same provider-scoped catalog as settings, agents, providers, and tasks.

## Message Actions

Secondary actions are intentionally hidden behind a compact message menu:

- Copy;
- Draft reply;
- Create memory;
- View details;
- Report response;
- Regenerate.

Feedback is still persisted, but thumbs-up/down and negative reason controls are no longer permanently visible below every assistant message.

## Message Details

Message details are available on demand. The first implementation shows recorded provider ID, full model ID, timestamp, returned token count, and feedback state. More runtime metrics can be added as backend run metadata expands.

## Supporting Screens

Agents, Memories, Tasks, MCP, Providers, and Settings remain available as supporting workspaces. Creation and editing forms remain functional, but the product navigation now makes conversation work the default path.

Agents, Memories, Tasks, MCP servers, and Providers present their lists as the primary surface. Create and edit workflows open in focused modal panels instead of occupying permanent split-screen form columns.
