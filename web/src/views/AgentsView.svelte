<script lang="ts">
  import Icon from '../components/common/Icon.svelte';
  import ModelPicker from '../components/models/ModelPicker.svelte';
  import type { Agent, Provider, ProviderModel } from '../lib/types';
  import { strings } from '../strings';

  export let agents: Agent[] = [];
  export let providers: Provider[] = [];
  export let providerModels: ProviderModel[] = [];
  export let editingAgentId = '';
  export let agentName = '';
  export let agentDescription = '';
  export let agentAvatar = 'sparkles';
  export let agentPrompt = '';
  export let agentDefaultProviderId = '';
  export let agentDefaultModel = '';
  export let agentFallbackModel = '';
  export let agentTemperature = 0.7;
  export let agentMaxToolIterations = 8;
  export let agentMemoryMode = 'pinned_only';
  export let agentToolPermissionDefault = 'ask';
  export let agentActive = true;
  export let onSubmit: () => void | Promise<void>;
  export let onCancelEdit: () => void;
  export let onRefresh: () => void | Promise<void>;
  export let onEdit: (agent: Agent) => void;
  export let onDuplicate: (agentId: string) => void | Promise<void>;
  export let onDelete: (agentId: string) => void | Promise<void>;
  export let onToggleActive: (agent: Agent) => void | Promise<void>;
  export let onTest: (agent: Agent) => void;

  let query = '';
  let stateFilter: 'all' | 'active' | 'disabled' = 'all';
  let formOpen = false;
  let openMenuId = '';

  $: if (editingAgentId) {
    formOpen = true;
  }
  $: filteredAgents = agents.filter((agent) => {
    const matchesState =
      stateFilter === 'all' ||
      (stateFilter === 'active' && agent.active) ||
      (stateFilter === 'disabled' && !agent.active);
    const haystack = `${agent.name} ${agent.description} ${agent.default_model ?? ''} ${agent.memory_access_mode}`.toLowerCase();
    return matchesState && haystack.includes(query.trim().toLowerCase());
  });

  function openCreate(): void {
    onCancelEdit();
    formOpen = true;
  }

  function closeForm(): void {
    onCancelEdit();
    formOpen = false;
  }

  function startEdit(agent: Agent): void {
    onEdit(agent);
    formOpen = true;
    openMenuId = '';
  }

  async function submitForm(): Promise<void> {
    await onSubmit();
    formOpen = false;
  }
</script>

<div class="workspace-module-grid" class:editor-open={formOpen}>
  <section class="window-list-pane" aria-label="Agents">
    <header class="window-panel-toolbar">
      <div>
        <strong>Agents</strong>
        <span>{agents.length} profiles</span>
      </div>
      <div class="window-toolbar-actions">
        <button aria-label="Refresh agents" on:click={onRefresh} type="button">
          <Icon name="refresh" size={13} />
        </button>
        <button on:click={openCreate} type="button">
          <Icon name="plus" size={13} /> New agent
        </button>
      </div>
    </header>

    <div class="window-filter-row">
      <label class="window-search">
        <Icon name="search" size={13} />
        <input bind:value={query} placeholder="Search agents" />
      </label>
      <select aria-label="Agent state" bind:value={stateFilter}>
        <option value="all">All</option>
        <option value="active">Active</option>
        <option value="disabled">Disabled</option>
      </select>
    </div>

    {#if filteredAgents.length === 0}
      <p class="window-empty">{strings.agents.noAgents}</p>
    {:else}
      <div class="dense-row-list">
        {#each filteredAgents as agent (agent.id)}
          <article class="agent-row">
            <span class="row-icon"><Icon name="agent" size={15} /></span>
            <span class={`status-dot ${agent.active ? 'healthy' : 'disabled'}`}></span>
            <div>
              <strong>{agent.name}</strong>
              <span>{agent.description || 'No description'}</span>
              <small>
                {agent.default_model || 'workspace default'} · memory {agent.memory_access_mode} · tools {agent.tool_permission_default}
              </small>
            </div>
            <div class="row-actions compact">
              <button aria-label={`Agent menu for ${agent.name}`} on:click={() => (openMenuId = openMenuId === agent.id ? '' : agent.id)} type="button">
                <Icon name="kebab" size={14} />
              </button>
              {#if openMenuId === agent.id}
                <div class="row-menu row-menu-right" role="menu">
                  <button on:click={() => startEdit(agent)} type="button"><Icon name="edit" size={13} /> Edit</button>
                  <button on:click={() => { openMenuId = ''; onDuplicate(agent.id); }} type="button">
                    <Icon name="copy" size={13} /> Duplicate
                  </button>
                  <button on:click={() => { openMenuId = ''; onToggleActive(agent); }} type="button">
                    <Icon name={agent.active ? 'minus' : 'check'} size={13} /> {agent.active ? 'Disable' : 'Enable'}
                  </button>
                  <button on:click={() => { openMenuId = ''; onTest(agent); }} type="button">
                    <Icon name="chat" size={13} /> Test
                  </button>
                  <button class="danger" on:click={() => { openMenuId = ''; onDelete(agent.id); }} type="button">
                    <Icon name="trash" size={13} /> Delete
                  </button>
                </div>
              {/if}
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>

  {#if formOpen}
    <aside class="window-editor-panel" aria-label={editingAgentId ? 'Edit agent' : 'Create agent'}>
      <header>
        <strong>{editingAgentId ? 'Edit agent' : strings.agents.add}</strong>
        <button aria-label="Close agent editor" on:click={closeForm} type="button"><Icon name="close" size={13} /></button>
      </header>
      <form class="compact-editor-form" on:submit|preventDefault={submitForm}>
        <label>
          Name
          <input bind:value={agentName} required />
        </label>
        <label>
          Description
          <input bind:value={agentDescription} />
        </label>
        <label>
          Icon name
          <input bind:value={agentAvatar} />
        </label>
        <label>
          System prompt
          <textarea bind:value={agentPrompt} required></textarea>
        </label>
        <label class="toggle-line">
          <input bind:checked={agentActive} type="checkbox" />
          Active
        </label>
        <ModelPicker
          bind:selectedModelId={agentDefaultModel}
          bind:selectedProviderId={agentDefaultProviderId}
          label="Agent default model"
          models={providerModels}
          {providers}
          role="chat"
        />
        <ModelPicker
          bind:selectedModelId={agentFallbackModel}
          bind:selectedProviderId={agentDefaultProviderId}
          fixedProviderId={agentDefaultProviderId}
          label="Agent fallback model"
          models={providerModels}
          {providers}
          role="chat"
        />
        <div class="two-col">
          <label>
            Memory mode
            <select bind:value={agentMemoryMode}>
              <option value="none">none</option>
              <option value="pinned_only">pinned_only</option>
              <option value="relevant">relevant</option>
              <option value="all">all</option>
            </select>
          </label>
          <label>
            Tool policy
            <select bind:value={agentToolPermissionDefault}>
              <option value="deny">deny</option>
              <option value="ask">ask</option>
              <option value="allow">allow</option>
            </select>
          </label>
        </div>
        <div class="two-col">
          <label>
            Temperature
            <input bind:value={agentTemperature} min="0" max="2" step="0.1" type="number" />
          </label>
          <label>
            Tool iterations
            <input bind:value={agentMaxToolIterations} min="1" max="32" type="number" />
          </label>
        </div>
        <div class="editor-actions">
          <button type="submit">{editingAgentId ? 'Save agent' : strings.agents.add}</button>
          <button on:click={closeForm} type="button">Cancel</button>
        </div>
      </form>
    </aside>
  {/if}
</div>
