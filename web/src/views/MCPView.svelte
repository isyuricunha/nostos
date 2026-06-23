<script lang="ts">
  import Icon from '../components/common/Icon.svelte';
  import type { MCPServer, MCPTool } from '../lib/types';
  import { strings } from '../strings';

  export let mcpServers: MCPServer[] = [];
  export let mcpTools: MCPTool[] = [];
  export let editingMCPServerId = '';
  export let mcpName = '';
  export let mcpDescription = '';
  export let mcpTransport = 'http';
  export let mcpHttpUrl = '';
  export let mcpHttpHeaders = '';
  export let mcpCommand = '';
  export let mcpArguments = '';
  export let mcpWorkingDirectory = '';
  export let mcpEnvironment = '';
  export let mcpAuthorization = '';
  export let mcpStartupTimeoutMS = 10000;
  export let mcpRequestTimeoutMS = 30000;
  export let mcpEnabled = true;
  export let onSubmit: () => void | Promise<void>;
  export let onCancelEdit: () => void;
  export let onRefresh: () => void | Promise<void>;
  export let onEdit: (server: MCPServer) => void;
  export let onDelete: (serverId: string) => void | Promise<void>;
  export let onDiscoverTools: (serverId: string) => void | Promise<void>;
  export let onTest: (serverId: string) => void | Promise<void>;
  export let onUpdateToolPermission: (toolId: string, permissionMode: string) => void | Promise<void>;

  type Tab = 'servers' | 'tools' | 'permissions';

  let activeTab: Tab = 'servers';
  let query = '';
  let formOpen = false;
  let openMenuId = '';

  $: if (editingMCPServerId) {
    formOpen = true;
  }
  $: filteredServers = mcpServers.filter((server) =>
    `${server.name} ${server.description} ${server.transport_type} ${server.http_url ?? ''}`
      .toLowerCase()
      .includes(query.trim().toLowerCase())
  );
  $: filteredTools = mcpTools.filter((tool) =>
    `${tool.name} ${tool.description} ${serverName(tool.server_id)} ${tool.permission_mode}`.toLowerCase().includes(query.trim().toLowerCase())
  );

  function openCreate(): void {
    onCancelEdit();
    formOpen = true;
  }

  function closeForm(): void {
    onCancelEdit();
    formOpen = false;
  }

  function startEdit(server: MCPServer): void {
    onEdit(server);
    formOpen = true;
    openMenuId = '';
  }

  async function submitForm(): Promise<void> {
    await onSubmit();
    formOpen = false;
  }

  function serverName(serverId: string): string {
    return mcpServers.find((server) => server.id === serverId)?.name ?? 'Unknown server';
  }

  function healthClass(server: MCPServer): string {
    if (!server.enabled) return 'disabled';
    if (server.health_status === 'healthy') return 'healthy';
    if (server.health_status === 'unhealthy') return 'unhealthy';
    return 'unknown';
  }

  function transportDetail(server: MCPServer): string {
    if (server.transport_type === 'http') return server.http_url ?? 'HTTP URL not set';
    return server.command ? `${server.command} ${server.arguments.join(' ')}`.trim() : 'stdio command not set';
  }
</script>

