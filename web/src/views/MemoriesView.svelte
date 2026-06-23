<script lang="ts">
  import EmptyState from '../components/common/EmptyState.svelte';
  import Modal from '../components/common/Modal.svelte';
  import StatusPill from '../components/common/StatusPill.svelte';
  import type { Memory } from '../lib/types';
  import { strings } from '../strings';

  export let memories: Memory[] = [];
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

  let formOpen = false;

  $: if (editingMemoryId) {
    formOpen = true;
  }

  function openCreate(): void {
    onCancelEdit();
    formOpen = true;
  }

  function closeForm(): void {
    onCancelEdit();
    formOpen = false;
  }

  async function submitForm(): Promise<void> {
    await onSubmit();
    formOpen = false;
  }
</script>

<section class="panel">
  <div class="panel-heading">
    <div>
      <p class="eyebrow">Explicit context</p>
      <h2>Memories</h2>
    </div>
    <div class="cluster">
      <button on:click={onRefresh} type="button">Refresh</button>
      <button on:click={openCreate} type="button">New memory</button>
    </div>
  </div>

  <Modal open={formOpen} title={editingMemoryId ? 'Edit memory' : strings.memories.add} onClose={closeForm}>
  <form class="form-grid" on:submit|preventDefault={submitForm}>
    <div class="form-section">
      <h3>Memory content</h3>
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
    </div>

    <div class="form-section">
      <h3>Use and scope</h3>
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
      <label class="inline-check">
        <input bind:checked={memoryPinned} type="checkbox" />
        {strings.memories.pin}
      </label>
      <label class="inline-check">
        <input bind:checked={memoryActive} type="checkbox" />
        Active
      </label>
    </div>

    <button type="submit">{editingMemoryId ? 'Save memory' : strings.memories.add}</button>
  </form>
  </Modal>

  {#if memories.length === 0}
    <EmptyState
      description="Create explicit memories to inject known facts and preferences into chat runs."
      title={strings.memories.noMemories}
    />
  {:else}
    <div class="memory-grid">
      {#each memories as memory (memory.id)}
        <article class="memory-card">
          <header>
            <div>
              <strong>{memory.title}</strong>
              <span>{memory.source} source / used {memory.use_count}</span>
            </div>
            <StatusPill status={memory.active ? 'active' : 'disabled'} tone={memory.active ? 'success' : 'neutral'} />
          </header>
          <p>{memory.content}</p>
          <div class="cluster">
            <StatusPill status={memory.scope} tone="accent" />
            <StatusPill status={`importance ${memory.importance}`} tone="warning" />
            {#if memory.pinned}
              <StatusPill status="pinned" tone="success" />
            {/if}
          </div>
          {#if memory.tags.length}
            <div class="tag-row">
              {#each memory.tags as tag (tag)}
                <span>{tag}</span>
              {/each}
            </div>
          {/if}
          <div class="message-actions">
            <button on:click={() => onEdit(memory)} type="button">Edit</button>
            <button on:click={() => onDelete(memory.id)} type="button">{strings.memories.delete}</button>
          </div>
        </article>
      {/each}
    </div>
  {/if}
</section>
