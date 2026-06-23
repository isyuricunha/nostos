<script lang="ts">
  import EmptyState from '../components/common/EmptyState.svelte';
  import StatusPill from '../components/common/StatusPill.svelte';
  import ModelPicker from '../components/models/ModelPicker.svelte';
  import type { FeedbackStats, ModelRoleDraft, Provider, ProviderModel, ReadyStatus, ReplyPreset, Session, User } from '../lib/types';
  import { strings } from '../strings';

  export let user: User;
  export let sessions: Session[] = [];
  export let status: ReadyStatus | null = null;
  export let feedbackStats: FeedbackStats = { positive: 0, negative: 0 };
  export let providers: Provider[] = [];
  export let providerModels: ProviderModel[] = [];
  export let chatRoleEntries: ModelRoleDraft[] = [];
  export let utilityRoleEntries: ModelRoleDraft[] = [];
  export let visionRoleEntries: ModelRoleDraft[] = [];
  export let replyPresets: ReplyPreset[] = [];
  export let replyPresetName = '';
  export let replyPresetDescription = '';
  export let replyPresetInstruction = '';
  export let submitting = false;
  export let onRefreshSessions: () => void | Promise<void>;
  export let onRevokeSession: (sessionId: string) => void | Promise<void>;
  export let onRefreshDiagnostics: () => void | Promise<void>;
  export let onRefreshFeedbackStats: () => void | Promise<void>;
  export let onSaveModelRole: (role: 'chat' | 'utility' | 'vision', entries: ModelRoleDraft[]) => void | Promise<void>;
  export let onCreateReplyPreset: () => void | Promise<void>;
  export let onToggleReplyPreset: (preset: ReplyPreset) => void | Promise<void>;
  export let onResetReplyPresets: () => void | Promise<void>;
</script>

