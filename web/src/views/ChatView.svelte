<script lang="ts">
  import PendingToolApprovals from '../components/chat/PendingToolApprovals.svelte';
  import Icon from '../components/common/Icon.svelte';
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
  export let onRegenerateSummary: () => void | Promise<void>;
  export let onClearSummary: () => void | Promise<void>;
  export let onSendMessage: () => void | Promise<void>;
  export let onStopGeneration: () => void | Promise<void>;
  export let onRememberMessage: (message: Message) => void | Promise<void>;
  export let onEditMessage: (message: Message) => void | Promise<void>;
  export let onSelectReplySource: (message: Message) => void;
  export let onSubmitFeedback: (message: Message, rating: 'positive' | 'negative') => void | Promise<void>;
  export let onClearFeedback: (messageId: string) => void | Promise<void>;
  export let onRegenerateWithFeedback: (message: Message, instruction: string | undefined) => void | Promise<void>;
  export let onGenerateReplyDraft: () => void | Promise<void>;
  export let onInsertReplyDraft: () => void;
  export let onRefreshPendingToolApprovals: () => void | Promise<void>;
  export let onApproveToolCall: (
    toolCall: ToolCall,
    decision: 'approve_once' | 'approve_conversation' | 'allow_agent'
  ) => void | Promise<void>;
  export let onDenyToolCall: (toolCall: ToolCall, decision: 'deny' | 'deny_disable_tool') => void | Promise<void>;

  let composerElement: HTMLTextAreaElement;
  let summaryOpen = false;
  let openMenuMessageId = '';
  let feedbackPanelMessageId = '';
  let detailsMessageId = '';

  $: activeAgent = agents.find((agent) => agent.id === selectedAgentId);
  $: activeProvider = providers.find((provider) => provider.id === selectedProviderId);
  $: activeModel = providerModels.find((model) => model.provider_id === selectedProviderId && model.model_id === selectedModel);
  $: title = selectedConversation?.title ?? strings.chat.newConversation;
  $: messageCountLabel = `${messages.length} ${messages.length === 1 ? 'message' : 'messages'}`;
  $: activeModelLabel = activeModel?.display_name || selectedModel || 'Select model';

  async function copyMessage(message: Message): Promise<void> {
    if (window.navigator.clipboard) {
      await window.navigator.clipboard.writeText(message.content);
    }
    openMenuMessageId = '';
  }

  function formatTime(value: string): string {
    return new Date(value).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  function resizeComposer(): void {
    if (!composerElement) return;
    composerElement.style.height = 'auto';
    composerElement.style.height = `${Math.min(composerElement.scrollHeight, 180)}px`;
  }

  async function handleComposerKeydown(event: KeyboardEvent): Promise<void> {
    if (event.key === 'Escape') {
      openMenuMessageId = '';
      summaryOpen = false;
      return;
    }
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      await onSendMessage();
      requestAnimationFrame(resizeComposer);
    }
  }

  function handleCanvasKeydown(event: KeyboardEvent): void {
    if (event.key !== 'Escape') return;
    openMenuMessageId = '';
    detailsMessageId = '';
    feedbackPanelMessageId = '';
    summaryOpen = false;
  }

  function switchMode(mode: 'chat' | 'agent'): void {
    if (mode === 'chat') {
      selectedAgentId = '';
      return;
    }
    selectedAgentId = selectedAgentId || agents[0]?.id || '';
  }
</script>

<svelte:window on:keydown={handleCanvasKeydown} />

