<script lang="ts">
  import ConversationSummaryPanel from '../components/chat/ConversationSummaryPanel.svelte';
  import PendingToolApprovals from '../components/chat/PendingToolApprovals.svelte';
  import Dropdown from '../components/common/Dropdown.svelte';
  import EmptyState from '../components/common/EmptyState.svelte';
  import StatusPill from '../components/common/StatusPill.svelte';
  import ModelPicker from '../components/models/ModelPicker.svelte';
  import type {
    Agent,
    Conversation,
    MemorySnippet,
    Message,
    MessageFeedback,
    Provider,
    ProviderModel,
    ReplyPreset,
    ToolCall,
    ToolCard
  } from '../lib/types';
  import { strings } from '../strings';

  export let messages: Message[] = [];
  export let providers: Provider[] = [];
  export let providerModels: ProviderModel[] = [];
  export let agents: Agent[] = [];
  export let selectedConversation: Conversation | undefined;
  export let selectedProviderId = '';
  export let selectedAgentId = '';
  export let selectedModel = '';
  export let composer = '';
  export let activeRunId = '';
  export let submitting = false;
  export let runMemories: MemorySnippet[] = [];
  export let toolCards: ToolCard[] = [];
  export let pendingToolApprovals: ToolCall[] = [];
  export let feedbackByMessage: Record<string, MessageFeedback> = {};
  export let negativeFeedbackReason = 'Incorrect information';
  export let selectedReplySourceId = '';
  export let selectedReplyPresetId = '';
  export let replyCustomInstruction = '';
  export let replyDraft = '';
  export let replyPresets: ReplyPreset[] = [];
  export let renderMarkdown: (content: string) => string;
  export let onRefreshModels: (providerId?: string) => void | Promise<void>;
  export let onRegenerateSummary: () => void | Promise<void>;
  export let onClearSummary: () => void | Promise<void>;
  export let onSendMessage: () => void | Promise<void>;
  export let onStopGeneration: () => void | Promise<void>;
  export let onRememberMessage: (message: Message) => void | Promise<void>;
  export let onSelectReplySource: (message: Message) => void;
  export let onSubmitFeedback: (message: Message, rating: 'positive' | 'negative') => void | Promise<void>;
  export let onClearFeedback: (messageId: string) => void | Promise<void>;
  export let onRegenerateWithFeedback: (message: Message) => void | Promise<void>;
  export let onGenerateReplyDraft: () => void | Promise<void>;
  export let onInsertReplyDraft: () => void;
  export let onRefreshPendingToolApprovals: () => void | Promise<void>;
  export let onApproveToolCall: (
    toolCall: ToolCall,
    decision: 'approve_once' | 'approve_conversation' | 'allow_agent'
  ) => void | Promise<void>;
  export let onDenyToolCall: (toolCall: ToolCall, decision: 'deny' | 'deny_disable_tool') => void | Promise<void>;

  let feedbackPanelMessageId = '';
  let detailsMessageId = '';

  $: activeAgent = agents.find((agent) => agent.id === selectedAgentId);
  $: activeProvider = providers.find((provider) => provider.id === selectedProviderId);
  $: approvedTools = toolCards.filter((tool) => tool.state === 'succeeded').length;

  async function copyMessage(message: Message): Promise<void> {
    if (window.navigator.clipboard) {
      await window.navigator.clipboard.writeText(message.content);
    }
  }
</script>

