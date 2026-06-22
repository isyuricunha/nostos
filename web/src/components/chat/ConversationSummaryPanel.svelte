<script lang="ts">
  import type { Conversation } from '../../lib/types';

  export let conversation: Conversation | undefined;
  export let onRegenerate: () => void | Promise<void>;
  export let onClear: () => void | Promise<void>;
</script>

{#if conversation}
  <details class="summary-panel" open={Boolean(conversation.summary)}>
    <summary>
      Conversation summary
      {#if conversation.summary_status}
        <span>{conversation.summary_status}</span>
      {/if}
    </summary>
    {#if conversation.summary}
      <p>{conversation.summary}</p>
      {#if conversation.summary_updated_at}
        <small>Updated {new Date(conversation.summary_updated_at).toLocaleString()}</small>
      {/if}
    {:else if conversation.summary_error}
      <p>{conversation.summary_error}</p>
    {:else}
      <p>No summary stored for this conversation.</p>
    {/if}
    <div class="message-actions">
      <button on:click={onRegenerate} type="button">Regenerate summary</button>
      {#if conversation.summary}
        <button on:click={onClear} type="button">Clear summary</button>
      {/if}
    </div>
  </details>
{/if}
