<script lang="ts">
  import { tick } from 'svelte';
  import Icon from '../components/common/Icon.svelte';
  import type { Memory } from '../lib/types';
  import { strings } from '../strings';

  export let memories: Memory[] = [];
  export let actionStates: Record<string, string> = {};
  export let editingMemoryId = '';
  export let memoryTitle = '';
  export let memoryContent = '';
  export let memoryTags = '';
  export let memoryScope = 'global';
  export let memoryImportance = 70;
  export let memoryPinned = true;
  export let memoryActive = true;
  export let onSubmit: () => void | Promise<void>;
  export let onCancelEdit: () => void;
  export let onRefresh: () => void | Promise<void>;
  export let onEdit: (memory: Memory) => void;
  export let onDelete: (memoryId: string) => void | Promise<void>;
  export let onToggleActive: (memory: Memory) => void | Promise<void>;
  export let onTogglePinned: (memory: Memory) => void | Promise<void>;

  type Tab = 'browse' | 'add' | 'settings';
  type SortKey = 'updated' | 'importance' | 'used';

  let activeTab: Tab = 'browse';
  let query = '';
  let scopeFilter = 'all';
  let tagFilter = '';
  let sortKey: SortKey = 'importance';
  let openMenuId = '';

  $: if (editingMemoryId) {
    activeTab = 'add';
  }
  $: scopes = ['all', ...Array.from(new Set(memories.map((memory) => memory.scope).filter(Boolean)))];
  $: tags = Array.from(new Set(memories.flatMap((memory) => memory.tags))).sort((left, right) => left.localeCompare(right));
  $: activeCount = memories.filter((memory) => memory.active).length;
  $: pinnedCount = memories.filter((memory) => memory.pinned).length;
  $: filteredMemories = memories
    .filter((memory) => {
      const haystack = `${memory.title} ${memory.content} ${memory.tags.join(' ')} ${memory.source}`.toLowerCase();
      const matchesSearch = haystack.includes(query.trim().toLowerCase());
      const matchesScope = scopeFilter === 'all' || memory.scope === scopeFilter;
      const matchesTag = !tagFilter || memory.tags.includes(tagFilter);
      return matchesSearch && matchesScope && matchesTag;
    })
    .sort((left, right) => {
      if (sortKey === 'used') return right.use_count - left.use_count;
      if (sortKey === 'updated') return Date.parse(right.updated_at ?? '') - Date.parse(left.updated_at ?? '');
      return right.importance - left.importance;
    });

  function openCreate(): void {
    onCancelEdit();
    activeTab = 'add';
  }

  function startEdit(memory: Memory): void {
    onEdit(memory);
    activeTab = 'add';
    openMenuId = '';
  }

  async function submitForm(): Promise<void> {
    await onSubmit();
    await tick();
    if (stateFor('memory-form') !== 'failed') {
      activeTab = 'browse';
    }
  }

  function stateFor(key: string): string {
    return actionStates[key] ?? '';
  }

  function cancelForm(): void {
    onCancelEdit();
    activeTab = 'browse';
  }

  function formatDate(value = ''): string {
    return value ? new Date(value).toLocaleString() : 'Never';
  }
</script>

