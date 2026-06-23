<script lang="ts">
  import EmptyState from '../components/common/EmptyState.svelte';
  import Modal from '../components/common/Modal.svelte';
  import StatusPill from '../components/common/StatusPill.svelte';
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

  let formOpen = false;

  $: if (editingAgentId) {
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
      <p class="eyebrow">Assistants</p>
      <h2>Agents</h2>
    </div>
    <div class="cluster">
      <button on:click={onRefresh} type="button">Refresh</button>
      <button on:click={openCreate} type="button">New agent</button>
    </div>
  </div>

  <Modal open={formOpen} title={editingAgentId ? 'Edit agent' : strings.agents.add} onClose={closeForm}>
  <form class="form-grid" on:submit|preventDefault={submitForm}>
    <div class="form-section">
      <h3>Basic identity</h3>
      <label>
        Name
        <input bind:value={agentName} required />
      </label>
      <label>
        Description
        <input bind:value={agentDescription} />
      </label>
      <label>
        Avatar or icon
        <input bind:value={agentAvatar} />
      </label>
      <label>
        System prompt
        <textarea bind:value={agentPrompt} required></textarea>
      </label>
      <label class="inline-check">
        <input bind:checked={agentActive} type="checkbox" />
        Active
      </label>
    </div>

    <div class="form-section">
      <h3>Provider and model</h3>
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
    </div>

    <div class="form-section">
      <h3>Memory and tools</h3>
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
        Default tool permission
        <select bind:value={agentToolPermissionDefault}>
          <option value="deny">deny</option>
          <option value="ask">ask</option>
          <option value="allow">allow</option>
        </select>
      </label>
    </div>

    <div class="form-section">
      <h3>Runtime parameters</h3>
      <label>
        Temperature
        <input bind:value={agentTemperature} min="0" max="2" step="0.1" type="number" />
      </label>
      <label>
        Maximum tool iterations
        <input bind:value={agentMaxToolIterations} min="1" max="32" type="number" />
      </label>
    </div>

    <button type="submit">{editingAgentId ? 'Save agent' : strings.agents.add}</button>
  </form>
  </Modal>

  {#if agents.length === 0}
    <EmptyState description="Create a focused assistant profile for chat and scheduled work." title={strings.agents.noAgents} />
  {:else}
    <div class="table-list agent-cards">
      {#each agents as agent (agent.id)}
        <article>
          <div>
            <div class="split">
              <strong>{agent.name}</strong>
              <StatusPill status={agent.active ? 'active' : 'disabled'} tone={agent.active ? 'success' : 'neutral'} />
            </div>
            <span>{agent.description || 'No description'}</span>
            <span>{agent.memory_access_mode} memory / tools {agent.tool_permission_default}</span>
            <span>max iterations {agent.max_tool_iterations} / temperature {agent.temperature}</span>
            {#if agent.default_provider_id || agent.default_model || agent.fallback_model}
              <span>
                {agent.default_provider_id ? 'provider configured' : ''}
                {agent.default_model ? ` / default ${agent.default_model}` : ''}
                {agent.fallback_model ? ` / fallback ${agent.fallback_model}` : ''}
              </span>
            {/if}
          </div>
          <div>
            <button on:click={() => onEdit(agent)} type="button">Edit</button>
            <button on:click={() => onDuplicate(agent.id)} type="button">{strings.agents.duplicate}</button>
            <button on:click={() => onDelete(agent.id)} type="button">Delete</button>
          </div>
        </article>
      {/each}
    </div>
  {/if}
</section>
