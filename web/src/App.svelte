<script lang="ts">
  import { onMount } from 'svelte';
  import { deleteJSON, getJSON, postJSON } from './lib/api';
  import { strings } from './strings';

  type User = {
    id: string;
    email: string;
    display_name: string;
    role: string;
    workspace_id: string;
  };

  type Session = {
    id: string;
    ip_address?: string;
    user_agent?: string;
    expires_at: string;
    created_at: string;
  };

  type ReadyStatus = {
    ready: boolean;
    version: string;
    database: {
      ok: boolean;
      driver?: string;
      message?: string;
    };
    components: Record<string, string>;
  };

  type SetupStatus = {
    available: boolean;
  };

  type UserResponse = {
    user: User;
  };

  type SessionsResponse = {
    sessions: Session[];
  };

  const navItems = [
    strings.nav.chat,
    strings.nav.agents,
    strings.nav.memories,
    strings.nav.tasks,
    strings.nav.mcp,
    strings.nav.providers,
    strings.nav.settings
  ];

  let setupAvailable = false;
  let user: User | null = null;
  let sessions: Session[] = [];
  let status: ReadyStatus | null = null;
  let activeView: string = strings.nav.chat;
  let loading = true;
  let submitting = false;
  let notice = '';
  let errorMessage = '';

  let setupEmail = '';
  let setupDisplayName = '';
  let setupPassword = '';
  let setupConfirmPassword = '';
  let loginEmail = '';
  let loginPassword = '';

  onMount(async () => {
    await refreshAppState();
  });

  async function refreshAppState(): Promise<void> {
    loading = true;
    errorMessage = '';
    try {
      const setup = await getJSON<SetupStatus>('/api/v1/setup/status');
      setupAvailable = setup.available;
      if (!setupAvailable) {
        await refreshUser();
      }
      if (user) {
        await Promise.all([refreshSessions(), refreshDiagnostics()]);
      }
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      loading = false;
    }
  }

  async function refreshUser(): Promise<void> {
    try {
      const response = await getJSON<UserResponse>('/api/v1/auth/me');
      user = response.user;
    } catch {
      user = null;
    }
  }

  async function refreshSessions(): Promise<void> {
    const response = await getJSON<SessionsResponse>('/api/v1/sessions');
    sessions = response.sessions;
  }

  async function refreshDiagnostics(): Promise<void> {
    status = await getJSON<ReadyStatus>('/api/v1/diagnostics');
  }

  async function submitSetup(): Promise<void> {
    submitting = true;
    notice = '';
    errorMessage = '';
    try {
      const response = await postJSON<UserResponse>('/api/v1/setup', {
        email: setupEmail,
        display_name: setupDisplayName,
        password: setupPassword,
        confirm_password: setupConfirmPassword
      });
      user = response.user;
      setupAvailable = false;
      setupPassword = '';
      setupConfirmPassword = '';
      notice = 'Owner account created.';
      await Promise.all([refreshSessions(), refreshDiagnostics()]);
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      submitting = false;
    }
  }

  async function submitLogin(): Promise<void> {
    submitting = true;
    notice = '';
    errorMessage = '';
    try {
      const response = await postJSON<UserResponse>('/api/v1/auth/login', {
        email: loginEmail,
        password: loginPassword
      });
      user = response.user;
      loginPassword = '';
      notice = 'Signed in.';
      await Promise.all([refreshSessions(), refreshDiagnostics()]);
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      submitting = false;
    }
  }

  async function logout(): Promise<void> {
    submitting = true;
    notice = '';
    errorMessage = '';
    try {
      await postJSON<{ ok: boolean }>('/api/v1/auth/logout');
      user = null;
      sessions = [];
      status = null;
      notice = 'Signed out.';
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      submitting = false;
    }
  }

  async function revokeSession(sessionId: string): Promise<void> {
    if (!confirm('Revoke this session?')) {
      return;
    }
    submitting = true;
    errorMessage = '';
    try {
      await deleteJSON<{ ok: boolean }>(`/api/v1/sessions/${sessionId}`);
      await refreshSessions();
      notice = 'Session revoked.';
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      submitting = false;
    }
  }

  function messageFromError(error: unknown): string {
    return error instanceof Error ? error.message : 'The request failed.';
  }
</script>

