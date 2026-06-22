<script lang="ts">
  import EmptyState from '../components/common/EmptyState.svelte';
  import StatusPill from '../components/common/StatusPill.svelte';
  import type { FeedbackStats, ReadyStatus, ReplyPreset, Session, User } from '../lib/types';
  import { strings } from '../strings';

  export let user: User;
  export let sessions: Session[] = [];
  export let status: ReadyStatus | null = null;
  export let feedbackStats: FeedbackStats = { positive: 0, negative: 0 };
  export let replyPresets: ReplyPreset[] = [];
  export let replyPresetName = '';
  export let replyPresetDescription = '';
  export let replyPresetInstruction = '';
  export let submitting = false;
  export let onRefreshSessions: () => void | Promise<void>;
  export let onRevokeSession: (sessionId: string) => void | Promise<void>;
  export let onRefreshDiagnostics: () => void | Promise<void>;
  export let onRefreshFeedbackStats: () => void | Promise<void>;
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
