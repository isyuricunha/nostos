<script lang="ts">
  import { onMount, tick } from 'svelte';
  import Icon from '../components/common/Icon.svelte';
  import ModelPicker from '../components/models/ModelPicker.svelte';
  import type { FeedbackStats, ModelRoleDraft, Provider, ProviderModel, ReadyStatus, ReplyPreset, Session, User } from '../lib/types';
  import { strings } from '../strings';

  export let user: User;
  export let sessions: Session[] = [];
  export let status: ReadyStatus | null = null;
  export let feedbackStats: FeedbackStats = { positive: 0, negative: 0 };
  export let providers: Provider[] = [];
  export let providerModels: ProviderModel[] = [];
  export let actionStates: Record<string, string> = {};
  export let chatRoleEntries: ModelRoleDraft[] = [];
  export let utilityRoleEntries: ModelRoleDraft[] = [];
  export let visionRoleEntries: ModelRoleDraft[] = [];
  export let replyPresets: ReplyPreset[] = [];
  export let replyPresetName = '';
  export let replyPresetDescription = '';
  export let replyPresetInstruction = '';
  export let submitting = false;
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
  export let editingProviderId = '';
  export let onRefreshSessions: () => void | Promise<void>;
  export let onRevokeSession: (sessionId: string) => void | Promise<void>;
  export let onRefreshDiagnostics: () => void | Promise<void>;
  export let onRefreshFeedbackStats: () => void | Promise<void>;
  export let onSaveModelRole: (role: 'chat' | 'utility' | 'vision', entries: ModelRoleDraft[]) => void | Promise<void>;
  export let onCreateReplyPreset: () => void | Promise<void>;
  export let onToggleReplyPreset: (preset: ReplyPreset) => void | Promise<void>;
  export let onResetReplyPresets: () => void | Promise<void>;
  export let onSubmitProvider: () => void | Promise<void>;
  export let onCancelProviderEdit: () => void;
  export let onRefreshProviders: () => void | Promise<void>;
  export let onEditProvider: (provider: Provider) => void;
  export let onTestProvider: (providerId: string) => void | Promise<void>;
  export let onRefreshProviderModels: (providerId: string) => void | Promise<void>;
  export let onDeleteProvider: (providerId: string) => void | Promise<void>;
  export let onToggleProviderEnabled: (provider: Provider) => void | Promise<void>;

  type Section =
    | 'add-provider'
    | 'providers'
    | 'defaults'
    | 'appearance'
    | 'shortcuts'
    | 'account'
    | 'sessions'
    | 'system'
    | 'agent-tools';

  let activeSection: Section = 'defaults';
  let endpointType: 'local' | 'api' = 'api';
  let patternEnabled = true;
  let density: 'compact' | 'comfortable' = 'compact';
  let uiScale = 100;
  let accentColor = '#e06c75';
  let messageWidth = 820;
  let reducedMotion = false;
  let savedRoleSnapshots: Record<'chat' | 'utility' | 'vision', string> = {
    chat: '',
    utility: '',
    vision: ''
  };
  let roleSnapshotsInitialized = false;

  $: editingProviderModels = editingProviderId
    ? providerModels.filter((model) => model.provider_id === editingProviderId)
    : [];
  $: editingProviderList = editingProviderId ? providers.filter((provider) => provider.id === editingProviderId) : [];
  $: if (!roleSnapshotsInitialized) {
    savedRoleSnapshots = {
      chat: serializeRoleEntries(chatRoleEntries),
      utility: serializeRoleEntries(utilityRoleEntries),
      vision: serializeRoleEntries(visionRoleEntries)
    };
    roleSnapshotsInitialized = true;
  }

  onMount(() => {
    loadAppearance();
  });

  function loadAppearance(): void {
    try {
      const raw = localStorage.getItem('nostos-appearance');
      if (raw) {
        const saved = JSON.parse(raw) as Partial<{
          patternEnabled: boolean;
          density: 'compact' | 'comfortable';
          uiScale: number;
          accentColor: string;
          messageWidth: number;
          reducedMotion: boolean;
        }>;
        patternEnabled = saved.patternEnabled ?? patternEnabled;
        density = saved.density ?? density;
        uiScale = saved.uiScale ?? uiScale;
        accentColor = saved.accentColor ?? accentColor;
        messageWidth = saved.messageWidth ?? messageWidth;
        reducedMotion = saved.reducedMotion ?? reducedMotion;
      }
    } catch {
      // Ignore invalid local preferences.
    }
    applyAppearance();
  }

  function applyAppearance(): void {
    const root = document.documentElement;
    root.dataset.pattern = patternEnabled ? 'on' : 'off';
    root.dataset.density = density;
    root.dataset.reducedMotion = reducedMotion ? 'on' : 'off';
    root.style.setProperty('--ui-scale', String(uiScale / 100));
    root.style.setProperty('--color-accent', accentColor);
    root.style.setProperty('--color-accent-strong', accentColor);
    root.style.setProperty('--chat-max-width', `${messageWidth}px`);
    localStorage.setItem(
      'nostos-appearance',
      JSON.stringify({ patternEnabled, density, uiScale, accentColor, messageWidth, reducedMotion })
    );
  }

  function providerModelsFor(providerId: string): ProviderModel[] {
    return providerModels.filter((model) => model.provider_id === providerId);
  }

  function sanitizedUrl(value: string): string {
    try {
      const url = new URL(value);
      const path = `${url.pathname}${url.search}${url.hash}`.replace(/\/$/, '');
      return `${url.protocol}//${url.host}${path}`;
    } catch {
      return value;
    }
  }

  function providerTone(provider: Provider): string {
    if (!provider.enabled) return 'disabled';
    if (provider.model_refresh_state === 'queued' || provider.model_refresh_state === 'running') return 'unknown';
    if (provider.health_status === 'healthy') return 'healthy';
    if (provider.health_status === 'unhealthy') return 'unhealthy';
    return 'unknown';
  }

  function serializeRoleEntries(entries: ModelRoleDraft[]): string {
    return entries.map((entry) => `${entry.provider_id}:${entry.model_id}`).join('|');
  }

  function roleDirty(role: 'chat' | 'utility' | 'vision', entries: ModelRoleDraft[]): boolean {
    return savedRoleSnapshots[role] !== '' && savedRoleSnapshots[role] !== serializeRoleEntries(entries);
  }

  async function saveRole(role: 'chat' | 'utility' | 'vision', entries: ModelRoleDraft[]): Promise<void> {
    await onSaveModelRole(role, entries);
    await tick();
    if (stateFor(`model-role:${role}`) !== 'failed') {
      savedRoleSnapshots = { ...savedRoleSnapshots, [role]: serializeRoleEntries(entries) };
    }
  }

  function stateFor(key: string): string {
    return actionStates[key] ?? '';
  }

  function providerRowState(provider: Provider): string {
    return (
      stateFor(`provider:${provider.id}:test`) ||
      stateFor(`provider:${provider.id}:models`) ||
      stateFor(`provider:${provider.id}:toggle`) ||
      provider.model_refresh_state ||
      ''
    );
  }

  function modelRefreshLabel(provider: Provider): string {
    const state = stateFor(`provider:${provider.id}:models`) || provider.model_refresh_state || '';
    if (state === 'queued') return 'queued';
    if (state === 'running' || state === 'refreshing') return 'refreshing';
    if (state === 'succeeded') return 'succeeded';
    if (state === 'failed') return 'failed';
    return '';
  }

  function editProvider(provider: Provider): void {
    onEditProvider(provider);
    activeSection = 'add-provider';
  }

  async function submitProvider(): Promise<void> {
    await onSubmitProvider();
    activeSection = 'providers';
  }

  function useEndpointType(type: 'local' | 'api'): void {
    endpointType = type;
    if (type === 'local' && !providerBaseUrl) {
      providerBaseUrl = 'http://127.0.0.1:11434/v1';
    }
  }

  const shortcuts = [
    ['Send message', 'Enter'],
    ['New line', 'Shift Enter'],
    ['Close menu or window', 'Esc'],
    ['Search conversations', 'Ctrl K'],
    ['Toggle sidebar', 'Ctrl B']
  ];
