<script lang="ts">
  import EmptyState from '../components/common/EmptyState.svelte';
  import Modal from '../components/common/Modal.svelte';
  import StatusPill from '../components/common/StatusPill.svelte';
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

  let formOpen = false;

  $: if (editingMCPServerId) {
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

  function healthTone(server: MCPServer): 'success' | 'warning' | 'danger' | 'neutral' {
    if (!server.enabled) return 'neutral';
    if (server.health_status === 'healthy') return 'success';
    if (server.health_status === 'unhealthy') return 'danger';
    return 'warning';
  }
</script>

<section class="panel">
  <div class="panel-heading">
    <div>
      <p class="eyebrow">Connected tools</p>
      <h2>MCP servers</h2>
    </div>
    <div class="cluster">
      <button on:click={onRefresh} type="button">Refresh</button>
      <button on:click={openCreate} type="button">New MCP server</button>
    </div>
  </div>

  <Modal open={formOpen} title={editingMCPServerId ? 'Edit MCP server' : strings.mcp.add} onClose={closeForm}>
  <form class="form-grid" on:submit|preventDefault={submitForm}>
    <div class="form-section">
      <h3>Identity</h3>
      <label>
        Name
        <input bind:value={mcpName} required />
      </label>
      <label>
        Description
        <input bind:value={mcpDescription} />
      </label>
      <label>
        Transport
        <select bind:value={mcpTransport}>
          <option value="http">http</option>
          <option value="stdio">stdio</option>
        </select>
      </label>
      <label class="inline-check">
        <input bind:checked={mcpEnabled} type="checkbox" />
        Enabled
      </label>
    </div>

    {#if mcpTransport === 'http'}
      <div class="form-section">
        <h3>HTTP transport</h3>
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
      </div>
    {:else}
      <div class="form-section">
        <h3>stdio transport</h3>
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
      </div>
    {/if}

    <div class="form-section">
      <h3>Timeouts</h3>
      <label>
        Startup timeout, milliseconds
        <input bind:value={mcpStartupTimeoutMS} min="1000" type="number" />
      </label>
      <label>
        Request timeout, milliseconds
        <input bind:value={mcpRequestTimeoutMS} min="1000" type="number" />
      </label>
    </div>

    <button type="submit">{editingMCPServerId ? 'Save MCP server' : strings.mcp.add}</button>
  </form>
  </Modal>

    {#if mcpServers.length === 0}
      <EmptyState description="Connect local stdio or HTTP MCP servers to expose approved tools." title={strings.mcp.noServers} />
    {:else}
      <div class="table-list mcp-cards">
        {#each mcpServers as server (server.id)}
          <article>
            <div>
              <div class="split">
                <strong>{server.name}</strong>
                <StatusPill status={server.enabled ? server.health_status : 'disabled'} tone={healthTone(server)} />
              </div>
              <span>{server.transport_type} transport / timeout {server.request_timeout_ms} ms</span>
              {#if server.environment_keys?.length}
                <span>environment keys: {server.environment_keys.join(', ')}</span>
              {/if}
              {#if server.http_header_keys?.length}
                <span>header keys: {server.http_header_keys.join(', ')}</span>
              {/if}
              {#if server.last_connected_at}
                <span>last connected {new Date(server.last_connected_at).toLocaleString()}</span>
              {/if}
              {#if server.last_error}
                <span class="danger-text">{server.last_error}</span>
              {/if}
            </div>
            <div>
              <button on:click={() => onEdit(server)} type="button">Edit</button>
              <button on:click={() => onTest(server.id)} type="button">Test</button>
              <button on:click={() => onDiscoverTools(server.id)} type="button">{strings.mcp.discover}</button>
              <button on:click={() => onDelete(server.id)} type="button">Delete</button>
            </div>
          </article>
        {/each}
      </div>
    {/if}

    <div class="panel-heading nested-heading">
      <div>
        <p class="eyebrow">Permissions</p>
        <h2>Tools</h2>
      </div>
    </div>
    {#if mcpTools.length === 0}
      <EmptyState description="Discover tools from an enabled MCP server to configure permissions." title={strings.mcp.noTools} />
    {:else}
      <div class="table-list">
        {#each mcpTools as tool (tool.id)}
          <article>
            <div>
              <strong>{tool.name}</strong>
              <span>{tool.description}</span>
            </div>
            <div>
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
</section>
