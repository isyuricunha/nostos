<script lang="ts">
  import ConversationSummaryPanel from '../components/chat/ConversationSummaryPanel.svelte';
  import PendingToolApprovals from '../components/chat/PendingToolApprovals.svelte';
  import EmptyState from '../components/common/EmptyState.svelte';
  import StatusPill from '../components/common/StatusPill.svelte';
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

  export let conversations: Conversation[] = [];
  export let messages: Message[] = [];
  export let providers: Provider[] = [];
  export let providerModels: ProviderModel[] = [];
  export let agents: Agent[] = [];
  export let selectedConversation: Conversation | undefined;
  export let selectedConversationId = '';
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
  export let onCreateConversation: () => void | Promise<void>;
  export let onSelectConversation: (conversationId: string) => void | Promise<void>;
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

  let conversationSearch = '';

  $: visibleConversations = conversations.filter((conversation) =>
    conversation.title.toLowerCase().includes(conversationSearch.trim().toLowerCase())
  );
  $: activeAgent = agents.find((agent) => agent.id === selectedAgentId);
  $: activeProvider = providers.find((provider) => provider.id === selectedProviderId);
  $: approvedTools = toolCards.filter((tool) => tool.state === 'succeeded').length;
</script>

<section class="workbench chat-workbench" aria-label="Chat workspace">
  <aside class="list-panel conversation-rail">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">Threads</p>
        <h2>Conversations</h2>
      </div>
      <button on:click={onCreateConversation} type="button">{strings.chat.newConversation}</button>
    </div>
    <label class="search-field">
      <span>Search conversations</span>
      <input bind:value={conversationSearch} placeholder="Search by title" />
    </label>
    {#if visibleConversations.length === 0}
      <EmptyState description="Start a new thread to build persistent context." title={strings.chat.noConversations} />
    {:else}
      <div class="conversation-list">
        {#each visibleConversations as conversation (conversation.id)}
          <button
            class:active={selectedConversationId === conversation.id}
            class="list-item conversation-item"
            on:click={() => onSelectConversation(conversation.id)}
            type="button"
          >
            <strong>{conversation.title}</strong>
            <span>{new Date(conversation.updated_at).toLocaleString()}</span>
            {#if conversation.summary_status}
              <small>{conversation.summary_status}</small>
            {/if}
          </button>
        {/each}
      </div>
    {/if}
  </aside>

  <section class="chat-panel">
    <div class="chat-toolbar">
      <label>
        Provider
        <select bind:value={selectedProviderId} on:change={() => onRefreshModels(selectedProviderId)}>
          <option value="">Select provider</option>
          {#each providers as provider (provider.id)}
            <option value={provider.id}>{provider.name}</option>
          {/each}
        </select>
      </label>
      <label>
        Agent
        <select bind:value={selectedAgentId}>
          <option value="">No agent</option>
          {#each agents as agent (agent.id)}
            <option value={agent.id}>{agent.name}</option>
          {/each}
        </select>
      </label>
      <label>
        Model
        <select bind:value={selectedModel}>
          <option value="">Manual model</option>
          {#each providerModels as model (model.id)}
            <option value={model.model_id}>{model.model_id}</option>
          {/each}
        </select>
      </label>
      <label>
        Model ID
        <input bind:value={selectedModel} aria-label="Model" placeholder="Model ID" />
      </label>
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
                <button on:click={() => onRememberMessage(message)} type="button">{strings.chat.remember}</button>
                <button on:click={() => onSelectReplySource(message)} type="button">{strings.chat.draftReply}</button>
                {#if message.role === 'assistant'}
                  <button
                    class:active={feedbackByMessage[message.id]?.rating === 'positive'}
                    on:click={() => onSubmitFeedback(message, 'positive')}
                    type="button"
                  >
                    {strings.chat.feedbackUp}
                  </button>
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
                  <button
                    class:active={feedbackByMessage[message.id]?.rating === 'negative'}
                    on:click={() => onSubmitFeedback(message, 'negative')}
                    type="button"
                  >
                    {strings.chat.feedbackDown}
                  </button>
                  {#if feedbackByMessage[message.id]}
                    <button on:click={() => onClearFeedback(message.id)} type="button">{strings.chat.clearFeedback}</button>
                    <button on:click={() => onRegenerateWithFeedback(message)} type="button">{strings.chat.regenerate}</button>
                  {/if}
                {/if}
              </div>
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
