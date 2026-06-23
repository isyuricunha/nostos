<script lang="ts">
  import { onMount } from 'svelte';
  import Icon from '../../components/common/Icon.svelte';
  import type { IconName } from '../../components/common/Icon.svelte';
  import type { Conversation, User } from '../../lib/types';
  import { strings } from '../../strings';

  export let activeView: string;
  export let navItems: string[] = [];
  export let user: User;
  export let conversations: Conversation[] = [];
  export let selectedConversationId = '';
  export let submitting = false;
  export let minimizedView = '';
  export let onLogout: () => void | Promise<void>;
  export let onCreateConversation: () => void | Promise<void>;
  export let onSelectConversation: (conversationId: string) => void | Promise<void>;
  export let onRenameConversation: (conversation: Conversation) => void | Promise<void>;
  export let onArchiveConversation: (conversation: Conversation) => void | Promise<void>;
  export let onUnarchiveConversation: (conversation: Conversation) => void | Promise<void>;
  export let onDeleteConversation: (conversation: Conversation) => void | Promise<void>;
  export let onRestoreWindow: (view: string) => void = () => undefined;

  const navIcons: Record<string, IconName> = {
    Chat: 'chat',
    Agents: 'agent',
    Memories: 'brain',
    Tasks: 'tasks',
    Tools: 'tools',
    Settings: 'gear'
  };

  let search = '';
  let collapsed = false;
  let mobileOpen = false;
  let chatsExpanded = true;
  let showArchived = false;
  let showAllConversations = false;
  let openConversationMenuId = '';

  onMount(() => {
    collapsed = localStorage.getItem('nostos-sidebar-collapsed') === 'true';
    function handleDocumentClick(event: MouseEvent): void {
      const target = event.target;
      if (!(target instanceof Element)) return;
      if (target.closest('.row-menu, .row-menu-button')) return;
      openConversationMenuId = '';
    }
    document.addEventListener('click', handleDocumentClick);
    return () => document.removeEventListener('click', handleDocumentClick);
  });

  $: activeConversations = conversations.filter((conversation) => !conversation.archived_at);
  $: archivedConversations = conversations.filter((conversation) => conversation.archived_at);
  $: searchedConversations = (showArchived ? archivedConversations : activeConversations).filter((conversation) =>
    conversation.title.toLowerCase().includes(search.trim().toLowerCase())
  );
  $: visibleConversations = searchedConversations.slice(0, showAllConversations ? 60 : 12);
  $: selectedConversation = conversations.find((conversation) => conversation.id === selectedConversationId);
  $: messageTarget = selectedConversation?.title ?? strings.chat.newConversation;

  function toggleCollapsed(): void {
    collapsed = !collapsed;
    localStorage.setItem('nostos-sidebar-collapsed', String(collapsed));
  }

  async function createConversation(): Promise<void> {
    activeView = strings.nav.chat;
    mobileOpen = false;
    await onCreateConversation();
  }

  async function openConversation(conversationId: string): Promise<void> {
    activeView = strings.nav.chat;
    mobileOpen = false;
    await onSelectConversation(conversationId);
  }

  function openView(view: string): void {
    activeView = view;
    mobileOpen = false;
  }

  function initials(name: string): string {
    return name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase())
      .join('') || 'N';
  }

  function handleConversationKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      openConversationMenuId = '';
      return;
    }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    const buttons = Array.from(document.querySelectorAll<HTMLButtonElement>('.conversation-row-main'));
    const index = buttons.indexOf(event.currentTarget as HTMLButtonElement);
    if (index === -1) return;
    event.preventDefault();
    const nextIndex = event.key === 'ArrowDown' ? Math.min(index + 1, buttons.length - 1) : Math.max(index - 1, 0);
    buttons[nextIndex]?.focus();
  }
</script>