</script>

<div class="settings-window-layout">
  <nav class="settings-window-nav" aria-label="Settings sections">
    <span>Models</span>
    <button
      aria-current={activeSection === 'add-provider' ? 'page' : undefined}
      data-active={activeSection === 'add-provider' ? 'true' : undefined}
      on:click={() => (activeSection = 'add-provider')}
      type="button"
    >
      <Icon name="plus" size={14} /> Add Provider
    </button>
    <button
      aria-current={activeSection === 'providers' ? 'page' : undefined}
      data-active={activeSection === 'providers' ? 'true' : undefined}
      on:click={() => (activeSection = 'providers')}
      type="button"
    >
      <Icon name="model" size={14} /> Providers
    </button>
    <button
      aria-current={activeSection === 'defaults' ? 'page' : undefined}
      data-active={activeSection === 'defaults' ? 'true' : undefined}
      on:click={() => (activeSection = 'defaults')}
      type="button"
    >
      <Icon name="spark" size={14} /> AI Defaults
    </button>

    <span>Workspace</span>
    <button
      aria-current={activeSection === 'appearance' ? 'page' : undefined}
      data-active={activeSection === 'appearance' ? 'true' : undefined}
      on:click={() => (activeSection = 'appearance')}
      type="button"
    >
      <Icon name="moon" size={14} /> Appearance
    </button>
    <button
      aria-current={activeSection === 'shortcuts' ? 'page' : undefined}
      data-active={activeSection === 'shortcuts' ? 'true' : undefined}
      on:click={() => (activeSection = 'shortcuts')}
      type="button"
    >
      <Icon name="grid" size={14} /> Shortcuts
    </button>
    <button
      aria-current={activeSection === 'account' ? 'page' : undefined}
      data-active={activeSection === 'account' ? 'true' : undefined}
      on:click={() => (activeSection = 'account')}
      type="button"
    >
      <Icon name="user" size={14} /> Account
    </button>

    <span>Administration</span>
    <button
      aria-current={activeSection === 'agent-tools' ? 'page' : undefined}
      data-active={activeSection === 'agent-tools' ? 'true' : undefined}
      on:click={() => (activeSection = 'agent-tools')}
      type="button"
    >
      <Icon name="agent" size={14} /> Agent Tools
    </button>
    <button
      aria-current={activeSection === 'sessions' ? 'page' : undefined}
      data-active={activeSection === 'sessions' ? 'true' : undefined}
      on:click={() => (activeSection = 'sessions')}
      type="button"
    >
      <Icon name="window" size={14} /> Sessions
    </button>
    <button
      aria-current={activeSection === 'system' ? 'page' : undefined}
      data-active={activeSection === 'system' ? 'true' : undefined}
      on:click={() => (activeSection = 'system')}
      type="button"
    >
      <Icon name="details" size={14} /> System
    </button>
  </nav>

  <section class="settings-window-panel">
    {#if activeSection === 'add-provider'}
      <div class="settings-panel-heading">
        <h3>{editingProviderId ? 'Edit provider' : 'Add provider'}</h3>
        {#if editingProviderId}
          <button on:click={onCancelProviderEdit} type="button">Cancel edit</button>
        {/if}
      </div>
      <div class="provider-type-grid">
        <button class:active={endpointType === 'local'} on:click={() => useEndpointType('local')} type="button">
          <Icon name="model" size={15} />
          <strong>Local endpoint</strong>
          <span>OpenAI-compatible local gateway.</span>
        </button>
        <button class:active={endpointType === 'api'} on:click={() => useEndpointType('api')} type="button">
          <Icon name="tools" size={15} />
          <strong>API endpoint</strong>
          <span>OpenAI-compatible hosted provider.</span>
        </button>
      </div>
      <form class="settings-form-grid" on:submit|preventDefault={submitProvider}>
        <label>
          Provider name
          <input bind:value={providerName} required />
        </label>
        <label>
          Base URL
          <input bind:value={providerBaseUrl} placeholder={endpointType === 'local' ? 'http://127.0.0.1:11434/v1' : 'https://api.example.com/v1'} required />
        </label>
        <label>
          API key
          <input bind:value={providerApiKey} autocomplete="off" placeholder="Write-only replacement" type="password" />
        </label>
        <label>
          Environment reference
          <input bind:value={providerApiKeyEnvRef} placeholder="env:NOSTOS_PROVIDER_KEY" />
        </label>
        <label>
          Request timeout, milliseconds
          <input bind:value={providerTimeoutMS} min="1000" max="600000" type="number" />
        </label>
        <label class="toggle-line">
          <input bind:checked={providerEnabled} type="checkbox" />
          Enabled
        </label>
        <label>
          Organization header
          <input bind:value={providerOrganization} />
        </label>
        <label>
          Project header
          <input bind:value={providerProject} />
        </label>
        <label class="span-2">
          Custom headers JSON
          <textarea bind:value={providerCustomHeaders} placeholder="JSON object with string header values"></textarea>
        </label>
        {#if editingProviderId}
          <div class="span-2 settings-model-pair">
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
          </div>
        {:else}
          <label>
            Default model
            <input bind:value={providerDefaultModel} placeholder="Full model ID" />
          </label>
          <label>
            Fallback model
            <input bind:value={providerFallbackModel} placeholder="Full model ID" />
          </label>
        {/if}
        <div class="span-2 provider-form-actions">
          {#if editingProviderId}
            <button disabled={submitting || stateFor(`provider:${editingProviderId}:test`) === 'testing'} on:click={() => onTestProvider(editingProviderId)} type="button">
              <Icon name="check" size={13} />
              {stateFor(`provider:${editingProviderId}:test`) === 'testing' ? 'Testing...' : 'Test'}
            </button>
          {/if}
          <button disabled={submitting || stateFor('provider-form') === 'saving'} type="submit">
            {stateFor('provider-form') === 'saving' ? 'Saving...' : editingProviderId ? 'Save provider' : strings.providers.add}
          </button>
        </div>
      </form>
    {:else if activeSection === 'providers'}
      <div class="settings-panel-heading">
        <h3>Providers</h3>
        <button on:click={onRefreshProviders} type="button"><Icon name="refresh" size={13} /> Refresh</button>
      </div>
      <div class="dense-row-list">
        {#if providers.length === 0}
          <p class="window-empty">{strings.providers.noProviders}</p>
        {:else}
          {#each providers as provider (provider.id)}
            <article class="provider-row">
              <span class={`status-dot ${providerTone(provider)}`}></span>
              <div>
                <strong>{provider.name}</strong>
                <span>{sanitizedUrl(provider.base_url)}</span>
                <small>
                  {provider.available_model_count ?? providerModelsFor(provider.id).filter((model) => model.available !== false).length} available /
                  {provider.model_count ?? providerModelsFor(provider.id).length} cached
                  {provider.model_refresh_completed_at ? ` · refreshed ${new Date(provider.model_refresh_completed_at).toLocaleString()}` : ''}
                </small>
                {#if providerRowState(provider)}
                  <small class={`row-state state-${providerRowState(provider)}`}>
                    {providerRowState(provider)}
                    {#if provider.model_refresh_duration_ms}
                      · {Math.round(provider.model_refresh_duration_ms / 1000)}s
                    {/if}
                  </small>
                {/if}
                {#if provider.last_error || provider.model_refresh_error_message}
                  <small class="danger-text">{provider.last_error || provider.model_refresh_error_message}</small>
                {/if}
              </div>
              <div class="row-actions">
                <button on:click={() => editProvider(provider)} type="button">Edit</button>
                <button disabled={stateFor(`provider:${provider.id}:toggle`) === 'enabling' || stateFor(`provider:${provider.id}:toggle`) === 'disabling'} on:click={() => onToggleProviderEnabled(provider)} type="button">
                  {stateFor(`provider:${provider.id}:toggle`) || (provider.enabled ? 'Disable' : 'Enable')}
                </button>
                <button disabled={stateFor(`provider:${provider.id}:test`) === 'testing'} on:click={() => onTestProvider(provider.id)} type="button">
                  {stateFor(`provider:${provider.id}:test`) === 'testing' ? 'Testing...' : strings.providers.test}
                </button>
                <button disabled={modelRefreshLabel(provider) === 'refreshing' || modelRefreshLabel(provider) === 'queued'} on:click={() => onRefreshProviderModels(provider.id)} type="button">
                  {modelRefreshLabel(provider) === 'refreshing' || modelRefreshLabel(provider) === 'queued' ? modelRefreshLabel(provider) : 'Refresh models'}
                </button>
                <button class="danger" on:click={() => onDeleteProvider(provider.id)} type="button">Delete</button>
              </div>
            </article>
          {/each}
        {/if}
      </div>
    {:else if activeSection === 'defaults'}
      <div class="settings-panel-heading">
        <h3>AI Defaults</h3>
        <span>{providerModels.length} cached models</span>
      </div>
      <div class="model-defaults-list">
        <section>
          <header>
            <strong>Chat Model</strong>
            <span>Normal conversations and final answers.</span>
          </header>
          {#each chatRoleEntries as entry, index (index)}
            <ModelPicker
              bind:selectedModelId={entry.model_id}
              bind:selectedProviderId={entry.provider_id}
              label={index === 0 ? 'Chat primary model' : `Chat fallback ${index}`}
              models={providerModels}
              {providers}
              role="chat"
            />
          {/each}
          <button class:dirty={roleDirty('chat', chatRoleEntries)} disabled={stateFor('model-role:chat') === 'saving'} on:click={() => saveRole('chat', chatRoleEntries)} type="button">
            {stateFor('model-role:chat') === 'saving' ? 'Saving...' : stateFor('model-role:chat') === 'saved' ? 'Saved' : roleDirty('chat', chatRoleEntries) ? 'Save Chat Chain*' : 'Save Chat Chain'}
          </button>
        </section>
        <section>
          <header>
            <strong>Utility Model</strong>
            <span>Summaries, titles, reply drafts, and worker AI operations.</span>
          </header>
          {#each utilityRoleEntries as entry, index (index)}
            <ModelPicker
              bind:selectedModelId={entry.model_id}
              bind:selectedProviderId={entry.provider_id}
              label={index === 0 ? 'Utility primary model' : `Utility fallback ${index}`}
              models={providerModels}
              {providers}
              role="utility"
            />
          {/each}
          <button class:dirty={roleDirty('utility', utilityRoleEntries)} disabled={stateFor('model-role:utility') === 'saving'} on:click={() => saveRole('utility', utilityRoleEntries)} type="button">
            {stateFor('model-role:utility') === 'saving' ? 'Saving...' : stateFor('model-role:utility') === 'saved' ? 'Saved' : roleDirty('utility', utilityRoleEntries) ? 'Save Utility Chain*' : 'Save Utility Chain'}
          </button>
        </section>
        <section>
          <header>
            <strong>Vision Model</strong>
            <span>Visual-understanding requests when a compatible model exists.</span>
          </header>
          {#each visionRoleEntries as entry, index (index)}
            <ModelPicker
              bind:selectedModelId={entry.model_id}
              bind:selectedProviderId={entry.provider_id}
              label={index === 0 ? 'Vision primary model' : `Vision fallback ${index}`}
              models={providerModels}
              {providers}
              role="vision"
            />
          {/each}
          <button class:dirty={roleDirty('vision', visionRoleEntries)} disabled={stateFor('model-role:vision') === 'saving'} on:click={() => saveRole('vision', visionRoleEntries)} type="button">
            {stateFor('model-role:vision') === 'saving' ? 'Saving...' : stateFor('model-role:vision') === 'saved' ? 'Saved' : roleDirty('vision', visionRoleEntries) ? 'Save Vision Chain*' : 'Save Vision Chain'}
          </button>
        </section>
      </div>
    {:else if activeSection === 'appearance'}
      <div class="settings-panel-heading">
        <h3>Appearance</h3>
        <button on:click={applyAppearance} type="button">Apply</button>
      </div>
      <div class="settings-form-grid">
        <label class="toggle-line">
          <input bind:checked={patternEnabled} on:change={applyAppearance} type="checkbox" />
          Background pattern
        </label>
        <label>
          Density
          <select bind:value={density} on:change={applyAppearance}>
            <option value="compact">compact</option>
            <option value="comfortable">comfortable</option>
          </select>
        </label>
        <label>
          UI scale
          <input bind:value={uiScale} max="115" min="90" on:input={applyAppearance} type="range" />
        </label>
        <label>
          Accent color
          <input bind:value={accentColor} on:input={applyAppearance} type="color" />
        </label>
        <label>
          Message width
          <input bind:value={messageWidth} max="980" min="680" on:input={applyAppearance} type="range" />
        </label>
        <label class="toggle-line">
          <input bind:checked={reducedMotion} on:change={applyAppearance} type="checkbox" />
          Reduced motion
        </label>
      </div>
    {:else if activeSection === 'shortcuts'}
      <div class="settings-panel-heading">
        <h3>Shortcuts</h3>
      </div>
      <div class="shortcut-list">
        {#each shortcuts as [name, value] (name)}
          <div>
            <span>{name}</span>
            <kbd>{value}</kbd>
          </div>
        {/each}
      </div>
    {:else if activeSection === 'account'}
      <div class="settings-panel-heading">
        <h3>Account</h3>
      </div>
      <dl class="compact-dl">
        <div><dt>Email</dt><dd>{user.email}</dd></div>
        <div><dt>Display name</dt><dd>{user.display_name}</dd></div>
        <div><dt>Role</dt><dd>{user.role}</dd></div>
        <div><dt>Workspace</dt><dd>{user.workspace_id}</dd></div>
      </dl>
    {:else if activeSection === 'sessions'}
      <div class="settings-panel-heading">
        <h3>{strings.auth.sessions}</h3>
        <button on:click={onRefreshSessions} type="button"><Icon name="refresh" size={13} /> Refresh</button>
      </div>
      <div class="dense-row-list">
        {#if sessions.length === 0}
          <p class="window-empty">No active sessions.</p>
        {:else}
          {#each sessions as session (session.id)}
            <article>
              <span class="status-dot healthy"></span>
              <div>
                <strong>{session.user_agent || 'Unknown client'}</strong>
                <span>{session.ip_address || 'Unknown address'}</span>
                <small>Expires {new Date(session.expires_at).toLocaleString()}</small>
              </div>
              <div class="row-actions">
                <button disabled={submitting} on:click={() => onRevokeSession(session.id)} type="button">{strings.auth.revoke}</button>
              </div>
            </article>
          {/each}
        {/if}
      </div>
    {:else if activeSection === 'system'}
      <div class="settings-panel-heading">
        <h3>System</h3>
        <button on:click={onRefreshDiagnostics} type="button"><Icon name="refresh" size={13} /> Refresh</button>
      </div>
      {#if status}
        <dl class="compact-dl">
          <div><dt>Version</dt><dd>{status.version}</dd></div>
          <div><dt>Database</dt><dd>{status.database.driver ?? 'unknown'} / {status.database.ok ? 'online' : 'offline'}</dd></div>
          {#each Object.entries(status.components) as [name, value] (name)}
            <div><dt>{name.replaceAll('_', ' ')}</dt><dd>{value}</dd></div>
          {/each}
        </dl>
      {:else}
        <p class="window-empty">Diagnostics not loaded.</p>
      {/if}
    {:else if activeSection === 'agent-tools'}
      <div class="settings-panel-heading">
        <h3>Agent Tools</h3>
        <button on:click={onRefreshFeedbackStats} type="button"><Icon name="refresh" size={13} /> Refresh stats</button>
      </div>
      <dl class="compact-dl two-col">
        <div><dt>Positive feedback</dt><dd>{feedbackStats.positive}</dd></div>
        <div><dt>Negative feedback</dt><dd>{feedbackStats.negative}</dd></div>
      </dl>
      <form class="settings-form-grid" on:submit|preventDefault={onCreateReplyPreset}>
        <label>
          Preset name
          <input bind:value={replyPresetName} required />
        </label>
        <label>
          Description
          <input bind:value={replyPresetDescription} />
        </label>
        <label class="span-2">
          Prompt instruction
          <textarea bind:value={replyPresetInstruction} required></textarea>
        </label>
        <button type="submit">{strings.replies.addPreset}</button>
        <button on:click={onResetReplyPresets} type="button">{strings.replies.resetDefaults}</button>
      </form>
      <div class="dense-row-list">
        {#each replyPresets as preset (preset.id)}
          <article>
            <span class={`status-dot ${preset.active ? 'healthy' : 'disabled'}`}></span>
            <div>
              <strong>{preset.name}</strong>
              <span>{preset.description}</span>
              <small>{preset.system_default ? 'default' : 'custom'}</small>
            </div>
            <div class="row-actions">
              <button on:click={() => onToggleReplyPreset(preset)} type="button">{preset.active ? 'Disable' : 'Enable'}</button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>
</div>
