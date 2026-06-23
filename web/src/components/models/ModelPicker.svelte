<script lang="ts">
  import { onMount } from 'svelte';
  import Icon from '../common/Icon.svelte';
  import type { Provider, ProviderModel } from '../../lib/types';

  export let label = 'Model';
  export let providers: Provider[] = [];
  export let models: ProviderModel[] = [];
  export let selectedProviderId = '';
  export let selectedModelId = '';
  export let role: 'chat' | 'utility' | 'vision' | '' = '';
  export let allowManual = true;
  export let compact = false;
  export let fixedProviderId = '';

  const PAGE_SIZE = 80;

  let open = false;
  let query = '';
  let manualModel = '';
  let visibleLimit = PAGE_SIZE;
  let activeIndex = 0;
  let searchInput: HTMLInputElement;
  let root: HTMLDivElement;
  let previousQuery = '';

  $: providerById = Object.fromEntries(providers.map((provider) => [provider.id, provider]));
  $: selectedProvider = providerById[selectedProviderId];
  $: selectedModel = models.find((model) => model.provider_id === selectedProviderId && model.model_id === selectedModelId);
  $: selectedLabel = selectedModel
    ? `${selectedModel.display_name || selectedModel.model_id}`
    : selectedModelId
      ? `${selectedProvider?.name ?? 'Manual'} / ${selectedModelId}`
      : 'Select model';
  $: normalizedQuery = query.trim().toLowerCase();
  $: scopedModels = fixedProviderId ? models.filter((model) => model.provider_id === fixedProviderId) : models;
  $: scopedProviders = fixedProviderId ? providers.filter((provider) => provider.id === fixedProviderId) : providers;
  $: filteredModels = scopedModels
    .filter((model) => isRoleCompatible(model, role))
    .filter((model) => {
      if (!normalizedQuery) return true;
      const provider = providerById[model.provider_id];
      return [model.model_id, model.display_name ?? '', provider?.name ?? '', provider?.health_status ?? '', ...(model.capabilities ?? [])]
        .join(' ')
        .toLowerCase()
        .includes(normalizedQuery);
    });
  $: renderedModels = filteredModels.slice(0, visibleLimit);
  $: groupedModels = groupModels(renderedModels);
  $: if (query !== previousQuery) {
    previousQuery = query;
    visibleLimit = PAGE_SIZE;
    activeIndex = 0;
  }

  onMount(() => {
    function handleDocumentClick(event: MouseEvent): void {
      if (!open || !root) return;
      const target = event.target;
      if (target instanceof Node && !root.contains(target)) {
        open = false;
      }
    }
    document.addEventListener('click', handleDocumentClick);
    return () => document.removeEventListener('click', handleDocumentClick);
  });

  function toggleOpen(): void {
    open = !open;
    if (open) {
      visibleLimit = PAGE_SIZE;
      activeIndex = Math.max(0, renderedModels.findIndex((model) => model.provider_id === selectedProviderId && model.model_id === selectedModelId));
      setTimeout(() => searchInput?.focus(), 0);
    }
  }

  function selectModel(model: ProviderModel): void {
    selectedProviderId = model.provider_id;
    selectedModelId = model.model_id;
    open = false;
  }

  function useManualModel(): void {
    const trimmed = manualModel.trim();
    if (!trimmed) return;
    selectedModelId = trimmed;
    if (!selectedProviderId) {
      selectedProviderId = fixedProviderId || scopedProviders[0]?.id || '';
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

  function modelStatus(model: ProviderModel): string {
    const provider = providerById[model.provider_id];
    if (model.available === false || model.enabled === false) return 'offline';
    return provider?.health_status || 'unknown';
  }

  async function copyModelId(modelId: string): Promise<void> {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(modelId);
    }
  }

  function handlePickerKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      open = false;
      event.stopPropagation();
      return;
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      activeIndex = Math.min(activeIndex + 1, renderedModels.length - 1);
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      activeIndex = Math.max(activeIndex - 1, 0);
      return;
    }
    if (event.key === 'Enter' && renderedModels[activeIndex]) {
      event.preventDefault();
      selectModel(renderedModels[activeIndex]);
    }
  }
</script>

<div bind:this={root} class:compact class="model-picker">
  <span class="model-picker-label">{label}</span>
  <button
    aria-expanded={open}
    aria-label={`${label}: ${selectedLabel}`}
    class="model-picker-trigger"
    on:click={toggleOpen}
    type="button"
  >
    <Icon name="model" size={13} />
    <span>{selectedLabel}</span>
    <small>{selectedProvider?.name ?? 'Manual'}</small>
    <Icon name="chevron-down" size={11} />
  </button>
  {#if open}
    <div class="model-picker-panel" on:keydown={handlePickerKeydown} role="dialog" aria-label={`${label} picker`} tabindex="-1">
      <div class="model-picker-search-row">
        <Icon name="search" size={13} />
        <input bind:this={searchInput} bind:value={query} placeholder="Search provider, model, capability" />
      </div>
      <div class="model-picker-count">
        <span>{filteredModels.length} matches</span>
        {#if renderedModels.length < filteredModels.length}
          <span>showing {renderedModels.length}</span>
        {/if}
      </div>
      <div class="model-picker-list" role="listbox" aria-label={`${label} results`}>
        {#if groupedModels.length === 0}
          <p>No cached models match this search.</p>
        {:else}
          {#each groupedModels as group (group.provider?.id ?? 'unknown')}
            <section>
              <header>
                <strong>{group.provider?.name ?? 'Unknown provider'}</strong>
                <small>{group.provider?.health_status ?? 'unknown'} · {group.models.length}</small>
              </header>
              {#each group.models as model (model.id)}
                <div
                  aria-selected={model.provider_id === selectedProviderId && model.model_id === selectedModelId}
                  class:keyboard-active={renderedModels[activeIndex]?.id === model.id}
                  class:offline={model.available === false || model.enabled === false}
                  class:selected={model.provider_id === selectedProviderId && model.model_id === selectedModelId}
                  class="model-option"
                  on:click={() => selectModel(model)}
                  on:keydown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      selectModel(model);
                    }
                  }}
                  role="option"
                  tabindex="0"
                  title={model.model_id}
                >
                  <span class="model-name">{model.display_name || model.model_id}</span>
                  <code>{model.model_id}</code>
                  <small>
                    {modelStatus(model)}
                    {model.manually_added ? ' / manual' : ' / cached'}
                    {#if model.capabilities?.length}
                      / {model.capabilities.join(', ')}
                    {/if}
                  </small>
                  <span class="model-row-actions">
                    <button
                      aria-label={`Copy ${model.model_id}`}
                      on:click|stopPropagation={() => copyModelId(model.model_id)}
                      type="button"
                    >
                      <Icon name="copy" size={12} />
                    </button>
                  </span>
                </div>
              {/each}
            </section>
          {/each}
        {/if}
      </div>
      {#if renderedModels.length < filteredModels.length}
        <button class="model-load-more" on:click={() => (visibleLimit += PAGE_SIZE)} type="button">
          Load more ({filteredModels.length - renderedModels.length} remaining)
        </button>
      {/if}
      {#if allowManual}
        <form class="manual-model" on:submit|preventDefault={useManualModel}>
          <input bind:value={manualModel} placeholder="Type an unlisted full model ID" />
          <button type="submit">Use ID</button>
        </form>
      {/if}
    </div>
  {/if}
</div>