<div class="window-tabs-layout">
  <header class="window-panel-toolbar">
    <div>
      <strong>Memories</strong>
      <span>{activeCount} active · {pinnedCount} pinned · {memories.length} total</span>
    </div>
    <div class="window-toolbar-actions">
      <button aria-label="Refresh memories" on:click={onRefresh} type="button"><Icon name="refresh" size={13} /></button>
      <button on:click={openCreate} type="button"><Icon name="plus" size={13} /> New memory</button>
    </div>
  </header>

  <nav class="segmented-tabs" aria-label="Memory tabs">
    <button class:active={activeTab === 'browse'} on:click={() => (activeTab = 'browse')} type="button">Memories</button>
    <button class:active={activeTab === 'add'} on:click={openCreate} type="button">Add</button>
    <button class:active={activeTab === 'settings'} on:click={() => (activeTab = 'settings')} type="button">Settings</button>
  </nav>

  {#if activeTab === 'browse'}
    <div class="window-filter-row">
      <label class="window-search">
        <Icon name="search" size={13} />
        <input bind:value={query} placeholder="Search memories" />
      </label>
      <select aria-label="Memory scope" bind:value={scopeFilter}>
        {#each scopes as scope (scope)}
          <option value={scope}>{scope === 'all' ? 'All scopes' : scope}</option>
        {/each}
      </select>
      <select aria-label="Memory tag" bind:value={tagFilter}>
        <option value="">All tags</option>
        {#each tags as tag (tag)}
          <option value={tag}>{tag}</option>
        {/each}
      </select>
      <select aria-label="Memory sort" bind:value={sortKey}>
        <option value="importance">Importance</option>
        <option value="used">Use count</option>
        <option value="updated">Updated</option>
      </select>
    </div>

    {#if filteredMemories.length === 0}
      <p class="window-empty">{strings.memories.noMemories}</p>
    {:else}
      <div class="memory-card-grid">
        {#each filteredMemories as memory (memory.id)}
          <article class="memory-compact-card" class:inactive={!memory.active}>
            <header>
              <span class="row-icon"><Icon name="brain" size={15} /></span>
              <div>
                <strong>{memory.title}</strong>
                <small>{memory.source} · used {memory.use_count} · last {formatDate(memory.last_used_at)}</small>
                {#if stateFor(`memory:${memory.id}:delete`) || stateFor(`memory:${memory.id}:active`) || stateFor(`memory:${memory.id}:pinned`)}
                  <small class="row-state">{stateFor(`memory:${memory.id}:delete`) || stateFor(`memory:${memory.id}:active`) || stateFor(`memory:${memory.id}:pinned`)}</small>
                {/if}
              </div>
              <button aria-label={`Memory menu for ${memory.title}`} on:click={() => (openMenuId = openMenuId === memory.id ? '' : memory.id)} type="button">
                <Icon name="kebab" size={14} />
              </button>
              {#if openMenuId === memory.id}
                <div class="row-menu row-menu-right" role="menu">
                  <button on:click={() => startEdit(memory)} type="button"><Icon name="edit" size={13} /> Edit</button>
                  <button on:click={() => { openMenuId = ''; onTogglePinned(memory); }} type="button">
                    <Icon name={memory.pinned ? 'minus' : 'check'} size={13} /> {memory.pinned ? 'Unpin' : 'Pin'}
                  </button>
                  <button on:click={() => { openMenuId = ''; onToggleActive(memory); }} type="button">
                    <Icon name={memory.active ? 'minus' : 'check'} size={13} /> {memory.active ? 'Disable' : 'Enable'}
                  </button>
                  <button class="danger" on:click={() => { openMenuId = ''; onDelete(memory.id); }} type="button">
                    <Icon name="trash" size={13} /> Delete
                  </button>
                </div>
              {/if}
            </header>
            <p>{memory.content}</p>
            <footer>
              <span>{memory.scope}</span>
              <span>importance {memory.importance}</span>
              {#if memory.pinned}<span>pinned</span>{/if}
              {#if !memory.active}<span>disabled</span>{/if}
            </footer>
            {#if memory.tags.length}
              <div class="tag-row">
                {#each memory.tags as tag (tag)}
                  <span>{tag}</span>
                {/each}
              </div>
            {/if}
          </article>
        {/each}
      </div>
    {/if}
  {:else if activeTab === 'add'}
    <section class="window-editor-panel inline" aria-label={editingMemoryId ? 'Edit memory' : 'Create memory'}>
      <header>
        <strong>{editingMemoryId ? 'Edit memory' : strings.memories.add}</strong>
        <button aria-label="Close memory editor" on:click={cancelForm} type="button"><Icon name="close" size={13} /></button>
      </header>
      <form class="compact-editor-form" on:submit|preventDefault={submitForm}>
        <label>
          Title
          <input bind:value={memoryTitle} required />
        </label>
        <label>
          Content
          <textarea bind:value={memoryContent} required></textarea>
        </label>
        <label>
          Tags
          <input bind:value={memoryTags} placeholder="project, preference" />
        </label>
        <div class="two-col">
          <label>
            Scope
            <select bind:value={memoryScope}>
              <option value="global">global</option>
              <option value="workspace">workspace</option>
              <option value="agent">agent</option>
              <option value="conversation">conversation</option>
            </select>
          </label>
          <label>
            Importance
            <input bind:value={memoryImportance} min="1" max="100" type="number" />
          </label>
        </div>
        <div class="two-col">
          <label class="toggle-line">
            <input bind:checked={memoryPinned} type="checkbox" />
            {strings.memories.pin}
          </label>
          <label class="toggle-line">
            <input bind:checked={memoryActive} type="checkbox" />
            Active
          </label>
        </div>
        <div class="editor-actions">
          <button type="submit">{editingMemoryId ? 'Save memory' : strings.memories.add}</button>
          {#if stateFor('memory-form')}
            <span class={`editor-state state-${stateFor('memory-form')}`}>{stateFor('memory-form')}</span>
          {/if}
          <button on:click={cancelForm} type="button">Cancel</button>
        </div>
      </form>
    </section>
  {:else}
    <section class="window-settings-note">
      <h3>Explicit memory behavior</h3>
      <p>Only active explicit memories are eligible for selection. Agent memory mode decides whether none, pinned, relevant, or all memories can be injected into a run.</p>
      <dl class="compact-dl two-col">
        <div><dt>Active</dt><dd>{activeCount}</dd></div>
        <div><dt>Pinned</dt><dd>{pinnedCount}</dd></div>
        <div><dt>Scopes</dt><dd>{scopes.length - 1}</dd></div>
        <div><dt>Tags</dt><dd>{tags.length}</dd></div>
      </dl>
    </section>
  {/if}
</div>