<section class="workbench chat-workbench single-chat-workbench" aria-label="Chat workspace">
  <section class="chat-panel">
    <div class="chat-toolbar">
      <label>
        Agent
        <select bind:value={selectedAgentId}>
          <option value="">No agent</option>
          {#each agents as agent (agent.id)}
            <option value={agent.id}>{agent.name}</option>
          {/each}
        </select>
      </label>
      <ModelPicker
        bind:selectedModelId={selectedModel}
        bind:selectedProviderId
        compact
        label="Chat model"
        models={providerModels}
        {providers}
        role="chat"
      />
      <button on:click={() => onRefreshModels(selectedProviderId)} type="button">Refresh catalog</button>
    </div>

    <div class="chat-context-strip">
      <StatusPill status={activeProvider?.name ?? 'No provider'} tone={activeProvider ? 'accent' : 'warning'} />
      <StatusPill status={activeAgent?.name ?? 'No agent'} tone={activeAgent ? 'success' : 'neutral'} />
      {#if activeRunId}
        <span class="streaming-indicator" aria-live="polite">Streaming</span>
      {/if}
      {#if approvedTools > 0}
        <StatusPill status={`${approvedTools} tools used`} tone="accent" />
      {/if}
    </div>

    <ConversationSummaryPanel
      conversation={selectedConversation}
      onClear={onClearSummary}
      onRegenerate={onRegenerateSummary}
    />

    <div class="message-list" aria-live="polite">
      {#if messages.length === 0}
        <EmptyState
          description="Pick a provider, choose an agent if needed, and send the first message."
          title={strings.chat.noMessages}
        />
      {:else}
        {#each messages as message (message.id)}
          <article class={`message ${message.role}`}>
            <header>
              <strong>{message.role}</strong>
              <span>{new Date(message.created_at).toLocaleString()}</span>
              {#if message.model}
                <span>{message.model}</span>
              {/if}
            </header>
            <!-- eslint-disable-next-line svelte/no-at-html-tags -->
            <div class="markdown-body">{@html renderMarkdown(message.content)}</div>
            {#if message.total_tokens}
              <small>{message.total_tokens} tokens</small>
            {/if}
            {#if message.content}
              <div class="message-actions">
                <Dropdown label="Actions">
                  <button on:click={() => copyMessage(message)} type="button">Copy</button>
                  {#if message.role === 'user'}
                    <button on:click={() => onSelectReplySource(message)} type="button">{strings.chat.draftReply}</button>
                  {/if}
                  <button on:click={() => onRememberMessage(message)} type="button">{strings.chat.remember}</button>
                  <button on:click={() => (detailsMessageId = detailsMessageId === message.id ? '' : message.id)} type="button">
                    View details
                  </button>
                  {#if message.role === 'assistant'}
                    <button on:click={() => (feedbackPanelMessageId = feedbackPanelMessageId === message.id ? '' : message.id)} type="button">
                      Report response
                    </button>
                    <button on:click={() => onRegenerateWithFeedback(message)} type="button">{strings.chat.regenerate}</button>
                    {#if feedbackByMessage[message.id]}
                      <button on:click={() => onClearFeedback(message.id)} type="button">{strings.chat.clearFeedback}</button>
                    {/if}
                  {/if}
                </Dropdown>
              </div>
            {/if}
            {#if feedbackPanelMessageId === message.id}
              <div class="message-feedback-panel">
                <select aria-label="Negative feedback reason" bind:value={negativeFeedbackReason}>
                  <option value="Incorrect information">Incorrect information</option>
                  <option value="Too long">Too long</option>
                  <option value="Too technical">Too technical</option>
                  <option value="Did not follow instructions">Did not follow instructions</option>
                  <option value="Inappropriate tone">Inappropriate tone</option>
                  <option value="Invented information">Invented information</option>
                  <option value="Ignored memories">Ignored memories</option>
                  <option value="Other">Other</option>
                </select>
                <button class:active={feedbackByMessage[message.id]?.rating === 'positive'} on:click={() => onSubmitFeedback(message, 'positive')} type="button">
                  Helpful
                </button>
                <button class:active={feedbackByMessage[message.id]?.rating === 'negative'} on:click={() => onSubmitFeedback(message, 'negative')} type="button">
                  Needs work
                </button>
              </div>
            {/if}
            {#if detailsMessageId === message.id}
              <dl class="message-details">
                <div>
                  <dt>Provider</dt>
                  <dd>{message.provider_id || 'Not recorded'}</dd>
                </div>
                <div>
                  <dt>Model</dt>
                  <dd>{message.model || 'Not recorded'}</dd>
                </div>
                <div>
                  <dt>Created</dt>
                  <dd>{new Date(message.created_at).toLocaleString()}</dd>
                </div>
                <div>
                  <dt>Tokens</dt>
                  <dd>{message.total_tokens ?? 'Not returned'}</dd>
                </div>
                <div>
                  <dt>Feedback</dt>
                  <dd>{feedbackByMessage[message.id]?.rating ?? 'None'}</dd>
                </div>
              </dl>
            {/if}
          </article>
        {/each}
      {/if}
    </div>

    {#if selectedReplySourceId}
      <section class="reply-panel" aria-labelledby="reply-draft-title">
        <div class="panel-heading">
          <div>
            <p class="eyebrow">Reply intent</p>
            <h2 id="reply-draft-title">{strings.replies.title}</h2>
          </div>
          <button on:click={() => (selectedReplySourceId = '')} type="button">Close</button>
        </div>
        <label>
          Preset
          <select bind:value={selectedReplyPresetId}>
            {#each replyPresets.filter((preset) => preset.active) as preset (preset.id)}
              <option value={preset.id}>{preset.name}</option>
            {/each}
          </select>
        </label>
        <label>
          Custom instruction
          <textarea bind:value={replyCustomInstruction} placeholder="Optional direction for this draft"></textarea>
        </label>
        <div class="message-actions">
          <button on:click={onGenerateReplyDraft} type="button">{strings.replies.generate}</button>
          {#if replyDraft}
            <button on:click={onInsertReplyDraft} type="button">{strings.replies.insert}</button>
          {/if}
        </div>
        {#if replyDraft}
          <textarea bind:value={replyDraft} aria-label="Generated reply draft"></textarea>
        {/if}
      </section>
    {/if}

    {#if toolCards.length > 0}
      <section class="tool-panel" aria-label="Tool calls">
        {#each toolCards as tool (tool.id)}
          <article>
            <strong>{tool.name}</strong>
            <span>{tool.state}</span>
            {#if tool.result}
              <p>{tool.result}</p>
            {/if}
          </article>
        {/each}
      </section>
    {/if}

    <PendingToolApprovals
      toolCalls={pendingToolApprovals}
      onApprove={onApproveToolCall}
      onDeny={onDenyToolCall}
      onRefresh={onRefreshPendingToolApprovals}
    />

    {#if runMemories.length > 0}
      <details class="memory-panel" open>
        <summary>{strings.chat.memoriesUsed}</summary>
        {#each runMemories as memory (memory.id)}
          <article>
            <strong>{memory.title}</strong>
            <p>{memory.content}</p>
          </article>
        {/each}
      </details>
    {/if}

    <form class="composer" on:submit|preventDefault={onSendMessage}>
      <textarea bind:value={composer} placeholder={strings.chat.composerPlaceholder}></textarea>
      <div>
        {#if activeRunId}
          <button on:click={onStopGeneration} type="button">{strings.chat.stop}</button>
        {/if}
        <button disabled={submitting || !composer.trim()} type="submit">{strings.chat.send}</button>
      </div>
    </form>
  </section>
</section>

<style>
  .single-chat-workbench {
    grid-template-columns: minmax(0, 1fr);
  }

  .chat-toolbar {
    align-items: end;
  }

  .message-actions {
    justify-content: flex-end;
  }

  .message-actions :global(.ui-dropdown) {
    position: relative;
  }

  .message-actions :global(.ui-dropdown-panel) {
    position: absolute;
    right: 0;
    z-index: 15;
    display: grid;
    min-width: 190px;
    gap: var(--space-1);
    border: 1px solid var(--color-border-muted);
    border-radius: var(--radius-md);
    background: var(--color-panel-elevated);
    box-shadow: var(--shadow-soft);
    padding: var(--space-2);
  }

  .message-actions :global(.ui-dropdown-panel button) {
    justify-content: flex-start;
    background: transparent;
    text-align: left;
    transform: none;
  }

  .message-feedback-panel,
  .message-details {
    display: grid;
    gap: var(--space-2);
    margin-top: var(--space-3);
    border: 1px solid var(--color-border-muted);
    border-radius: var(--radius-md);
    background: rgba(255, 255, 255, 0.025);
    padding: var(--space-3);
  }

  .message-feedback-panel {
    grid-template-columns: minmax(180px, 1fr) auto auto;
    align-items: center;
  }

  .message-details {
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  }

  .message-details div {
    display: grid;
    gap: 2px;
  }

  .message-details dt {
    color: var(--color-subtle);
    font-size: var(--font-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }

  .message-details dd {
    margin: 0;
    color: var(--color-text-soft);
    overflow-wrap: anywhere;
  }

  @media (max-width: 720px) {
    .message-feedback-panel {
      grid-template-columns: 1fr;
    }
  }
</style>
