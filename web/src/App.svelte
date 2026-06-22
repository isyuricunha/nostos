<script lang="ts">
  import { onMount } from 'svelte';
  import { getJSON } from './lib/api';
  import { strings } from './strings';

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

  const navItems = [
    strings.nav.chat,
    strings.nav.agents,
    strings.nav.memories,
    strings.nav.tasks,
    strings.nav.mcp,
    strings.nav.providers,
    strings.nav.settings
  ];

  let status: ReadyStatus | null = null;
  let errorMessage = '';

  onMount(async () => {
    try {
      status = await getJSON<ReadyStatus>('/api/v1/diagnostics');
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : 'Unable to load diagnostics.';
    }
  });
</script>

<main class="app-shell">
  <aside class="sidebar" aria-label="Main navigation">
    <div class="brand">
      <span class="brand-mark" aria-hidden="true">N</span>
      <span>{strings.appName}</span>
    </div>
    <nav>
      {#each navItems as item (item)}
        <a href={`#${item.toLowerCase()}`}>{item}</a>
      {/each}
    </nav>
  </aside>

  <section class="workspace" aria-labelledby="workspace-title">
    <header class="topbar">
      <div>
        <p class="eyebrow">Self-hosted workspace</p>
        <h1 id="workspace-title">{strings.foundation.title}</h1>
      </div>
      <div class:online={status?.ready} class="health-pill">
        {status?.ready ? 'Ready' : 'Checking'}
      </div>
    </header>

    <section class="panel" aria-labelledby="status-title">
      <h2 id="status-title">{strings.foundation.status}</h2>
      <p>{strings.foundation.subtitle}</p>

      {#if errorMessage}
        <div class="notice error" role="alert">{errorMessage}</div>
      {:else if status}
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
        <div class="skeleton" aria-label="Loading diagnostics"></div>
      {/if}
    </section>
  </section>
</main>