<div class="screen-grid">
  <section class="panel" aria-labelledby="settings-title">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">Profile</p>
        <h2 id="settings-title">{strings.auth.currentUser}</h2>
      </div>
      <StatusPill status={user.role} tone="accent" />
    </div>
    <dl class="status-grid">
      <div>
        <dt>Email</dt>
        <dd>{user.email}</dd>
      </div>
      <div>
        <dt>Role</dt>
        <dd>{user.role}</dd>
      </div>
      <div>
        <dt>Workspace</dt>
        <dd>{user.workspace_id}</dd>
      </div>
    </dl>
  </section>

  <section class="panel" aria-labelledby="sessions-title">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">Access</p>
        <h2 id="sessions-title">{strings.auth.sessions}</h2>
      </div>
      <button on:click={onRefreshSessions} type="button">Refresh</button>
    </div>
    {#if sessions.length === 0}
      <EmptyState description="Signed-in sessions will appear here." title="No active sessions" />
    {:else}
      <div class="table-list">
        {#each sessions as session (session.id)}
          <article>
            <div>
              <strong>{session.user_agent || 'Unknown client'}</strong>
              <span>{session.ip_address || 'Unknown address'}</span>
              <span>Expires {new Date(session.expires_at).toLocaleString()}</span>
            </div>
            <div>
              <button disabled={submitting} on:click={() => onRevokeSession(session.id)} type="button">
                {strings.auth.revoke}
              </button>
            </div>
          </article>
        {/each}
      </div>
    {/if}
  </section>

  <section class="panel models-settings-panel" aria-labelledby="models-title">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">Model platform</p>
        <h2 id="models-title">Model defaults</h2>
      </div>
      <StatusPill status={`${providerModels.length} cached models`} tone="accent" />
    </div>
    <div class="model-role-grid">
      <div class="model-role-card">
        <div>
          <h3>Chat Model</h3>
          <p>Default for normal conversations and final user-facing answers.</p>
        </div>
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
        <button on:click={() => onSaveModelRole('chat', chatRoleEntries)} type="button">Save Chat Chain</button>
      </div>
      <div class="model-role-card">
        <div>
          <h3>Utility Model</h3>
          <p>Used for summaries, titles, reply drafts, and lightweight worker AI operations.</p>
        </div>
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
        <button on:click={() => onSaveModelRole('utility', utilityRoleEntries)} type="button">Save Utility Chain</button>
      </div>
      <div class="model-role-card">
        <div>
          <h3>Vision Model</h3>
          <p>Configured now for future visual-understanding requests. Image generation is not implemented.</p>
        </div>
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
        <button on:click={() => onSaveModelRole('vision', visionRoleEntries)} type="button">Save Vision Chain</button>
      </div>
    </div>
    <div class="catalog-strip">
      {#each providers as provider (provider.id)}
        <article>
          <strong>{provider.name}</strong>
          <span>{provider.available_model_count ?? 0} available / {provider.model_count ?? 0} cached</span>
          <small>{provider.model_refresh_state ?? 'idle'}</small>
        </article>
      {/each}
    </div>
  </section>

  <section class="panel" aria-labelledby="diagnostics-title">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">Runtime</p>
        <h2 id="diagnostics-title">{strings.workspace.diagnostics}</h2>
      </div>
      <button on:click={onRefreshDiagnostics} type="button">Refresh</button>
    </div>
    {#if status}
      <dl class="status-grid">
        <div>
          <dt>Version</dt>
          <dd>{status.version}</dd>
        </div>
        <div>
          <dt>Database</dt>
          <dd>
            <StatusPill
              status={`${status.database.driver ?? 'unknown'} / ${status.database.ok ? 'online' : 'offline'}`}
              tone={status.database.ok ? 'success' : 'danger'}
            />
          </dd>
        </div>
        {#each Object.entries(status.components) as [name, value] (name)}
          <div>
            <dt>{name.replaceAll('_', ' ')}</dt>
            <dd>{value}</dd>
          </div>
        {/each}
      </dl>
    {:else}
      <EmptyState description="Refresh diagnostics to inspect runtime health." title="Diagnostics not loaded" />
    {/if}
  </section>

  <section class="panel" aria-labelledby="feedback-stats-title">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">Quality</p>
        <h2 id="feedback-stats-title">Feedback statistics</h2>
      </div>
      <button on:click={onRefreshFeedbackStats} type="button">Refresh</button>
    </div>
    <dl class="status-grid">
      <div>
        <dt>Positive</dt>
        <dd>{feedbackStats.positive}</dd>
      </div>
      <div>
        <dt>Negative</dt>
        <dd>{feedbackStats.negative}</dd>
      </div>
    </dl>
  </section>

  <section class="providers-layout">
    <form class="panel form-grid" on:submit|preventDefault={onCreateReplyPreset}>
      <div>
        <p class="eyebrow">Reply intents</p>
        <h2>{strings.replies.addPreset}</h2>
      </div>
      <label>
        Name
        <input bind:value={replyPresetName} required />
      </label>
      <label>
        Description
        <input bind:value={replyPresetDescription} />
      </label>
      <label>
        Prompt instruction
        <textarea bind:value={replyPresetInstruction} required></textarea>
      </label>
      <button type="submit">{strings.replies.addPreset}</button>
    </form>

    <section class="panel" aria-labelledby="reply-presets-title">
      <div class="panel-heading">
        <div>
          <p class="eyebrow">Drafting</p>
          <h2 id="reply-presets-title">{strings.replies.presets}</h2>
        </div>
        <button on:click={onResetReplyPresets} type="button">{strings.replies.resetDefaults}</button>
      </div>
      {#if replyPresets.length === 0}
        <EmptyState description="Create or reset reply intents to generate drafts from messages." title="No presets" />
      {:else}
        <div class="table-list">
          {#each replyPresets as preset (preset.id)}
            <article>
              <div>
                <strong>{preset.name}</strong>
                <span>{preset.description}</span>
                <span>{preset.active ? 'active' : 'disabled'}{preset.system_default ? ' / default' : ''}</span>
              </div>
              <div>
                <button on:click={() => onToggleReplyPreset(preset)} type="button">
                  {preset.active ? 'Disable' : 'Enable'}
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    </section>
  </section>
</div>

<style>
  .models-settings-panel {
    grid-column: 1 / -1;
  }

  .model-role-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: var(--space-4);
  }

  .model-role-card {
    display: grid;
    gap: var(--space-3);
    border: 1px solid var(--color-border-muted);
    border-radius: var(--radius-lg);
    background: rgba(255, 255, 255, 0.025);
    padding: var(--space-4);
  }

  .model-role-card p {
    margin-bottom: 0;
    font-size: var(--font-sm);
  }

  .catalog-strip {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--space-3);
    margin-top: var(--space-4);
  }

  .catalog-strip article {
    display: grid;
    gap: 2px;
    border: 1px solid var(--color-border-muted);
    border-radius: var(--radius-md);
    background: rgba(255, 255, 255, 0.02);
    padding: var(--space-3);
  }

  .catalog-strip span,
  .catalog-strip small {
    color: var(--color-subtle);
    font-size: var(--font-xs);
  }
</style>
