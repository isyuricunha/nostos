<script lang="ts">
  import SidebarItem from '../../components/common/SidebarItem.svelte';
  import StatusPill from '../../components/common/StatusPill.svelte';
  import type { Conversation, ReadyStatus, User } from '../../lib/types';
  import { strings } from '../../strings';

  export let activeView: string;
  export let navItems: string[] = [];
  export let user: User;
  export let status: ReadyStatus | null = null;
  export let conversations: Conversation[] = [];
  export let selectedConversationId = '';
  export let submitting = false;
  export let onLogout: () => void | Promise<void>;
  export let onCreateConversation: () => void | Promise<void>;
  export let onSelectConversation: (conversationId: string) => void | Promise<void>;
  export let onRenameConversation: (conversation: Conversation) => void | Promise<void>;
  export let onArchiveConversation: (conversation: Conversation) => void | Promise<void>;
  export let onDeleteConversation: (conversation: Conversation) => void | Promise<void>;

  const navIcons: Record<string, string> = {
    Chat: 'C',
    Agents: 'A',
    Memories: 'M',
    Tasks: 'T',
    MCP: 'P',
    Providers: 'I',
    Settings: 'S'
  };

  let search = '';

  $: readyTone = (status?.ready ? 'success' : 'warning') as 'success' | 'warning';
  $: readyLabel = status?.ready ? 'Ready' : 'Checking';
  $: visibleConversations = conversations
    .filter((conversation) => !conversation.archived_at)
    .filter((conversation) => conversation.title.toLowerCase().includes(search.trim().toLowerCase()))
    .slice(0, 40);
  $: title = activeView === strings.nav.chat ? (conversations.find((item) => item.id === selectedConversationId)?.title ?? strings.nav.chat) : activeView;

  async function openConversation(conversationId: string): Promise<void> {
    activeView = strings.nav.chat;
    await onSelectConversation(conversationId);
  }
</script>