<main class="workspace-shell" class:sidebar-collapsed={collapsed} class:sidebar-open={mobileOpen}>
  {#if activeView === strings.nav.chat}
    <button aria-label="Open navigation" class="mobile-sidebar-button" on:click={() => (mobileOpen = true)} type="button">
      <Icon name="menu" size={18} />
    </button>
  {/if}

  {#if mobileOpen}
    <button aria-label="Close navigation" class="sidebar-scrim" on:click={() => (mobileOpen = false)} type="button"></button>
  {/if}

  <aside class="workspace-sidebar" aria-label="Nostos workspace navigation">
    <header class="sidebar-top">
      <button aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'} class="sidebar-icon-button" on:click={toggleCollapsed} type="button">
        <Icon name="menu" size={17} />
      </button>
      <button class="sidebar-brand" on:click={createConversation} type="button">
        <span aria-hidden="true" class="brand-glyph">N</span>
        <span class="sidebar-label">Nostos</span>
      </button>
    </header>

    <div class="sidebar-actions">
      <button class="sidebar-action primary" on:click={createConversation} type="button">
        <Icon name="plus" size={15} />
        <span class="sidebar-label">New Chat</span>
      </button>
      <label class="conversation-search">
        <span class="visually-hidden">Search conversations</span>
        <Icon name="search" size={14} />
        <input bind:value={search} placeholder="Search" />
      </label>
    </div>

    <section class="sidebar-section conversation-section" aria-label="Conversations">
      <button
        aria-expanded={chatsExpanded}
        class="section-toggle"
        on:click={() => (chatsExpanded = !chatsExpanded)}
        type="button"
      >
        <Icon name="chat" size={13} />
        <span class="sidebar-label">Chats</span>
        <span class="section-count">{searchedConversations.length}</span>
        <Icon name="chevron-down" size={12} />
      </button>

      {#if chatsExpanded && !collapsed}
        <div class="archive-toggle-row">
          <button class:active={!showArchived} on:click={() => (showArchived = false)} type="button">Recent</button>
          <button class:active={showArchived} on:click={() => (showArchived = true)} type="button">Archived</button>
        </div>

        <div class="conversation-list" role="listbox">
          {#if visibleConversations.length === 0}
            <p class="sidebar-empty">{showArchived ? 'No archived conversations.' : strings.chat.noConversations}</p>
          {:else}
            {#each visibleConversations as conversation (conversation.id)}
              <article class="conversation-row" class:active={conversation.id === selectedConversationId}>
                <button
                  aria-selected={conversation.id === selectedConversationId}
                  class="conversation-row-main"
                  on:click={() => openConversation(conversation.id)}
                  on:keydown={handleConversationKeydown}
                  role="option"
                  type="button"
                >
                  <Icon name={conversation.archived_at ? 'archive' : 'chat'} size={13} />
                  <span class="conversation-title">{conversation.title}</span>
                  {#if conversation.summary}
                    <span aria-label="Summary available" class="conversation-dot"></span>
                  {/if}
                </button>
                <button
                  aria-expanded={openConversationMenuId === conversation.id}
                  aria-label={`Conversation menu for ${conversation.title}`}
                  class="row-menu-button"
                  on:click={() => (openConversationMenuId = openConversationMenuId === conversation.id ? '' : conversation.id)}
                  type="button"
                >
                  <Icon name="kebab" size={14} />
                </button>
                {#if openConversationMenuId === conversation.id}
                  <div class="row-menu" role="menu">
                    <button on:click={() => { openConversationMenuId = ''; onRenameConversation(conversation); }} type="button">
                      <Icon name="edit" size={13} /> Rename
                    </button>
                    {#if conversation.archived_at}
                      <button on:click={() => { openConversationMenuId = ''; onUnarchiveConversation(conversation); }} type="button">
                        <Icon name="archive" size={13} /> Restore
                      </button>
                    {:else}
                      <button on:click={() => { openConversationMenuId = ''; onArchiveConversation(conversation); }} type="button">
                        <Icon name="archive" size={13} /> Archive
                      </button>
                    {/if}
                    <button class="danger" on:click={() => { openConversationMenuId = ''; onDeleteConversation(conversation); }} type="button">
                      <Icon name="trash" size={13} /> Delete
                    </button>
                  </div>
                {/if}
              </article>
            {/each}
          {/if}
        </div>

        {#if searchedConversations.length > visibleConversations.length}
          <button class="show-more" on:click={() => (showAllConversations = true)} type="button">
            Show {searchedConversations.length - visibleConversations.length} more
          </button>
        {/if}
      {/if}
    </section>

    <nav class="workspace-shortcuts" aria-label="Workspace windows">
      {#each navItems as item (item)}
        <button
          class:active={activeView === item}
          class="shortcut-row"
          on:click={() => openView(item)}
          type="button"
        >
          <Icon name={navIcons[item] ?? 'grid'} size={15} />
          <span class="sidebar-label">{item}</span>
        </button>
      {/each}
    </nav>

    <footer class="sidebar-footer">
      {#if minimizedView}
        <button class="minimized-chip" on:click={() => onRestoreWindow(minimizedView)} type="button">
          <Icon name={navIcons[minimizedView] ?? 'window'} size={14} />
          <span class="sidebar-label">{minimizedView}</span>
        </button>
      {/if}
      <button class="profile-button" type="button" title={user.email}>
        <span class="profile-avatar">{initials(user.display_name)}</span>
        <span class="profile-copy sidebar-label">
          <strong>{user.display_name}</strong>
          <small>{messageTarget}</small>
        </span>
      </button>
      <button class="shortcut-row settings-row" class:active={activeView === strings.nav.settings} on:click={() => openView(strings.nav.settings)} type="button">
        <Icon name="gear" size={15} />
        <span class="sidebar-label">Settings</span>
      </button>
      <button class="logout-button" disabled={submitting} on:click={onLogout} type="button">
        <Icon name="close" size={14} />
        <span class="sidebar-label">{strings.auth.signOut}</span>
      </button>
    </footer>
  </aside>

  <section class="workspace-canvas" aria-label="Chat workspace">
    <slot />
  </section>
</main>