<section class="chat-canvas" aria-label="Chat workspace">
  <h1 class="visually-hidden">Chat</h1>

  <div class="conversation-meta">
    <button
      aria-expanded={summaryOpen}
      class:has-summary={Boolean(selectedConversation?.summary)}
      class="conversation-meta-button"
      on:click={() => (summaryOpen = !summaryOpen)}
      type="button"
    >
      <span>{title}</span>
      <small>{messageCountLabel}</small>
      {#if selectedConversation?.summary}
        <span aria-label="Conversation summary available" class="summary-indicator"></span>
      {/if}
      <Icon name="chevron-down" size={12} />
    </button>

    {#if summaryOpen}
      <div class="summary-popover" role="dialog" aria-label="Conversation summary">
        <header>
          <strong>Conversation summary</strong>
          <span>{selectedConversation?.summary_status || 'idle'}</span>
        </header>
        {#if selectedConversation?.summary}
          <p>{selectedConversation.summary}</p>
          {#if selectedConversation.summary_updated_at}
            <small>Updated {new Date(selectedConversation.summary_updated_at).toLocaleString()}</small>
          {/if}
        {:else if selectedConversation?.summary_error}
          <p class="danger-text">{selectedConversation.summary_error}</p>
        {:else}
          <p>No summary stored for this conversation.</p>
        {/if}
        <div class="summary-actions">
          <button on:click={onRegenerateSummary} type="button">
            <Icon name="refresh" size={13} /> Regenerate summary
          </button>
          {#if selectedConversation?.summary}
            <button on:click={onClearSummary} type="button">
              <Icon name="trash" size={13} /> Clear summary
            </button>
          {/if}
        </div>
      </div>
    {/if}
  </div>

  <div class="message-stream" aria-live="polite">
    {#if messages.length === 0}
      <div class="chat-empty-state">
        <Icon name="spark" size={18} />
        <p>{strings.chat.noMessages}</p>
      </div>
    {:else}
      {#each messages as message, index (message.id)}
        <article class:assistant={message.role === 'assistant'} class:user={message.role === 'user'} class="chat-message">
          <header class="message-header">
            <strong>{message.role === 'user' ? 'You' : message.model || activeModelLabel}</strong>
            <span>{formatTime(message.created_at)}</span>
            {#if message.role === 'assistant' && message.model}
              <code title={message.model}>{message.model}</code>
            {/if}
          </header>

          <!-- eslint-disable-next-line svelte/no-at-html-tags -->
          <div class="markdown-body">{@html renderMarkdown(message.content)}</div>

          {#if message.content}
            <footer class="message-footer">
              <div class="message-metrics">
                {#if message.total_tokens}
                  <span>{message.total_tokens} tok</span>
                {/if}
                {#if message.role === 'assistant' && index === messages.length - 1 && runMemories.length > 0}
                  <span>{runMemories.length} memories</span>
                {/if}
                {#if message.role === 'assistant' && toolCards.length > 0 && index === messages.length - 1}
                  <span>{toolCards.length} tools</span>
                {/if}
                {#if feedbackByMessage[message.id]}
                  <span>{feedbackByMessage[message.id].rating}</span>
                {/if}
              </div>
              <div class="message-action-strip">
                <button aria-label="Copy message" on:click={() => copyMessage(message)} type="button">
                  <Icon name="copy" size={13} />
                </button>
                <button
                  aria-expanded={detailsMessageId === message.id}
                  aria-label="View message details"
                  on:click={() => (detailsMessageId = detailsMessageId === message.id ? '' : message.id)}
                  type="button"
                >
                  <Icon name="details" size={13} />
                </button>
                <button
                  aria-expanded={openMenuMessageId === message.id}
                  aria-label="Message menu"
                  on:click={() => (openMenuMessageId = openMenuMessageId === message.id ? '' : message.id)}
                  type="button"
                >
                  <Icon name="kebab" size={13} />
                </button>
              </div>
            </footer>
          {/if}

          {#if openMenuMessageId === message.id}
            <div class="message-menu" role="menu">
              {#if message.role === 'user'}
                <button on:click={() => { openMenuMessageId = ''; onEditMessage(message); }} type="button">
                  <Icon name="edit" size={13} /> Edit
                </button>
                <button on:click={() => { openMenuMessageId = ''; onSelectReplySource(message); }} type="button">
                  <Icon name="chat" size={13} /> Use as reply source
                </button>
              {:else}
                <button on:click={() => { openMenuMessageId = ''; onRegenerateWithFeedback(message, undefined); }} type="button">
                  <Icon name="refresh" size={13} /> Regenerate from here
                </button>
                <button
                  on:click={() => {
                    openMenuMessageId = '';
                    onRegenerateWithFeedback(message, 'Rewrite the response to be shorter while preserving the answer.');
                  }}
                  type="button"
                >
                  <Icon name="spark" size={13} /> Rewrite shorter
                </button>
                <button
                  on:click={() => {
                    openMenuMessageId = '';
                    feedbackPanelMessageId = feedbackPanelMessageId === message.id ? '' : message.id;
                  }}
                  type="button"
                >
                  <Icon name="details" size={13} /> Report response
                </button>
                {#if feedbackByMessage[message.id]}
                  <button on:click={() => { openMenuMessageId = ''; onClearFeedback(message.id); }} type="button">
                    <Icon name="close" size={13} /> Clear report
                  </button>
                {/if}
              {/if}
              <button on:click={() => { openMenuMessageId = ''; onRememberMessage(message); }} type="button">
                <Icon name="brain" size={13} /> Create memory
              </button>
              <button on:click={() => { openMenuMessageId = ''; detailsMessageId = message.id; }} type="button">
                <Icon name="details" size={13} /> View details
              </button>
            </div>
          {/if}

          {#if feedbackPanelMessageId === message.id}
            <div class="message-feedback-panel" role="group" aria-label="Report response">
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
                Useful
              </button>
              <button class:active={feedbackByMessage[message.id]?.rating === 'negative'} on:click={() => onSubmitFeedback(message, 'negative')} type="button">
                Needs work
              </button>
            </div>
          {/if}

          {#if detailsMessageId === message.id}
            <dl class="message-details-panel">
              <div>
                <dt>Provider</dt>
                <dd>{providers.find((provider) => provider.id === message.provider_id)?.name || message.provider_id || 'Not recorded'}</dd>
              </div>
              <div>
                <dt>Full model ID</dt>
                <dd>{message.model || 'Not recorded'}</dd>
              </div>
              <div>
                <dt>Total tokens</dt>
                <dd>{message.total_tokens ?? 'Not returned'}</dd>
              </div>
              <div>
                <dt>Created</dt>
                <dd>{new Date(message.created_at).toLocaleString()}</dd>
              </div>
              <div>
                <dt>Memories used</dt>
                <dd>{message.role === 'assistant' && index === messages.length - 1 ? runMemories.length : 0}</dd>
              </div>
              <div>
                <dt>Tool calls</dt>
                <dd>{message.role === 'assistant' && index === messages.length - 1 ? toolCards.length : 0}</dd>
              </div>
              <div>
                <dt>Branch</dt>
                <dd>{message.branch_id || 'main'}</dd>
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

    {#if toolCards.length > 0}
      <section class="inline-tool-calls" aria-label="Tool calls">
        {#each toolCards as tool (tool.id)}
          <article>
            <Icon name={tool.state === 'completed' || tool.state === 'succeeded' ? 'check' : 'tools'} size={13} />
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
      <details class="inline-memory-strip">
        <summary>{strings.chat.memoriesUsed}</summary>
        {#each runMemories as memory (memory.id)}
          <article>
            <strong>{memory.title}</strong>
            <p>{memory.content}</p>
          </article>
        {/each}
      </details>
    {/if}
  </div>

  {#if selectedReplySourceId}
    <section class="reply-draft-popover" aria-labelledby="reply-draft-title">
      <header>
        <h2 id="reply-draft-title">{strings.replies.title}</h2>
        <button aria-label="Close reply draft" on:click={() => (selectedReplySourceId = '')} type="button">
          <Icon name="close" size={13} />
        </button>
      </header>
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
      <div class="reply-actions">
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

  <form class="workspace-composer" on:submit|preventDefault={onSendMessage}>
    <div class="composer-topline">
      <textarea
        bind:this={composerElement}
        bind:value={composer}
        on:input={resizeComposer}
        on:keydown={handleComposerKeydown}
        placeholder={strings.chat.composerPlaceholder}
        rows="1"
      ></textarea>
    </div>
    <div class="composer-controls">
      <div class="composer-left-tools">
        <button aria-label="Add attachment" disabled title="Attachments are not configured for this workspace" type="button">
          <Icon name="plus" size={14} />
        </button>
        <button aria-label="Search or tool mode" disabled title="Tool mode is configured from Tools" type="button">
          <Icon name="search" size={14} />
        </button>
        <div class="mode-toggle" role="group" aria-label="Chat mode">
          <button class:active={!selectedAgentId} on:click={() => switchMode('chat')} type="button">Chat</button>
          <button class:active={Boolean(selectedAgentId)} on:click={() => switchMode('agent')} type="button">Agent</button>
        </div>
      </div>

      <div class="composer-model">
        <ModelPicker
          bind:selectedModelId={selectedModel}
          bind:selectedProviderId
          compact
          label="Chat model"
          models={providerModels}
          {providers}
          role="chat"
        />
        {#if agents.length > 0}
          <label class="agent-select">
            <span class="visually-hidden">Agent</span>
            <select bind:value={selectedAgentId}>
              <option value="">No agent</option>
              {#each agents as agent (agent.id)}
                <option value={agent.id}>{agent.name}</option>
              {/each}
            </select>
          </label>
        {/if}
      </div>

      <div class="composer-send">
        {#if activeRunId}
          <button aria-label={strings.chat.stop} on:click={onStopGeneration} type="button">
            <Icon name="stop" size={15} />
          </button>
        {:else}
          <button aria-label={strings.chat.send} disabled={submitting || !composer.trim()} type="submit">
            <Icon name="send" size={15} />
          </button>
        {/if}
      </div>
    </div>
    <div class="composer-status">
      <span title={selectedModel}>{activeProvider?.name ?? 'No provider'} · {activeModelLabel}</span>
      {#if activeAgent}
        <span>Agent: {activeAgent.name}</span>
      {/if}
      <kbd>Enter</kbd>
    </div>
  </form>
</section>
