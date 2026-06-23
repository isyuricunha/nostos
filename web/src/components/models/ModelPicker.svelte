<script lang="ts">
  import type { Provider, ProviderModel } from '../../lib/types';

  export let label = 'Model';
  export let providers: Provider[] = [];
  export let models: ProviderModel[] = [];
  export let selectedProviderId = '';
  export let selectedModelId = '';
  export let role: 'chat' | 'utility' | 'vision' | '' = '';
  export let allowManual = true;
  export let compact = false;

  let open = false;
  let query = '';
  let manualModel = '';

  $: providerById = Object.fromEntries(providers.map((provider) => [provider.id, provider]));
  $: selectedProvider = providerById[selectedProviderId];
  $: selectedModel = models.find((model) => model.provider_id === selectedProviderId && model.model_id === selectedModelId);
  $: selectedLabel = selectedModel
    ? `${selectedModel.display_name || selectedModel.model_id}`
    : selectedModelId
      ? `${selectedProvider?.name ?? 'Manual'} / ${selectedModelId}`
      : 'Select model';
  $: normalizedQuery = query.trim().toLowerCase();
  $: filteredModels = models
    .filter((model) => isRoleCompatible(model, role))
    .filter((model) => {
      if (!normalizedQuery) return true;
      const provider = providerById[model.provider_id];
      return [model.model_id, model.display_name ?? '', provider?.name ?? '', ...(model.capabilities ?? [])]
        .join(' ')
        .toLowerCase()
        .includes(normalizedQuery);
    })
    .slice(0, 120);
  $: groupedModels = groupModels(filteredModels);

  function selectModel(model: ProviderModel): void {
    selectedProviderId = model.provider_id;
    selectedModelId = model.model_id;
    open = false;
  }

  function useManualModel(): void {
    const trimmed = manualModel.trim();
    if (!trimmed) return;
    selectedModelId = trimmed;
    if (!selectedProviderId && providers.length > 0) {
      selectedProviderId = providers[0].id;
    }
    open = false;
  }

  function groupModels(items: ProviderModel[]): Array<{ provider: Provider | undefined; models: ProviderModel[] }> {
    const groups: Record<string, ProviderModel[]> = {};
    for (const model of items) {
      groups[model.provider_id] = [...(groups[model.provider_id] ?? []), model];
    }
    return Object.entries(groups).map(([providerId, providerModels]) => ({
      provider: providerById[providerId],
      models: providerModels
    }));
  }

  function isRoleCompatible(model: ProviderModel, targetRole: string): boolean {
    if (!targetRole || targetRole === 'chat' || targetRole === 'utility') {
      return !(model.capabilities ?? []).some((capability) => capability === 'embedding' || capability === 'audio');
    }
    if (targetRole === 'vision') {
      return (model.capabilities ?? []).includes('vision') || (model.capabilities ?? []).length === 0;
    }
    return true;
  }
</script>

<div class:compact class="model-picker">
  <span class="model-picker-label">{label}</span>
  <button aria-expanded={open} class="model-picker-trigger" on:click={() => (open = !open)} type="button">
    <span>{selectedLabel}</span>
    {#if selectedProvider}
      <small>{selectedProvider.name}</small>
    {/if}
  </button>
  {#if open}
    <div class="model-picker-panel" role="dialog" aria-label={`${label} picker`}>
      <input bind:value={query} placeholder="Search provider, model, capability" />
      <div class="model-picker-list" role="listbox">
        {#if groupedModels.length === 0}
          <p>No cached models match this search.</p>
        {:else}
          {#each groupedModels as group (group.provider?.id ?? 'unknown')}
            <section>
              <header>
                <strong>{group.provider?.name ?? 'Unknown provider'}</strong>
                <small>{group.provider?.health_status ?? 'unknown'}</small>
              </header>
              {#each group.models as model (model.id)}
                <button
                  aria-selected={model.provider_id === selectedProviderId && model.model_id === selectedModelId}
                  class:offline={model.available === false || model.enabled === false}
                  class:selected={model.provider_id === selectedProviderId && model.model_id === selectedModelId}
                  on:click={() => selectModel(model)}
                  role="option"
                  type="button"
                >
                  <span>{model.display_name || model.model_id}</span>
                  <code>{model.model_id}</code>
                  <small>
                    {model.manually_added ? 'manual' : 'cached'}
                    {model.available === false ? ' / unavailable' : ''}
                    {#if model.capabilities?.length}
                      / {model.capabilities.join(', ')}
                    {/if}
                  </small>
                </button>
              {/each}
            </section>
          {/each}
        {/if}
      </div>
      {#if allowManual}
        <form class="manual-model" on:submit|preventDefault={useManualModel}>
          <input bind:value={manualModel} placeholder="Type an unlisted full model ID" />
          <button type="submit">Use ID</button>
        </form>
      {/if}
    </div>
  {/if}
</div>

<style>
  .model-picker {
    position: relative;
    display: grid;
    gap: var(--space-2);
    min-width: min(100%, 280px);
  }

  .model-picker.compact {
    min-width: min(100%, 240px);
  }

  .model-picker-label {
    color: var(--color-subtle);
    font-size: var(--font-xs);
    font-weight: 720;
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .model-picker-trigger {
    display: grid;
    min-width: 0;
    gap: 2px;
    text-align: left;
  }

  .model-picker-trigger span,
  .model-picker-trigger small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .model-picker-trigger small {
    color: var(--color-subtle);
    font-size: var(--font-xs);
  }

  .model-picker-panel {
    position: absolute;
    z-index: 20;
    top: calc(100% + var(--space-2));
    right: 0;
    display: grid;
    width: min(680px, calc(100vw - 32px));
    max-height: min(620px, calc(100vh - 180px));
    gap: var(--space-3);
    border: 1px solid var(--color-border-muted);
    border-radius: var(--radius-lg);
    background: var(--color-panel-elevated);
    box-shadow: var(--shadow-soft);
    padding: var(--space-3);
  }

  .model-picker-list {
    display: grid;
    gap: var(--space-3);
    overflow: auto;
  }

  .model-picker-list section {
    display: grid;
    gap: var(--space-2);
  }

  .model-picker-list header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    color: var(--color-muted);
    font-size: var(--font-sm);
  }

  .model-picker-list button {
    display: grid;
    gap: 2px;
    border-radius: var(--radius-md);
    padding: 10px;
    text-align: left;
    transform: none;
  }

  .model-picker-list button.selected {
    border-color: var(--color-border-strong);
    background: var(--color-accent-muted);
  }

  .model-picker-list button.offline {
    opacity: 0.62;
  }

  .model-picker-list span,
  .model-picker-list code,
  .model-picker-list small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .model-picker-list code {
    color: var(--color-text-soft);
    font-size: var(--font-xs);
  }

  .model-picker-list small {
    color: var(--color-subtle);
    font-size: var(--font-xs);
  }

  .manual-model {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: var(--space-2);
  }

  @media (max-width: 720px) {
    .model-picker-panel {
      position: fixed;
      inset: auto var(--space-3) var(--space-3) var(--space-3);
      width: auto;
    }

    .manual-model {
      grid-template-columns: 1fr;
    }
  }
</style>