<div class:editor-open={formOpen} class="workspace-module-grid">
  <section class="window-list-pane" aria-label="Tools">
    <header class="window-panel-toolbar">
      <div>
        <strong>Tools</strong>
        <span>{mcpServers.length} MCP servers · {mcpTools.length} tools</span>
      </div>
      <div class="window-toolbar-actions">
        <button aria-label="Refresh tools" on:click={onRefresh} type="button"><Icon name="refresh" size={13} /></button>
        <button on:click={openCreate} type="button"><Icon name="plus" size={13} /> New MCP server</button>
      </div>
    </header>

    <nav class="segmented-tabs" aria-label="Tools tabs">
      <button class:active={activeTab === 'servers'} on:click={() => (activeTab = 'servers')} type="button">MCP Servers</button>
      <button class:active={activeTab === 'tools'} on:click={() => (activeTab = 'tools')} type="button">Discovered Tools</button>
      <button class:active={activeTab === 'permissions'} on:click={() => (activeTab = 'permissions')} type="button">Permissions</button>
    </nav>

    <div class="window-filter-row">
      <label class="window-search">
        <Icon name="search" size={13} />
        <input bind:value={query} placeholder="Search tools" />
      </label>
    </div>

    {#if activeTab === 'servers'}
      {#if filteredServers.length === 0}
        <p class="window-empty">{strings.mcp.noServers}</p>
      {:else}
        <div class="dense-row-list">
          {#each filteredServers as server (server.id)}
            <article class="tool-row">
              <span class="row-icon"><Icon name="tools" size={15} /></span>
              <span class={`status-dot ${healthClass(server)}`}></span>
              <div>
                <strong>{server.name}</strong>
                <span>{server.description || transportDetail(server)}</span>
                <small>
                  {server.transport_type} · timeout {server.request_timeout_ms} ms
                  {server.last_connected_at ? ` · connected ${new Date(server.last_connected_at).toLocaleString()}` : ''}
                </small>
                {#if server.last_error}
                  <small class="danger-text">{server.last_error}</small>
                {/if}
              </div>
              <div class="row-actions compact">
                <button aria-label={`MCP server menu for ${server.name}`} on:click={() => (openMenuId = openMenuId === server.id ? '' : server.id)} type="button">
                  <Icon name="kebab" size={14} />
                </button>
                {#if openMenuId === server.id}
                  <div class="row-menu row-menu-right" role="menu">
                    <button on:click={() => startEdit(server)} type="button"><Icon name="edit" size={13} /> Edit</button>
                    <button on:click={() => { openMenuId = ''; onTest(server.id); }} type="button"><Icon name="check" size={13} /> Test</button>
                    <button on:click={() => { openMenuId = ''; onDiscoverTools(server.id); }} type="button">
                      <Icon name="search" size={13} /> {strings.mcp.discover}
                    </button>
                    <button class="danger" on:click={() => { openMenuId = ''; onDelete(server.id); }} type="button">
                      <Icon name="trash" size={13} /> Delete
                    </button>
                  </div>
                {/if}
              </div>
            </article>
          {/each}
        </div>
      {/if}
    {:else if activeTab === 'tools'}
      {#if filteredTools.length === 0}
        <p class="window-empty">{strings.mcp.noTools}</p>
      {:else}
        <div class="dense-row-list">
          {#each filteredTools as tool (tool.id)}
            <article class="tool-row">
              <span class="row-icon"><Icon name="tools" size={15} /></span>
              <span class={`status-dot ${tool.permission_mode === 'allow' ? 'healthy' : tool.permission_mode === 'deny' ? 'unhealthy' : 'unknown'}`}></span>
              <div>
                <strong>{tool.name}</strong>
                <span>{tool.description || 'No description'}</span>
                <small>{serverName(tool.server_id)} · permission {tool.permission_mode}</small>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    {:else}
      {#if filteredTools.length === 0}
        <p class="window-empty">No tools available for permissions.</p>
      {:else}
        <div class="dense-row-list">
          {#each filteredTools as tool (tool.id)}
            <article class="tool-row">
              <span class="row-icon"><Icon name="tools" size={15} /></span>
              <div>
                <strong>{tool.name}</strong>
                <span>{serverName(tool.server_id)}</span>
              </div>
              <div class="row-actions">
                <select
                  aria-label={`Permission for ${tool.name}`}
                  value={tool.permission_mode}
                  on:change={(event) => onUpdateToolPermission(tool.id, event.currentTarget.value)}
                >
                  <option value="deny">deny</option>
                  <option value="ask">ask</option>
                  <option value="allow">allow</option>
                </select>
              </div>
            </article>
          {/each}
        </div>
      {/if}
    {/if}
  </section>

  {#if formOpen}
    <aside class="window-editor-panel" aria-label={editingMCPServerId ? 'Edit MCP server' : 'Create MCP server'}>
      <header>
        <strong>{editingMCPServerId ? 'Edit MCP server' : strings.mcp.add}</strong>
        <button aria-label="Close MCP server editor" on:click={closeForm} type="button"><Icon name="close" size={13} /></button>
      </header>
      <form class="compact-editor-form" on:submit|preventDefault={submitForm}>
        <label>
          Name
          <input bind:value={mcpName} required />
        </label>
        <label>
          Description
          <input bind:value={mcpDescription} />
        </label>
        <div class="two-col">
          <label>
            Transport
            <select bind:value={mcpTransport}>
              <option value="http">http</option>
              <option value="stdio">stdio</option>
            </select>
          </label>
          <label class="toggle-line">
            <input bind:checked={mcpEnabled} type="checkbox" />
            Enabled
          </label>
        </div>

        {#if mcpTransport === 'http'}
          <label>
            HTTP URL
            <input bind:value={mcpHttpUrl} placeholder="http://localhost:9000/mcp" required />
          </label>
          <label>
            HTTP headers JSON
            <textarea bind:value={mcpHttpHeaders} placeholder="Write-only replacement headers"></textarea>
          </label>
          <label>
            Authorization header
            <input bind:value={mcpAuthorization} autocomplete="off" placeholder="Bearer token" type="password" />
          </label>
        {:else}
          <label>
            Command
            <input bind:value={mcpCommand} required />
          </label>
          <label>
            Arguments
            <input bind:value={mcpArguments} placeholder="--stdio" />
          </label>
          <label>
            Working directory
            <input bind:value={mcpWorkingDirectory} />
          </label>
          <label>
            Environment JSON
            <textarea bind:value={mcpEnvironment} placeholder="Write-only replacement environment"></textarea>
          </label>
        {/if}

        <div class="two-col">
          <label>
            Startup timeout, ms
            <input bind:value={mcpStartupTimeoutMS} min="1000" type="number" />
          </label>
          <label>
            Request timeout, ms
            <input bind:value={mcpRequestTimeoutMS} min="1000" type="number" />
          </label>
        </div>
        <div class="editor-actions">
          <button type="submit">{editingMCPServerId ? 'Save MCP server' : strings.mcp.add}</button>
          <button on:click={closeForm} type="button">Cancel</button>
        </div>
      </form>
    </aside>
  {/if}
</div>