{#if loading}
  <main class="auth-screen">
    <div class="auth-panel">
      <div class="skeleton" aria-label="Loading application state"></div>
    </div>
  </main>
{:else if setupAvailable}
  <main class="auth-screen">
    <form class="auth-panel" on:submit|preventDefault={submitSetup}>
      <span class="brand-mark" aria-hidden="true">N</span>
      <h1>{strings.auth.setupTitle}</h1>
      <p>{strings.auth.setupSubtitle}</p>
      <label>
        {strings.auth.email}
        <input bind:value={setupEmail} autocomplete="email" required type="email" />
      </label>
      <label>
        {strings.auth.displayName}
        <input bind:value={setupDisplayName} autocomplete="name" />
      </label>
      <label>
        {strings.auth.password}
        <input bind:value={setupPassword} autocomplete="new-password" minlength="12" required type="password" />
      </label>
      <label>
        {strings.auth.confirmPassword}
        <input
          bind:value={setupConfirmPassword}
          autocomplete="new-password"
          minlength="12"
          required
          type="password"
        />
      </label>
      {#if errorMessage}
        <div class="notice error" role="alert">{errorMessage}</div>
      {/if}
      <button disabled={submitting} type="submit">{submitting ? 'Creating...' : strings.auth.createOwner}</button>
    </form>
  </main>
{:else if !user}
  <main class="auth-screen">
    <form class="auth-panel" on:submit|preventDefault={submitLogin}>
      <span class="brand-mark" aria-hidden="true">N</span>
      <h1>{strings.auth.loginTitle}</h1>
      <p>{strings.auth.loginSubtitle}</p>
      <label>
        {strings.auth.email}
        <input bind:value={loginEmail} autocomplete="email" required type="email" />
      </label>
      <label>
        {strings.auth.password}
        <input bind:value={loginPassword} autocomplete="current-password" required type="password" />
      </label>
      {#if notice}
        <div class="notice success">{notice}</div>
      {/if}
      {#if errorMessage}
        <div class="notice error" role="alert">{errorMessage}</div>
      {/if}
      <button disabled={submitting} type="submit">{submitting ? 'Signing in...' : strings.auth.signIn}</button>
    </form>
  </main>
{:else}
  <main class="app-shell">
    <aside class="sidebar" aria-label="Main navigation">
      <div class="brand">
        <span class="brand-mark" aria-hidden="true">N</span>
        <span>{strings.appName}</span>
      </div>
      <nav>
        {#each navItems as item (item)}
          <button class:active={activeView === item} on:click={() => (activeView = item)} type="button">{item}</button>
        {/each}
      </nav>
    </aside>

    <section class="workspace" aria-labelledby="workspace-title">
      <header class="topbar">
        <div>
          <p class="eyebrow">Self-hosted workspace</p>
          <h1 id="workspace-title">{activeView}</h1>
        </div>
        <div class="user-menu">
          <span>{user.display_name}</span>
          <button disabled={submitting} on:click={logout} type="button">{strings.auth.signOut}</button>
        </div>
      </header>

      {#if notice}
        <div class="notice success">{notice}</div>
      {/if}
      {#if errorMessage}
        <div class="notice error" role="alert">{errorMessage}</div>
      {/if}

      {#if activeView === strings.nav.settings}
        <section class="panel" aria-labelledby="settings-title">
          <h2 id="settings-title">{strings.auth.currentUser}</h2>
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
            <h2 id="sessions-title">{strings.auth.sessions}</h2>
            <button on:click={refreshSessions} type="button">Refresh</button>
          </div>
          {#if sessions.length === 0}
            <p>No active sessions.</p>
          {:else}
            <div class="table-list">
              {#each sessions as session (session.id)}
                <article>
                  <div>
                    <strong>{session.user_agent || 'Unknown client'}</strong>
                    <span>{session.ip_address || 'Unknown address'}</span>
                  </div>
                  <div>
                    <span>Expires {new Date(session.expires_at).toLocaleString()}</span>
                    <button disabled={submitting} on:click={() => revokeSession(session.id)} type="button">
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
            <h2 id="diagnostics-title">{strings.workspace.diagnostics}</h2>
            <button on:click={refreshDiagnostics} type="button">Refresh</button>
          </div>
          {#if status}
            <dl class="status-grid">
              <div>
                <dt>Version</dt>
                <dd>{status.version}</dd>
              </div>
              <div>
                <dt>Database</dt>
                <dd>{status.database.driver ?? 'unknown'} / {status.database.ok ? 'online' : 'offline'}</dd>
              </div>
              {#each Object.entries(status.components) as [name, value] (name)}
                <div>
                  <dt>{name.replaceAll('_', ' ')}</dt>
                  <dd>{value}</dd>
                </div>
              {/each}
            </dl>
          {:else}
            <p>Diagnostics have not been loaded.</p>
          {/if}
        </section>
      {:else}
        <section class="panel" aria-labelledby="screen-title">
          <p class="eyebrow">{strings.workspace.title}</p>
          <h2 id="screen-title">{activeView}</h2>
          <p>{strings.workspace.emptyScreen}</p>
        </section>
      {/if}
    </section>
  </main>
{/if}
