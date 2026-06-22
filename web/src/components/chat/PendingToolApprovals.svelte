<script lang="ts">
  import type { ToolCall } from '../../lib/types';

  export let toolCalls: ToolCall[] = [];
  export let onRefresh: () => void | Promise<void>;
  export let onApprove: (
    toolCall: ToolCall,
    decision: 'approve_once' | 'approve_conversation' | 'allow_agent'
  ) => void | Promise<void>;
  export let onDeny: (toolCall: ToolCall, decision: 'deny' | 'deny_disable_tool') => void | Promise<void>;
</script>

{#if toolCalls.length > 0}
  <section class="tool-panel" aria-label="Pending tool approvals">
    <div class="panel-heading">
      <h2>Pending tool approvals</h2>
      <button on:click={onRefresh} type="button">Refresh</button>
    </div>
    {#each toolCalls as toolCall (toolCall.id)}
      <article>
        <strong>{toolCall.name}</strong>
        <span>{toolCall.state}</span>
        <pre>{toolCall.input}</pre>
        {#if toolCall.error_message}
          <p>{toolCall.error_message}</p>
        {/if}
        <div class="message-actions">
          <button on:click={() => onApprove(toolCall, 'approve_once')} type="button">Approve once</button>
          <button on:click={() => onApprove(toolCall, 'approve_conversation')} type="button">
            Approve conversation
          </button>
          <button on:click={() => onApprove(toolCall, 'allow_agent')} type="button">Allow for agent</button>
          <button on:click={() => onDeny(toolCall, 'deny')} type="button">Deny</button>
          <button on:click={() => onDeny(toolCall, 'deny_disable_tool')} type="button">Deny and disable</button>
        </div>
      </article>
    {/each}
  </section>
{/if}