<main class="app-shell chat-first-shell">
  <aside class="sidebar chat-sidebar" aria-label="Workspace navigation">
    <div class="brand">
      <span class="brand-mark" aria-hidden="true">N</span>
      <div>
        <strong>{strings.appName}</strong>
        <span>Chat-first AI workspace</span>
      </div>
    </div>

    <button class="new-chat-button" on:click={onCreateConversation} type="button">{strings.chat.newConversation}</button>

    <label class="sidebar-search">
      <span>Search conversations</span>
      <input bind:value={search} placeholder="Search conversations" />
    </label>

    <section class="conversation-nav" aria-label="Recent conversations">
      <div class="sidebar-section-title">
        <span>Recent</span>
        <small>{visibleConversations.length}</small>
      </div>
      {#if visibleConversations.length === 0}
        <p class="sidebar-empty">Start a chat to create persistent context.</p>
      {:else}
        {#each visibleConversations as conversation (conversation.id)}
          <article class:active={conversation.id === selectedConversationId} class="conversation-nav-item">
            <button on:click={() => openConversation(conversation.id)} type="button">
              <strong>{conversation.title}</strong>
              <span>{new Date(conversation.updated_at).toLocaleDateString()}</span>
            </button>
            <div class="conversation-actions" aria-label={`Actions for ${conversation.title}`}>
              <button aria-label="Rename conversation" on:click={() => onRenameConversation(conversation)} type="button">R</button>
              <button aria-label="Archive conversation" on:click={() => onArchiveConversation(conversation)} type="button">A</button>
              <button aria-label="Delete conversation" on:click={() => onDeleteConversation(conversation)} type="button">D</button>
            </div>
          </article>
        {/each}
      {/if}
    </section>

    <nav class="shortcut-nav" aria-label="Workspace shortcuts">
      {#each navItems as item (item)}
        <SidebarItem
          active={activeView === item}
          icon={navIcons[item] ?? item.slice(0, 1)}
          label={item}
          onSelect={() => (activeView = item)}
        />
      {/each}
    </nav>

    <div class="sidebar-footer">
      <StatusPill status={readyLabel} tone={readyTone} />
      <div class="sidebar-user">
        <span>{user.display_name}</span>
        <small>{user.email}</small>
      </div>
      <button disabled={submitting} on:click={onLogout} type="button">{strings.auth.signOut}</button>
    </div>
  </aside>

  <section class="workspace" aria-labelledby="workspace-title">
    <header class="topbar compact-topbar">
      <div>
        <p class="eyebrow">{activeView === strings.nav.chat ? 'Conversation' : 'Workspace'}</p>
        <h1 id="workspace-title">{title}</h1>
      </div>
      <div class="topbar-meta">
        {#if status}
          <StatusPill status={`${status.database.driver ?? 'database'} ${status.database.ok ? 'online' : 'offline'}`} tone={status.database.ok ? 'success' : 'danger'} />
        {/if}
        <span>{new Date().toLocaleDateString()}</span>
      </div>
    </header>

    <slot />
  </section>
</main>

<style>
  .chat-first-shell {
    grid-template-columns: minmax(270px, 320px) 1fr;
  }

  .chat-sidebar {
    gap: var(--space-3);
    overflow-y: auto;
  }

  .new-chat-button {
    width: 100%;
    justify-content: center;
    border-color: rgba(242, 183, 99, 0.42);
    background:
      linear-gradient(180deg, rgba(255, 221, 156, 0.17), rgba(213, 141, 45, 0.08)),
      rgba(213, 141, 45, 0.16);
    color: var(--color-accent-strong);
    font-weight: 760;
  }

  .sidebar-search {
    gap: var(--space-2);
  }

  .sidebar-search span {
    color: var(--color-subtle);
    font-size: var(--font-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .sidebar-search input {
    padding: 9px 10px;
    border-radius: var(--radius-sm);
  }

  .conversation-nav {
    display: grid;
    min-height: 0;
    gap: var(--space-2);
    overflow: auto;
  }

  .sidebar-section-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    color: var(--color-subtle);
    font-size: var(--font-xs);
    font-weight: 720;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .sidebar-empty {
    margin: 0;
    border: 1px dashed var(--color-border-muted);
    border-radius: var(--radius-md);
    padding: var(--space-3);
    color: var(--color-subtle);
    font-size: var(--font-sm);
  }

  .conversation-nav-item {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: var(--space-2);
    border: 1px solid transparent;
    border-radius: var(--radius-md);
    background: transparent;
    padding: 4px;
    transition:
      border-color var(--duration-fast) var(--ease-standard),
      background-color var(--duration-fast) var(--ease-standard);
  }

  .conversation-nav-item:hover,
  .conversation-nav-item.active {
    border-color: var(--color-border-muted);
    background: rgba(255, 255, 255, 0.035);
  }

  .conversation-nav-item.active {
    background: var(--color-accent-muted);
  }

  .conversation-nav-item > button {
    display: grid;
    min-width: 0;
    gap: 2px;
    border: 0;
    background: transparent;
    padding: 7px;
    text-align: left;
    transform: none;
  }

  .conversation-nav-item strong,
  .conversation-nav-item span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .conversation-nav-item strong {
    color: var(--color-text);
    font-size: var(--font-sm);
  }

  .conversation-nav-item span {
    color: var(--color-subtle);
    font-size: var(--font-xs);
  }

  .conversation-actions {
    display: flex;
    gap: 3px;
    opacity: 0;
    transition: opacity var(--duration-fast) var(--ease-standard);
  }

  .conversation-nav-item:hover .conversation-actions,
  .conversation-nav-item:focus-within .conversation-actions {
    opacity: 1;
  }

  .conversation-actions button {
    display: grid;
    width: 24px;
    height: 24px;
    place-items: center;
    border-radius: var(--radius-xs);
    padding: 0;
    color: var(--color-subtle);
    font-size: 0.68rem;
  }

  .shortcut-nav {
    display: grid;
    gap: var(--space-1);
    border-top: 1px solid var(--color-border-muted);
    padding-top: var(--space-3);
  }

  .chat-sidebar :global(.sidebar-footer) {
    gap: var(--space-2);
    padding-top: var(--space-3);
  }

  .compact-topbar h1 {
    max-width: min(68vw, 880px);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  @media (max-width: 860px) {
    .chat-first-shell {
      grid-template-columns: 1fr;
    }

    .chat-sidebar {
      position: relative;
      min-height: auto;
      max-height: none;
    }

    .conversation-nav {
      max-height: 260px;
    }
  }
</style>
