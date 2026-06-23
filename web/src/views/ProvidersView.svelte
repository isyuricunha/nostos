<script lang="ts">
  import EmptyState from '../components/common/EmptyState.svelte';
  import Modal from '../components/common/Modal.svelte';
  import StatusPill from '../components/common/StatusPill.svelte';
  import ModelPicker from '../components/models/ModelPicker.svelte';
  import type { Provider, ProviderModel } from '../lib/types';
  import { strings } from '../strings';

  export let providers: Provider[] = [];
  export let providerModels: ProviderModel[] = [];
  export let editingProviderId = '';
  export let providerName = '';
  export let providerBaseUrl = '';
  export let providerApiKey = '';
  export let providerApiKeyEnvRef = '';
  export let providerOrganization = '';
  export let providerProject = '';
  export let providerCustomHeaders = '';
  export let providerDefaultModel = '';
  export let providerFallbackModel = '';
  export let providerTimeoutMS = 60000;
  export let providerEnabled = true;
  export let submitting = false;
  export let onSubmit: () => void | Promise<void>;
  export let onCancelEdit: () => void;
  export let onRefresh: () => void | Promise<void>;
  export let onEdit: (provider: Provider) => void;
  export let onTest: (providerId: string) => void | Promise<void>;
  export let onRefreshModels: (providerId: string) => void | Promise<void>;
  export let onDelete: (providerId: string) => void | Promise<void>;

  function providerTone(provider: Provider): 'success' | 'warning' | 'danger' | 'neutral' {
    if (!provider.enabled) return 'neutral';
    if (provider.health_status === 'healthy') return 'success';
    if (provider.health_status === 'unhealthy') return 'danger';
    return 'warning';
  }

  $: editingProviderModels = editingProviderId
    ? providerModels.filter((model) => model.provider_id === editingProviderId)
    : [];
  $: editingProviderList = editingProviderId ? providers.filter((provider) => provider.id === editingProviderId) : [];

  let formOpen = false;

  $: if (editingProviderId) {
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
      <p class="eyebrow">Models and health</p>
      <h2>{strings.providers.title}</h2>
    </div>
    <div class="cluster">
      <button on:click={onRefresh} type="button">Refresh</button>
      <button on:click={openCreate} type="button">New provider</button>
    </div>
  </div>

  <Modal open={formOpen} title={editingProviderId ? 'Edit provider' : strings.providers.add} onClose={closeForm}>
  <form class="form-grid" on:submit|preventDefault={submitForm}>
    <div class="form-section">
      <h3>Connection</h3>
      <label>
        Name
        <input bind:value={providerName} required />
      </label>
      <label>
        Base URL
        <input bind:value={providerBaseUrl} placeholder="https://bifrost.example.com" required />
      </label>
      <label class="inline-check">
        <input bind:checked={providerEnabled} type="checkbox" />
        Enabled
      </label>
      <label>
        Request timeout, milliseconds
        <input bind:value={providerTimeoutMS} min="1000" max="600000" type="number" />
      </label>
    </div>

    <div class="form-section">
      <h3>Secrets</h3>
      <p>{strings.providers.apiKeyHelp}</p>
      <label>
        API key
        <input bind:value={providerApiKey} autocomplete="off" placeholder="Write-only replacement" type="password" />
      </label>
      <label>
        Environment reference
        <input bind:value={providerApiKeyEnvRef} placeholder="env:BIFROST_API_KEY" />
      </label>
      <label>
        Organization header
        <input bind:value={providerOrganization} />
      </label>
      <label>
        Project header
        <input bind:value={providerProject} />
      </label>
      <label>
        Custom headers JSON
        <textarea bind:value={providerCustomHeaders} placeholder="JSON object with string header values"></textarea>
      </label>
    </div>

    <div class="form-section">
      <h3>Model defaults</h3>
      {#if editingProviderId}
        <ModelPicker
          bind:selectedModelId={providerDefaultModel}
          selectedProviderId={editingProviderId}
          fixedProviderId={editingProviderId}
          label="Provider default model"
          models={editingProviderModels}
          providers={editingProviderList}
          role="chat"
        />
        <ModelPicker
          bind:selectedModelId={providerFallbackModel}
          selectedProviderId={editingProviderId}
          fixedProviderId={editingProviderId}
          label="Provider fallback model"
          models={editingProviderModels}
          providers={editingProviderList}
          role="chat"
        />
      {:else}
        <p>Enter manual model IDs now. After saving and refreshing the catalog, edit the provider to choose from cached models.</p>
        <label>
          Default model
          <input bind:value={providerDefaultModel} placeholder="gpt-4.1-mini" />
        </label>
        <label>
          Fallback model
          <input bind:value={providerFallbackModel} />
        </label>
      {/if}
    </div>

    <button disabled={submitting} type="submit">
      {editingProviderId ? 'Save provider' : strings.providers.add}
    </button>
  </form>
  </Modal>

    {#if providers.length === 0}
      <EmptyState
        description="Add a Bifrost or OpenAI-compatible provider before starting chat."
        title={strings.providers.noProviders}
      />
    {:else}
      <div class="table-list provider-cards">
        {#each providers as provider (provider.id)}
          <article>
            <div>
              <div class="split">
                <strong>{provider.name}</strong>
                <StatusPill status={provider.enabled ? provider.health_status : 'disabled'} tone={providerTone(provider)} />
              </div>
              <span>{provider.base_url}</span>
              {#if provider.default_model || provider.fallback_model}
                <span>
                  {provider.default_model ? `default ${provider.default_model}` : ''}
                  {provider.fallback_model ? ` / fallback ${provider.fallback_model}` : ''}
                </span>
              {/if}
              {#if provider.api_key_env_ref}
                <span>secret source {provider.api_key_env_ref}</span>
              {/if}
              {#if provider.last_health_check_at}
                <span>
                  Checked {new Date(provider.last_health_check_at).toLocaleString()}
                  {provider.health_latency_ms ? ` / ${provider.health_latency_ms} ms` : ''}
                </span>
              {/if}
              {#if provider.last_error}
                <span class="danger-text">{provider.last_error}</span>
              {/if}
            </div>
            <div>
              <button on:click={() => onEdit(provider)} type="button">Edit</button>
              <button on:click={() => onTest(provider.id)} type="button">{strings.providers.test}</button>
              <button on:click={() => onRefreshModels(provider.id)} type="button">{strings.providers.refreshModels}</button>
              <button on:click={() => onDelete(provider.id)} type="button">Delete</button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
</section>
