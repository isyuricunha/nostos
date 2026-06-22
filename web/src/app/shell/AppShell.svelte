<script lang="ts">
  import SidebarItem from '../../components/common/SidebarItem.svelte';
  import StatusPill from '../../components/common/StatusPill.svelte';
  import type { ReadyStatus, User } from '../../lib/types';
  import { strings } from '../../strings';

  export let activeView: string;
  export let navItems: string[] = [];
  export let user: User;
  export let status: ReadyStatus | null = null;
  export let submitting = false;
  export let onLogout: () => void | Promise<void>;

  const navIcons: Record<string, string> = {
    Chat: 'C',
    Agents: 'A',
    Memories: 'M',
    Tasks: 'T',
    MCP: 'P',
    Providers: 'I',
    Settings: 'S'
  };

  $: readyTone = (status?.ready ? 'success' : 'warning') as 'success' | 'warning';
  $: readyLabel = status?.ready ? 'Ready' : 'Checking';
</script>

<main class="app-shell">
  <aside class="sidebar" aria-label="Main navigation">
    <div class="brand">
      <span class="brand-mark" aria-hidden="true">N</span>
      <div>
        <strong>{strings.appName}</strong>
        <span>AI workspace</span>
      </div>
    </div>

    <nav>
      {#each navItems as item (item)}
        <SidebarItem
          active={activeView === item}
          icon={navIcons[item] ?? item.slice(0, 1)}
          label={item}
          onSelect={() => (activeView = item)}
        />
      {/each}
    </nav>

    <div class="sidebar-footer">
      <StatusPill status={readyLabel} tone={readyTone} />
      <div class="sidebar-user">
        <span>{user.display_name}</span>
        <small>{user.email}</small>
      </div>
      <button disabled={submitting} on:click={onLogout} type="button">{strings.auth.signOut}</button>
    </div>
  </aside>

  <section class="workspace" aria-labelledby="workspace-title">
    <header class="topbar">
      <div>
        <p class="eyebrow">Self-hosted command center</p>
        <h1 id="workspace-title">{activeView}</h1>
      </div>
      <div class="topbar-meta">
        {#if status}
          <StatusPill status={`${status.database.driver ?? 'database'} ${status.database.ok ? 'online' : 'offline'}`} tone={status.database.ok ? 'success' : 'danger'} />
        {/if}
        <span>{new Date().toLocaleDateString()}</span>
      </div>
    </header>

    <slot />
  </section>
</main>
