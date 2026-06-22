<script lang="ts">
  import { onMount } from 'svelte';
  import DOMPurify from 'dompurify';
  import { marked } from 'marked';
  import ConversationSummaryPanel from './components/chat/ConversationSummaryPanel.svelte';
  import PendingToolApprovals from './components/chat/PendingToolApprovals.svelte';
  import { deleteJSON, getJSON, postJSON, postStream, putJSON } from './lib/api';
  import { formatHeaderText, parseHeaderText } from './lib/provider-form';
  import type {
    Agent,
    AgentResponse,
    AgentsResponse,
    Conversation,
    ConversationResponse,
    ConversationsResponse,
    FeedbackListResponse,
    FeedbackResponse,
    FeedbackStats,
    FeedbackStatsResponse,
    MCPServer,
    MCPServerResponse,
    MCPServersResponse,
    MCPTool,
    MCPToolsResponse,
    MemoriesResponse,
    Memory,
    MemoryResponse,
    MemorySnippet,
    Message,
    MessageFeedback,
    MessagesResponse,
    ModelsResponse,
    Provider,
    ProviderModel,
    ProviderResponse,
    ProvidersResponse,
    ReadyStatus,
    ReplyDraftResponse,
    ReplyPreset,
    ReplyPresetResponse,
    ReplyPresetsResponse,
    Session,
    SessionsResponse,
    SetupStatus,
    TaskRecord,
    TaskResponse,
    TaskRun,
    TaskRunEvent,
    TaskRunRecordResponse,
    TaskRunResponse,
    TaskRunsResponse,
    TasksResponse,
    ToolApprovalsResponse,
    ToolCall,
    ToolCallResponse,
    ToolCard,
    User,
    UserResponse
  } from './lib/types';
  import { strings } from './strings';

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
  let providers: Provider[] = [];
  let providerModels: ProviderModel[] = [];
  let conversations: Conversation[] = [];
  let messages: Message[] = [];
  let agents: Agent[] = [];
  let memories: Memory[] = [];
  let mcpServers: MCPServer[] = [];
  let mcpTools: MCPTool[] = [];
  let taskRecords: TaskRecord[] = [];
  let taskRuns: TaskRun[] = [];
  let taskRunEvents: TaskRunEvent[] = [];
  let feedbackByMessage: Record<string, MessageFeedback> = {};
  let feedbackStats: FeedbackStats = { positive: 0, negative: 0 };
  let replyPresets: ReplyPreset[] = [];
  let selectedReplySourceId = '';
  let selectedReplyPresetId = '';
  let replyCustomInstruction = '';
  let replyDraft = '';
  let runMemories: MemorySnippet[] = [];
  let toolCards: ToolCard[] = [];
  let pendingToolApprovals: ToolCall[] = [];
  let selectedConversationId = '';
  let selectedAgentId = '';
  let selectedProviderId = '';
  let selectedModel = '';
  let composer = '';
  let activeRunId = '';
  let activeView: string = strings.nav.chat;
  let loading = true;
  let submitting = false;
  let notice = '';
  let errorMessage = '';

  $: selectedConversation = conversations.find((conversation) => conversation.id === selectedConversationId);

  let setupEmail = '';
  let setupDisplayName = '';
  let setupPassword = '';
  let setupConfirmPassword = '';
  let loginEmail = '';
  let loginPassword = '';
  let editingProviderId = '';
  let providerName = '';
  let providerBaseUrl = '';
  let providerApiKey = '';
  let providerApiKeyEnvRef = '';
  let providerOrganization = '';
  let providerProject = '';
  let providerCustomHeaders = '';
  let providerDefaultModel = '';
  let providerFallbackModel = '';
  let providerTimeoutMS = 60000;
  let providerEnabled = true;
  let editingAgentId = '';
  let agentName = '';
  let agentDescription = '';
  let agentAvatar = 'sparkles';
  let agentPrompt = '';
  let agentDefaultProviderId = '';
  let agentDefaultModel = '';
  let agentFallbackModel = '';
  let agentTemperature = 0.7;
  let agentMaxToolIterations = 8;
  let agentMemoryMode = 'pinned_only';
  let agentToolPermissionDefault = 'ask';
  let agentActive = true;
  let editingMemoryId = '';
  let memoryTitle = '';
  let memoryContent = '';
  let memoryTags = '';
  let memoryScope = 'global';
  let memoryImportance = 70;
  let memoryPinned = true;
  let memoryActive = true;
  let editingMCPServerId = '';
  let mcpName = '';
  let mcpDescription = '';
  let mcpTransport = 'http';
  let mcpHttpUrl = '';
  let mcpHttpHeaders = '';
  let mcpCommand = '';
  let mcpArguments = '';
  let mcpWorkingDirectory = '';
  let mcpEnvironment = '';
  let mcpAuthorization = '';
  let mcpStartupTimeoutMS = 10000;
  let mcpRequestTimeoutMS = 30000;
  let mcpEnabled = true;
  let editingTaskId = '';
  let taskName = '';
  let taskDescription = '';
  let taskPrompt = '';
  let taskType = 'agent';
  let taskState = 'enabled';
  let taskAgentId = '';
  let taskProviderId = '';
  let taskModel = '';
  let taskScheduleMode = 'manual';
  let taskCronExpression = '';
  let taskIntervalSeconds = 3600;
  let taskRunAt = '';
  let taskToolPolicy = 'use_preapproved_tools_only';
  let taskMaxRetries = 3;
  let taskTimeoutMS = 600000;
  let taskConcurrencyPolicy = 'skip';
  let negativeFeedbackReason = 'Incorrect information';
  let replyPresetName = '';
  let replyPresetInstruction = '';
  let replyPresetDescription = '';

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
        await refreshWorkspaceData();
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
    sessions = response.sessions ?? [];
  }

  async function refreshDiagnostics(): Promise<void> {
    status = await getJSON<ReadyStatus>('/api/v1/diagnostics');
  }

  async function refreshWorkspaceData(): Promise<void> {
    await Promise.all([
      refreshSessions(),
      refreshDiagnostics(),
      refreshProviders(),
      refreshAgents(),
      refreshMemories(),
      refreshMCP(),
      refreshTasks(),
      refreshReplyPresets(),
      refreshFeedbackStats(),
      refreshPendingToolApprovals(),
      refreshConversations()
    ]);
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
      await refreshWorkspaceData();
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
      await refreshWorkspaceData();
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

  async function refreshProviders(): Promise<void> {
    const response = await getJSON<ProvidersResponse>('/api/v1/providers');
    providers = response.providers ?? [];
    if (!selectedProviderId && providers.length > 0) {
      selectedProviderId = providers[0].id;
      selectedModel = providers[0].default_model ?? '';
      await refreshModels(selectedProviderId);
    }
  }

  async function refreshModels(providerId = selectedProviderId): Promise<void> {
    if (!providerId) {
      providerModels = [];
      return;
    }
    const response = await getJSON<ModelsResponse>(`/api/v1/providers/${providerId}/models`);
    providerModels = response.models ?? [];
    if (!selectedModel && providerModels.length > 0) {
      selectedModel = providerModels[0].model_id;
    }
  }

  async function createProvider(): Promise<void> {
    submitting = true;
    errorMessage = '';
    notice = '';
    try {
      const customHeaders = parseHeaderText(providerCustomHeaders);
      const payload = {
        name: providerName,
        base_url: providerBaseUrl,
        api_key: providerApiKey || undefined,
        api_key_env_ref: providerApiKeyEnvRef,
        organization_header: providerOrganization,
        project_header: providerProject,
        custom_headers: customHeaders,
        enabled: providerEnabled,
        request_timeout_ms: providerTimeoutMS,
        default_model: providerDefaultModel,
        fallback_model: providerFallbackModel
      };
      const providerBeingEdited = editingProviderId;
      const response = providerBeingEdited
        ? await putJSON<ProviderResponse>(`/api/v1/providers/${providerBeingEdited}`, payload)
        : await postJSON<ProviderResponse>('/api/v1/providers', payload);
      resetProviderForm();
      selectedProviderId = response.provider.id;
      selectedModel = response.provider.default_model ?? '';
      await refreshProviders();
      notice = providerBeingEdited ? 'Provider updated.' : 'Provider saved.';
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      submitting = false;
    }
  }

  function editProvider(provider: Provider): void {
    editingProviderId = provider.id;
    providerName = provider.name;
    providerBaseUrl = provider.base_url;
    providerApiKey = '';
    providerApiKeyEnvRef = provider.api_key_env_ref ?? '';
    providerOrganization = provider.organization_header ?? '';
    providerProject = provider.project_header ?? '';
    providerCustomHeaders = formatHeaderText(provider.custom_headers);
    providerDefaultModel = provider.default_model ?? '';
    providerFallbackModel = provider.fallback_model ?? '';
    providerTimeoutMS = provider.request_timeout_ms;
    providerEnabled = provider.enabled;
  }

  function resetProviderForm(): void {
    editingProviderId = '';
    providerName = '';
    providerBaseUrl = '';
    providerApiKey = '';
    providerApiKeyEnvRef = '';
    providerOrganization = '';
    providerProject = '';
    providerCustomHeaders = '';
    providerDefaultModel = '';
    providerFallbackModel = '';
    providerTimeoutMS = 60000;
    providerEnabled = true;
  }

  async function deleteProvider(providerId: string): Promise<void> {
    if (!confirm('Delete this provider? Conversations that used it keep their stored messages.')) {
      return;
    }
    submitting = true;
    errorMessage = '';
    try {
      await deleteJSON<{ ok: boolean }>(`/api/v1/providers/${providerId}`);
      if (selectedProviderId === providerId) {
        selectedProviderId = '';
        selectedModel = '';
      }
      if (editingProviderId === providerId) {
        resetProviderForm();
      }
      await refreshProviders();
      notice = 'Provider deleted.';
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      submitting = false;
    }
  }

  async function testProvider(providerId: string): Promise<void> {
    submitting = true;
    errorMessage = '';
    try {
      await postJSON<{ ok: boolean }>(`/api/v1/providers/${providerId}/test`);
      await refreshProviders();
      notice = 'Provider connection succeeded.';
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      submitting = false;
    }
  }

  async function refreshProviderModels(providerId: string): Promise<void> {
    submitting = true;
    errorMessage = '';
    try {
      const response = await postJSON<ModelsResponse>(`/api/v1/providers/${providerId}/models/refresh`);
      providerModels = response.models ?? [];
      selectedProviderId = providerId;
      if (providerModels.length > 0) {
        selectedModel = providerModels[0].model_id;
      }
      await refreshProviders();
      notice = 'Models refreshed.';
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      submitting = false;
    }
  }

  async function refreshConversations(): Promise<void> {
    const response = await getJSON<ConversationsResponse>('/api/v1/conversations');
    conversations = response.conversations ?? [];
    if (!selectedConversationId && conversations.length > 0) {
      await selectConversation(conversations[0].id);
    }
  }

  async function createConversation(): Promise<void> {
    const response = await postJSON<ConversationResponse>('/api/v1/conversations', {
      title: strings.chat.newConversation,
      agent_id: selectedAgentId,
      provider_id: selectedProviderId,
      model: selectedModel
    });
    conversations = [response.conversation, ...conversations];
    await selectConversation(response.conversation.id);
  }

  async function selectConversation(conversationId: string): Promise<void> {
    selectedConversationId = conversationId;
    const response = await getJSON<MessagesResponse>(`/api/v1/conversations/${conversationId}/messages`);
    messages = response.messages ?? [];
    await refreshFeedback(conversationId);
    const conversation = conversations.find((item) => item.id === conversationId);
    if (conversation) {
      selectedAgentId = conversation.agent_id || selectedAgentId;
      selectedProviderId = conversation.provider_id || selectedProviderId;
      selectedModel = conversation.model || selectedModel;
      await refreshModels(selectedProviderId);
    }
  }

  async function regenerateSummary(): Promise<void> {
    if (!selectedConversationId) return;
    try {
      const response = await postJSON<{ conversation: Conversation; queued: boolean }>(
        `/api/v1/conversations/${selectedConversationId}/summary/regenerate`
      );
      conversations = conversations.map((conversation) =>
        conversation.id === response.conversation.id ? response.conversation : conversation
      );
      notice = response.queued ? 'Conversation summary regeneration queued.' : 'Conversation summary is already queued.';
    } catch (error) {
      errorMessage = messageFromError(error);
    }
  }

  async function clearSummary(): Promise<void> {
    if (!selectedConversationId || !confirm('Clear this conversation summary?')) return;
    try {
      const response = await deleteJSON<ConversationResponse>(`/api/v1/conversations/${selectedConversationId}/summary`);
      conversations = conversations.map((conversation) =>
        conversation.id === response.conversation.id ? response.conversation : conversation
      );
      notice = 'Conversation summary cleared.';
    } catch (error) {
      errorMessage = messageFromError(error);
    }
  }

  async function sendMessage(): Promise<void> {
    if (!composer.trim()) {
      return;
    }
    if (!selectedProviderId) {
      errorMessage = 'Select a provider before sending a message.';
      return;
    }
    if (!selectedConversationId) {
      await createConversation();
    }
    const content = composer;
    composer = '';
    errorMessage = '';
    await postStream(
      `/api/v1/conversations/${selectedConversationId}/runs`,
      { content, provider_id: selectedProviderId, model: selectedModel },
      handleChatEvent
    );
    activeRunId = '';
    await Promise.all([refreshConversations(), selectConversation(selectedConversationId)]);
  }

  async function stopGeneration(): Promise<void> {
    if (!activeRunId) {
      return;
    }
    await postJSON<{ ok: boolean }>(`/api/v1/chat-runs/${activeRunId}/cancel`);
  }

  async function refreshPendingToolApprovals(): Promise<void> {
    const response = await getJSON<ToolApprovalsResponse>('/api/v1/tool-approvals/pending');
    pendingToolApprovals = response.tool_calls ?? [];
  }

  async function approveToolCall(toolCall: ToolCall, decision: 'approve_once' | 'approve_conversation' | 'allow_agent'): Promise<void> {
    try {
      await postJSON<ToolCallResponse>(`/api/v1/tool-calls/${toolCall.id}/approve`, { decision });
      pendingToolApprovals = pendingToolApprovals.filter((item) => item.id !== toolCall.id);
      await resumeRun(toolCall.chat_run_id);
    } catch (error) {
      errorMessage = messageFromError(error);
    }
  }

  async function denyToolCall(toolCall: ToolCall, decision: 'deny' | 'deny_disable_tool'): Promise<void> {
    try {
      await postJSON<ToolCallResponse>(`/api/v1/tool-calls/${toolCall.id}/deny`, { decision });
      pendingToolApprovals = pendingToolApprovals.filter((item) => item.id !== toolCall.id);
      await resumeRun(toolCall.chat_run_id);
    } catch (error) {
      errorMessage = messageFromError(error);
    }
  }

  async function resumeRun(runId: string): Promise<void> {
    activeRunId = runId;
    await postStream(`/api/v1/chat-runs/${runId}/resume`, {}, handleChatEvent);
    activeRunId = '';
    await Promise.all([
      refreshConversations(),
      selectedConversationId ? selectConversation(selectedConversationId) : Promise.resolve(),
      refreshPendingToolApprovals()
    ]);
  }

  function handleChatEvent(event: string, payload: unknown): void {
    if (event === 'run_started' && isRunStarted(payload)) {
      activeRunId = payload.run.id;
      runMemories = [];
      toolCards = [];
      messages = [...messages, payload.user_message, payload.assistant_message];
    }
    if (event === 'memories_used' && isMemoriesUsed(payload)) {
      runMemories = payload.memories;
    }
    if (event === 'content_delta' && isContentDelta(payload)) {
      const last = messages[messages.length - 1];
      if (last?.role === 'assistant') {
        messages = [...messages.slice(0, -1), { ...last, content: `${last.content}${payload.delta}` }];
      }
    }
    if (event === 'tool_call_ready' && isToolCallReady(payload)) {
      toolCards = payload.tool_calls.map((toolCall) => ({
        id: toolCall.id,
        name: toolCall.function.name,
        state: 'running'
      }));
    }
    if (event === 'tool_approval_required') {
      void refreshPendingToolApprovals();
    }
    if (event === 'tool_result' && isToolResult(payload)) {
      toolCards = [
        ...toolCards.filter((tool) => tool.id !== payload.tool_call_id),
        { id: payload.tool_call_id, name: payload.name, state: 'completed', result: payload.result }
      ];
    }
    if (event === 'run_completed' || event === 'run_failed' || event === 'run_cancelled') {
      activeRunId = '';
    }
  }

  function isRunStarted(payload: unknown): payload is { run: { id: string }; user_message: Message; assistant_message: Message } {
    return typeof payload === 'object' && payload !== null && 'run' in payload && 'user_message' in payload;
  }

  function isContentDelta(payload: unknown): payload is { delta: string } {
    return typeof payload === 'object' && payload !== null && 'delta' in payload;
  }

  function isMemoriesUsed(payload: unknown): payload is { memories: MemorySnippet[] } {
    return typeof payload === 'object' && payload !== null && 'memories' in payload;
  }

  function isToolCallReady(payload: unknown): payload is { tool_calls: { id: string; function: { name: string } }[] } {
    return typeof payload === 'object' && payload !== null && 'tool_calls' in payload;
  }

  function isToolResult(payload: unknown): payload is { tool_call_id: string; name: string; result: string } {
    return typeof payload === 'object' && payload !== null && 'tool_call_id' in payload && 'result' in payload;
  }

  async function refreshAgents(): Promise<void> {
    const response = await getJSON<AgentsResponse>('/api/v1/agents');
    agents = response.agents ?? [];
    if (!selectedAgentId && agents.length > 0) {
      selectedAgentId = agents[0].id;
    }
  }

  async function createAgent(): Promise<void> {
    const payload = {
      name: agentName,
      description: agentDescription,
      avatar: agentAvatar,
      system_prompt: agentPrompt,
      default_provider_id: agentDefaultProviderId,
      default_model: agentDefaultModel,
      fallback_model: agentFallbackModel,
      temperature: agentTemperature,
      max_tool_iterations: agentMaxToolIterations,
      memory_access_mode: agentMemoryMode,
      tool_permission_default: agentToolPermissionDefault,
      active: agentActive
    };
    const agentBeingEdited = editingAgentId;
    const response = agentBeingEdited
      ? await putJSON<AgentResponse>(`/api/v1/agents/${agentBeingEdited}`, payload)
      : await postJSON<AgentResponse>('/api/v1/agents', payload);
    agents = agentBeingEdited
      ? agents.map((agent) => (agent.id === response.agent.id ? response.agent : agent))
      : [response.agent, ...agents];
    selectedAgentId = response.agent.id;
    resetAgentForm();
    notice = agentBeingEdited ? 'Agent updated.' : 'Agent saved.';
  }

  function editAgent(agent: Agent): void {
    editingAgentId = agent.id;
    agentName = agent.name;
    agentDescription = agent.description;
    agentAvatar = agent.avatar;
    agentPrompt = agent.system_prompt;
    agentDefaultProviderId = agent.default_provider_id ?? '';
    agentDefaultModel = agent.default_model ?? '';
    agentFallbackModel = agent.fallback_model ?? '';
    agentTemperature = agent.temperature;
    agentMaxToolIterations = agent.max_tool_iterations;
    agentMemoryMode = agent.memory_access_mode;
    agentToolPermissionDefault = agent.tool_permission_default;
    agentActive = agent.active;
  }

  function resetAgentForm(): void {
    editingAgentId = '';
    agentName = '';
    agentDescription = '';
    agentAvatar = 'sparkles';
    agentPrompt = '';
    agentDefaultProviderId = '';
    agentDefaultModel = '';
    agentFallbackModel = '';
    agentTemperature = 0.7;
    agentMaxToolIterations = 8;
    agentMemoryMode = 'pinned_only';
    agentToolPermissionDefault = 'ask';
    agentActive = true;
  }

  async function duplicateAgent(agentId: string): Promise<void> {
    const response = await postJSON<AgentResponse>(`/api/v1/agents/${agentId}/duplicate`);
    agents = [response.agent, ...agents];
    notice = 'Agent duplicated.';
  }

  async function deleteAgent(agentId: string): Promise<void> {
    if (!confirm('Delete this agent? Existing conversations keep their messages.')) {
      return;
    }
    await deleteJSON<{ ok: boolean }>(`/api/v1/agents/${agentId}`);
    await refreshAgents();
  }

  async function refreshMemories(): Promise<void> {
    const response = await getJSON<MemoriesResponse>('/api/v1/memories');
    memories = response.memories ?? [];
  }

  async function createMemory(): Promise<void> {
    const payload = {
      title: memoryTitle,
      content: memoryContent,
      tags: memoryTags
        .split(',')
        .map((tag) => tag.trim())
        .filter(Boolean),
      scope: memoryScope,
      importance: memoryImportance,
      pinned: memoryPinned,
      active: memoryActive,
      source: 'manual'
    };
    const memoryBeingEdited = editingMemoryId;
    const response = memoryBeingEdited
      ? await putJSON<MemoryResponse>(`/api/v1/memories/${memoryBeingEdited}`, payload)
      : await postJSON<MemoryResponse>('/api/v1/memories', payload);
    memories = memoryBeingEdited
      ? memories.map((memory) => (memory.id === response.memory.id ? response.memory : memory))
      : [response.memory, ...memories];
    resetMemoryForm();
    notice = memoryBeingEdited ? 'Memory updated.' : 'Memory saved.';
  }

  function editMemory(memory: Memory): void {
    editingMemoryId = memory.id;
    memoryTitle = memory.title;
    memoryContent = memory.content;
    memoryTags = memory.tags.join(', ');
    memoryScope = memory.scope;
    memoryImportance = memory.importance;
    memoryPinned = memory.pinned;
    memoryActive = memory.active;
  }

  function resetMemoryForm(): void {
    editingMemoryId = '';
    memoryTitle = '';
    memoryContent = '';
    memoryTags = '';
    memoryScope = 'global';
    memoryImportance = 70;
    memoryPinned = true;
    memoryActive = true;
  }

  async function deleteMemory(memoryId: string): Promise<void> {
    if (!confirm('Delete this memory?')) {
      return;
    }
    await deleteJSON<{ ok: boolean }>(`/api/v1/memories/${memoryId}`);
    if (editingMemoryId === memoryId) {
      resetMemoryForm();
    }
    await refreshMemories();
  }

  async function rememberMessage(message: Message): Promise<void> {
    const response = await postJSON<MemoryResponse>('/api/v1/memories', {
      title: message.content.slice(0, 80) || 'Chat memory',
      content: message.content,
      tags: [],
      scope: 'conversation',
      conversation_id: selectedConversationId,
      importance: 60,
      pinned: false,
      active: true,
      source: 'message',
      source_message_id: message.id
    });
    memories = [response.memory, ...memories];
    notice = 'Memory created from message.';
  }

  async function refreshMCP(): Promise<void> {
    const [serversResponse, toolsResponse] = await Promise.all([
      getJSON<MCPServersResponse>('/api/v1/mcp-servers'),
      getJSON<MCPToolsResponse>('/api/v1/mcp-tools')
    ]);
    mcpServers = serversResponse.servers ?? [];
    mcpTools = toolsResponse.tools ?? [];
  }

  async function createMCPServer(): Promise<void> {
    const headers = parseHeaderText(mcpHttpHeaders);
    if (mcpAuthorization.trim()) {
      headers.Authorization = mcpAuthorization.trim();
    }
    const payload = {
      name: mcpName,
      description: mcpDescription,
      transport_type: mcpTransport,
      http_url: mcpHttpUrl,
      command: mcpCommand,
      arguments: mcpArguments.split(' ').filter(Boolean),
      working_directory: mcpWorkingDirectory,
      http_headers: headers,
      environment: parseHeaderText(mcpEnvironment),
      enabled: mcpEnabled,
      startup_timeout_ms: mcpStartupTimeoutMS,
      request_timeout_ms: mcpRequestTimeoutMS
    };
    const serverBeingEdited = editingMCPServerId;
    const response = serverBeingEdited
      ? await putJSON<MCPServerResponse>(`/api/v1/mcp-servers/${serverBeingEdited}`, payload)
      : await postJSON<MCPServerResponse>('/api/v1/mcp-servers', payload);
    mcpServers = serverBeingEdited
      ? mcpServers.map((server) => (server.id === response.server.id ? response.server : server))
      : [response.server, ...mcpServers];
    resetMCPServerForm();
    notice = serverBeingEdited ? 'MCP server updated.' : 'MCP server saved.';
  }

  function editMCPServer(server: MCPServer): void {
    editingMCPServerId = server.id;
    mcpName = server.name;
    mcpDescription = server.description;
    mcpTransport = server.transport_type;
    mcpHttpUrl = server.http_url ?? '';
    mcpHttpHeaders = '';
    mcpCommand = server.command ?? '';
    mcpArguments = server.arguments.join(' ');
    mcpWorkingDirectory = server.working_directory ?? '';
    mcpEnvironment = '';
    mcpAuthorization = '';
    mcpStartupTimeoutMS = server.startup_timeout_ms;
    mcpRequestTimeoutMS = server.request_timeout_ms;
    mcpEnabled = server.enabled;
  }

  function resetMCPServerForm(): void {
    editingMCPServerId = '';
    mcpName = '';
    mcpDescription = '';
    mcpTransport = 'http';
    mcpHttpUrl = '';
    mcpHttpHeaders = '';
    mcpCommand = '';
    mcpArguments = '';
    mcpWorkingDirectory = '';
    mcpEnvironment = '';
    mcpAuthorization = '';
    mcpStartupTimeoutMS = 10000;
    mcpRequestTimeoutMS = 30000;
    mcpEnabled = true;
  }

  async function deleteMCPServer(serverId: string): Promise<void> {
    if (!confirm('Delete this MCP server and its discovered tools?')) {
      return;
    }
    await deleteJSON<{ ok: boolean }>(`/api/v1/mcp-servers/${serverId}`);
    if (editingMCPServerId === serverId) {
      resetMCPServerForm();
    }
    await refreshMCP();
    notice = 'MCP server deleted.';
  }

  async function discoverMCPTools(serverId: string): Promise<void> {
    const response = await postJSON<MCPToolsResponse>(`/api/v1/mcp-servers/${serverId}/discover`);
    mcpTools = [...(response.tools ?? []), ...mcpTools.filter((tool) => tool.server_id !== serverId)];
    await refreshMCP();
    notice = 'MCP tools discovered.';
  }

  async function testMCPServer(serverId: string): Promise<void> {
    await postJSON<MCPToolsResponse>(`/api/v1/mcp-servers/${serverId}/test`);
    await refreshMCP();
    notice = 'MCP server connection tested.';
  }

  async function updateToolPermission(toolId: string, permissionMode: string): Promise<void> {
    await putJSON<{ ok: boolean }>(`/api/v1/mcp-tools/${toolId}/permission`, { permission_mode: permissionMode });
    await refreshMCP();
  }

  async function refreshTasks(): Promise<void> {
    const [tasksResponse, runsResponse] = await Promise.all([
      getJSON<TasksResponse>('/api/v1/tasks'),
      getJSON<TaskRunsResponse>('/api/v1/task-runs')
    ]);
    taskRecords = tasksResponse.tasks ?? [];
    taskRuns = runsResponse.runs ?? [];
  }

  async function createTask(): Promise<void> {
    submitting = true;
    errorMessage = '';
    notice = '';
    try {
      const payload = {
        name: taskName,
        description: taskDescription,
        task_type: taskType,
        state: taskState,
        agent_id: taskType === 'agent' ? taskAgentId : '',
        provider_id: taskType === 'agent' ? taskProviderId : '',
        model: taskType === 'agent' ? taskModel : '',
        prompt: taskPrompt,
        tool_policy: taskToolPolicy,
        max_retries: taskMaxRetries,
        timeout_ms: taskTimeoutMS,
        concurrency_policy: taskConcurrencyPolicy,
        schedule_mode: taskScheduleMode,
        cron_expression: taskScheduleMode === 'cron' ? taskCronExpression : '',
        interval_seconds: taskScheduleMode === 'interval' ? taskIntervalSeconds : 0,
        run_at: taskScheduleMode === 'one_time' ? taskRunAt : '',
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
      };
      const taskBeingEdited = editingTaskId;
      const response = taskBeingEdited
        ? await putJSON<TaskResponse>(`/api/v1/tasks/${taskBeingEdited}`, payload)
        : await postJSON<TaskResponse>('/api/v1/tasks', payload);
      const record = { task: response.task, schedule: response.schedule };
      taskRecords = taskBeingEdited
        ? taskRecords.map((item) => (item.task.id === response.task.id ? record : item))
        : [record, ...taskRecords];
      resetTaskForm();
      notice = taskBeingEdited ? 'Task updated.' : 'Task saved.';
    } catch (error) {
      errorMessage = messageFromError(error);
    } finally {
      submitting = false;
    }
  }

  function editTask(record: TaskRecord): void {
    editingTaskId = record.task.id;
    taskName = record.task.name;
    taskDescription = record.task.description;
    taskPrompt = record.task.prompt;
    taskType = record.task.task_type;
    taskState = record.task.state;
    taskAgentId = record.task.agent_id ?? '';
    taskProviderId = record.task.provider_id ?? '';
    taskModel = record.task.model ?? '';
    taskScheduleMode = record.schedule.mode;
    taskCronExpression = record.schedule.cron_expression ?? '';
    taskIntervalSeconds = record.schedule.interval_seconds ?? 3600;
    taskRunAt = record.schedule.run_at ?? '';
    taskToolPolicy = record.task.tool_policy;
    taskMaxRetries = record.task.max_retries;
    taskTimeoutMS = record.task.timeout_ms;
    taskConcurrencyPolicy = record.task.concurrency_policy;
  }

  function resetTaskForm(): void {
    editingTaskId = '';
    taskName = '';
    taskDescription = '';
    taskPrompt = '';
    taskType = 'agent';
    taskState = 'enabled';
    taskAgentId = '';
    taskProviderId = '';
    taskModel = '';
    taskScheduleMode = 'manual';
    taskCronExpression = '';
    taskIntervalSeconds = 3600;
    taskRunAt = '';
    taskToolPolicy = 'use_preapproved_tools_only';
    taskMaxRetries = 3;
    taskTimeoutMS = 600000;
    taskConcurrencyPolicy = 'skip';
  }

  async function deleteTask(taskId: string): Promise<void> {
    if (!confirm('Delete this task and its schedules? Existing run history will be removed.')) {
      return;
    }
    await deleteJSON<{ ok: boolean }>(`/api/v1/tasks/${taskId}`);
    if (editingTaskId === taskId) {
      resetTaskForm();
    }
    await refreshTasks();
    notice = 'Task deleted.';
  }

  async function runTask(taskId: string): Promise<void> {
    const response = await postJSON<TaskRunResponse>(`/api/v1/tasks/${taskId}/run`);
    taskRuns = [response.run, ...taskRuns];
    notice = 'Task queued.';
  }

  async function cancelTaskRun(runId: string): Promise<void> {
    await postJSON<{ ok: boolean }>(`/api/v1/task-runs/${runId}/cancel`);
    await refreshTasks();
    notice = 'Task run cancelled.';
  }

  async function retryTaskRun(runId: string): Promise<void> {
    const response = await postJSON<TaskRunResponse>(`/api/v1/task-runs/${runId}/retry`);
    taskRuns = [response.run, ...taskRuns];
    notice = 'Task retry queued.';
  }

  async function showTaskRunEvents(runId: string): Promise<void> {
    const response = await getJSON<TaskRunRecordResponse>(`/api/v1/task-runs/${runId}`);
    taskRunEvents = response.events ?? [];
  }

  function taskNameForRun(run: TaskRun): string {
    return taskRecords.find((record) => record.task.id === run.task_id)?.task.name ?? run.task_id;
  }

  async function refreshFeedback(conversationId = selectedConversationId): Promise<void> {
    if (!conversationId) {
      feedbackByMessage = {};
      return;
    }
    const response = await getJSON<FeedbackListResponse>(`/api/v1/feedback?conversation_id=${conversationId}`);
    feedbackByMessage = Object.fromEntries((response.feedback ?? []).map((item) => [item.message_id, item]));
  }

  async function refreshFeedbackStats(): Promise<void> {
    const response = await getJSON<FeedbackStatsResponse>('/api/v1/feedback/stats');
    feedbackStats = response.stats;
  }

  async function submitFeedback(message: Message, rating: 'positive' | 'negative'): Promise<void> {
    const response = await putJSON<FeedbackResponse>(`/api/v1/messages/${message.id}/feedback`, {
      rating,
      reason: rating === 'negative' ? negativeFeedbackReason : '',
      comment: ''
    });
    feedbackByMessage = { ...feedbackByMessage, [message.id]: response.feedback };
    await refreshFeedbackStats();
    notice = rating === 'positive' ? 'Positive feedback saved.' : 'Negative feedback saved.';
  }

  async function clearFeedback(messageId: string): Promise<void> {
    await deleteJSON<{ ok: boolean }>(`/api/v1/messages/${messageId}/feedback`);
    const next = { ...feedbackByMessage };
    delete next[messageId];
    feedbackByMessage = next;
    await refreshFeedbackStats();
    notice = 'Feedback removed.';
  }

  async function regenerateWithFeedback(message: Message): Promise<void> {
    const feedback = feedbackByMessage[message.id];
    const instruction =
      feedback?.rating === 'negative'
        ? `Address this feedback reason: ${feedback.reason || negativeFeedbackReason}. Preserve the original user intent and produce a better answer.`
        : 'Regenerate the response with a clearer and more useful answer.';
    await postStream(
      `/api/v1/messages/${message.id}/regenerate`,
      { provider_id: selectedProviderId, model: selectedModel, regeneration_instruction: instruction },
      handleChatEvent
    );
    await Promise.all([selectConversation(selectedConversationId), refreshConversations()]);
  }

  async function refreshReplyPresets(): Promise<void> {
    const response = await getJSON<ReplyPresetsResponse>('/api/v1/reply-presets');
    replyPresets = response.presets ?? [];
    if (!selectedReplyPresetId) {
      selectedReplyPresetId = replyPresets.find((preset) => preset.active)?.id ?? '';
    }
  }

  async function createReplyPreset(): Promise<void> {
    const response = await postJSON<ReplyPresetResponse>('/api/v1/reply-presets', {
      name: replyPresetName,
      description: replyPresetDescription,
      prompt_instruction: replyPresetInstruction,
      icon: 'message-circle',
      sort_order: replyPresets.length + 1,
      active: true
    });
    replyPresets = [...replyPresets, response.preset];
    replyPresetName = '';
    replyPresetDescription = '';
    replyPresetInstruction = '';
    notice = 'Reply preset saved.';
  }

  async function toggleReplyPreset(preset: ReplyPreset): Promise<void> {
    const response = await putJSON<ReplyPresetResponse>(`/api/v1/reply-presets/${preset.id}`, {
      name: preset.name,
      description: preset.description,
      prompt_instruction: preset.prompt_instruction,
      icon: preset.icon,
      sort_order: preset.sort_order,
      active: !preset.active
    });
    replyPresets = replyPresets.map((item) => (item.id === preset.id ? response.preset : item));
  }

  async function resetReplyPresets(): Promise<void> {
    await postJSON<{ ok: boolean }>('/api/v1/reply-presets/reset');
    await refreshReplyPresets();
    notice = 'Default reply presets are available.';
  }

  function selectReplySource(message: Message): void {
    selectedReplySourceId = message.id;
    replyDraft = '';
  }

  async function generateReplyDraft(): Promise<void> {
    if (!selectedReplySourceId || !selectedReplyPresetId) {
      errorMessage = 'Select a source message and reply preset.';
      return;
    }
    const response = await postJSON<ReplyDraftResponse>('/api/v1/reply-drafts', {
      source_message_id: selectedReplySourceId,
      preset_id: selectedReplyPresetId,
      custom_instruction: replyCustomInstruction,
      provider_id: selectedProviderId,
      model: selectedModel
    });
    replyDraft = response.draft.generated_draft;
    notice = 'Reply draft generated.';
  }

  function insertReplyDraft(): void {
    composer = replyDraft;
    replyDraft = '';
    selectedReplySourceId = '';
  }

  function renderMarkdown(content: string): string {
    const rendered = marked.parse(content, {
      async: false,
      breaks: true,
      gfm: true
    });
    return DOMPurify.sanitize(rendered);
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

        <section class="panel" aria-labelledby="feedback-stats-title">
          <div class="panel-heading">
            <h2 id="feedback-stats-title">Feedback statistics</h2>
            <button on:click={refreshFeedbackStats} type="button">Refresh</button>
          </div>
          <dl class="status-grid">
            <div>
              <dt>Positive</dt>
              <dd>{feedbackStats.positive}</dd>
            </div>
            <div>
              <dt>Negative</dt>
              <dd>{feedbackStats.negative}</dd>
            </div>
          </dl>
        </section>

        <section class="providers-layout">
          <form class="panel" on:submit|preventDefault={createReplyPreset}>
            <h2>{strings.replies.addPreset}</h2>
            <label>
              Name
              <input bind:value={replyPresetName} required />
            </label>
            <label>
              Description
              <input bind:value={replyPresetDescription} />
            </label>
            <label>
              Prompt instruction
              <textarea bind:value={replyPresetInstruction} required></textarea>
            </label>
            <button type="submit">{strings.replies.addPreset}</button>
          </form>
          <section class="panel" aria-labelledby="reply-presets-title">
            <div class="panel-heading">
              <h2 id="reply-presets-title">{strings.replies.presets}</h2>
              <button on:click={resetReplyPresets} type="button">{strings.replies.resetDefaults}</button>
            </div>
            <div class="table-list">
              {#each replyPresets as preset (preset.id)}
                <article>
                  <div>
                    <strong>{preset.name}</strong>
                    <span>{preset.description}</span>
                    <span>{preset.active ? 'active' : 'disabled'}{preset.system_default ? ' / default' : ''}</span>
                  </div>
                  <div>
                    <button on:click={() => toggleReplyPreset(preset)} type="button">
                      {preset.active ? 'Disable' : 'Enable'}
                    </button>
                  </div>
                </article>
              {/each}
            </div>
          </section>
        </section>
      {:else}
        {#if activeView === strings.nav.chat}
          <section class="workbench" aria-label="Chat workspace">
            <aside class="list-panel">
              <div class="panel-heading">
                <h2>Conversations</h2>
                <button on:click={createConversation} type="button">{strings.chat.newConversation}</button>
              </div>
              {#if conversations.length === 0}
                <p>{strings.chat.noConversations}</p>
              {:else}
                {#each conversations as conversation (conversation.id)}
                  <button
                    class:active={selectedConversationId === conversation.id}
                    class="list-item"
                    on:click={() => selectConversation(conversation.id)}
                    type="button"
                  >
                    <strong>{conversation.title}</strong>
                    <span>{new Date(conversation.updated_at).toLocaleString()}</span>
                  </button>
                {/each}
              {/if}
            </aside>
            <section class="chat-panel">
              <div class="chat-toolbar">
                <select bind:value={selectedProviderId} on:change={() => refreshModels(selectedProviderId)}>
                  <option value="">Select provider</option>
                  {#each providers as provider (provider.id)}
                    <option value={provider.id}>{provider.name}</option>
                  {/each}
                </select>
                <select bind:value={selectedAgentId}>
                  <option value="">No agent</option>
                  {#each agents as agent (agent.id)}
                    <option value={agent.id}>{agent.name}</option>
                  {/each}
                </select>
                <select bind:value={selectedModel}>
                  <option value="">Manual model</option>
                  {#each providerModels as model (model.id)}
                    <option value={model.model_id}>{model.model_id}</option>
                  {/each}
                </select>
                <input bind:value={selectedModel} aria-label="Model" placeholder="Model ID" />
              </div>
              <ConversationSummaryPanel
                conversation={selectedConversation}
                onClear={clearSummary}
                onRegenerate={regenerateSummary}
              />
              <div class="message-list" aria-live="polite">
                {#if messages.length === 0}
                  <p>{strings.chat.noMessages}</p>
                {:else}
                  {#each messages as message (message.id)}
                    <article class={`message ${message.role}`}>
                      <header>
                        <strong>{message.role}</strong>
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
                          <button on:click={() => rememberMessage(message)} type="button">{strings.chat.remember}</button>
                          <button on:click={() => selectReplySource(message)} type="button">{strings.chat.draftReply}</button>
                          {#if message.role === 'assistant'}
                            <button
                              class:active={feedbackByMessage[message.id]?.rating === 'positive'}
                              on:click={() => submitFeedback(message, 'positive')}
                              type="button"
                            >
                              {strings.chat.feedbackUp}
                            </button>
                            <select
                              aria-label="Negative feedback reason"
                              bind:value={negativeFeedbackReason}
                            >
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
                              on:click={() => submitFeedback(message, 'negative')}
                              type="button"
                            >
                              {strings.chat.feedbackDown}
                            </button>
                            {#if feedbackByMessage[message.id]}
                              <button on:click={() => clearFeedback(message.id)} type="button">{strings.chat.clearFeedback}</button>
                              <button on:click={() => regenerateWithFeedback(message)} type="button">{strings.chat.regenerate}</button>
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
                    <h2 id="reply-draft-title">{strings.replies.title}</h2>
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
                    <button on:click={generateReplyDraft} type="button">{strings.replies.generate}</button>
                    {#if replyDraft}
                      <button on:click={insertReplyDraft} type="button">{strings.replies.insert}</button>
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
                onApprove={approveToolCall}
                onDeny={denyToolCall}
                onRefresh={refreshPendingToolApprovals}
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
              <form class="composer" on:submit|preventDefault={sendMessage}>
                <textarea bind:value={composer} placeholder={strings.chat.composerPlaceholder}></textarea>
                <div>
                  {#if activeRunId}
                    <button on:click={stopGeneration} type="button">{strings.chat.stop}</button>
                  {/if}
                  <button disabled={submitting || !composer.trim()} type="submit">{strings.chat.send}</button>
                </div>
              </form>
            </section>
          </section>
        {:else if activeView === strings.nav.agents}
          <section class="providers-layout">
            <form class="panel" on:submit|preventDefault={createAgent}>
              <div class="panel-heading">
                <h2>{editingAgentId ? 'Edit agent' : strings.agents.add}</h2>
                {#if editingAgentId}
                  <button on:click={resetAgentForm} type="button">Cancel edit</button>
                {/if}
              </div>
              <label>
                Name
                <input bind:value={agentName} required />
              </label>
              <label>
                Description
                <input bind:value={agentDescription} />
              </label>
              <label>
                Avatar or icon
                <input bind:value={agentAvatar} />
              </label>
              <label>
                System prompt
                <textarea bind:value={agentPrompt} required></textarea>
              </label>
              <label>
                Default provider
                <select bind:value={agentDefaultProviderId}>
                  <option value="">No default provider</option>
                  {#each providers as provider (provider.id)}
                    <option value={provider.id}>{provider.name}</option>
                  {/each}
                </select>
              </label>
              <label>
                Default model
                <input bind:value={agentDefaultModel} />
              </label>
              <label>
                Fallback model
                <input bind:value={agentFallbackModel} />
              </label>
              <label>
                Temperature
                <input bind:value={agentTemperature} min="0" max="2" step="0.1" type="number" />
              </label>
              <label>
                Maximum tool iterations
                <input bind:value={agentMaxToolIterations} min="1" max="32" type="number" />
              </label>
              <label>
                Memory mode
                <select bind:value={agentMemoryMode}>
                  <option value="none">none</option>
                  <option value="pinned_only">pinned_only</option>
                  <option value="relevant">relevant</option>
                  <option value="all">all</option>
                </select>
              </label>
              <label>
                Default tool permission
                <select bind:value={agentToolPermissionDefault}>
                  <option value="deny">deny</option>
                  <option value="ask">ask</option>
                  <option value="allow">allow</option>
                </select>
              </label>
              <label class="inline-check">
                <input bind:checked={agentActive} type="checkbox" />
                Active
              </label>
              <button type="submit">{editingAgentId ? 'Save agent' : strings.agents.add}</button>
            </form>
            <section class="panel">
              <div class="panel-heading">
                <h2>Agents</h2>
                <button on:click={refreshAgents} type="button">Refresh</button>
              </div>
              {#if agents.length === 0}
                <p>{strings.agents.noAgents}</p>
              {:else}
                <div class="table-list">
                  {#each agents as agent (agent.id)}
                    <article>
                      <div>
                        <strong>{agent.name}</strong>
                        <span>{agent.memory_access_mode} / {agent.active ? 'active' : 'disabled'}</span>
                        <span>
                          tools {agent.tool_permission_default} / max iterations {agent.max_tool_iterations} / temp
                          {agent.temperature}
                        </span>
                        {#if agent.default_provider_id || agent.default_model || agent.fallback_model}
                          <span>
                            {agent.default_provider_id ? 'provider configured' : ''}
                            {agent.default_model ? ` / default ${agent.default_model}` : ''}
                            {agent.fallback_model ? ` / fallback ${agent.fallback_model}` : ''}
                          </span>
                        {/if}
                      </div>
                      <div>
                        <button on:click={() => editAgent(agent)} type="button">Edit</button>
                        <button on:click={() => duplicateAgent(agent.id)} type="button">{strings.agents.duplicate}</button>
                        <button on:click={() => deleteAgent(agent.id)} type="button">Delete</button>
                      </div>
                    </article>
                  {/each}
                </div>
              {/if}
            </section>
          </section>
        {:else if activeView === strings.nav.memories}
          <section class="providers-layout">
            <form class="panel" on:submit|preventDefault={createMemory}>
              <div class="panel-heading">
                <h2>{editingMemoryId ? 'Edit memory' : strings.memories.add}</h2>
                {#if editingMemoryId}
                  <button on:click={resetMemoryForm} type="button">Cancel edit</button>
                {/if}
              </div>
              <label>
                Title
                <input bind:value={memoryTitle} required />
              </label>
              <label>
                Content
                <textarea bind:value={memoryContent} required></textarea>
              </label>
              <label>
                Tags
                <input bind:value={memoryTags} placeholder="project, preference" />
              </label>
              <label>
                Scope
                <select bind:value={memoryScope}>
                  <option value="global">global</option>
                  <option value="workspace">workspace</option>
                  <option value="agent">agent</option>
                  <option value="conversation">conversation</option>
                </select>
              </label>
              <label>
                Importance
                <input bind:value={memoryImportance} min="1" max="100" type="number" />
              </label>
              <label class="inline-check">
                <input bind:checked={memoryPinned} type="checkbox" />
                {strings.memories.pin}
              </label>
              <label class="inline-check">
                <input bind:checked={memoryActive} type="checkbox" />
                Active
              </label>
              <button type="submit">{editingMemoryId ? 'Save memory' : strings.memories.add}</button>
            </form>
            <section class="panel">
              <div class="panel-heading">
                <h2>Memories</h2>
                <button on:click={refreshMemories} type="button">Refresh</button>
              </div>
              {#if memories.length === 0}
                <p>{strings.memories.noMemories}</p>
              {:else}
                <div class="table-list">
                  {#each memories as memory (memory.id)}
                    <article>
                      <div>
                        <strong>{memory.title}</strong>
                        <span>
                          {memory.scope} / importance {memory.importance} / {memory.pinned ? 'pinned' : 'unpinned'} /
                          {memory.active ? 'active' : 'disabled'} / used {memory.use_count}
                        </span>
                        <span>source {memory.source}</span>
                        <span>{memory.tags.join(', ')}</span>
                      </div>
                      <div>
                        <button on:click={() => editMemory(memory)} type="button">Edit</button>
                        <button on:click={() => deleteMemory(memory.id)} type="button">{strings.memories.delete}</button>
                      </div>
                    </article>
                  {/each}
                </div>
              {/if}
            </section>
          </section>
        {:else if activeView === strings.nav.tasks}
          <section class="providers-layout">
            <form class="panel" on:submit|preventDefault={createTask}>
              <div class="panel-heading">
                <h2>{editingTaskId ? 'Edit task' : strings.tasks.add}</h2>
                {#if editingTaskId}
                  <button on:click={resetTaskForm} type="button">Cancel edit</button>
                {/if}
              </div>
              <label>
                Name
                <input bind:value={taskName} required />
              </label>
              <label>
                Description
                <input bind:value={taskDescription} />
              </label>
              <label>
                Type
                <select bind:value={taskType}>
                  <option value="agent">agent</option>
                  <option value="system">system</option>
                </select>
              </label>
              <label>
                State
                <select bind:value={taskState}>
                  <option value="draft">draft</option>
                  <option value="enabled">enabled</option>
                  <option value="disabled">disabled</option>
                </select>
              </label>
              {#if taskType === 'agent'}
                <label>
                  Agent
                  <select bind:value={taskAgentId}>
                    <option value="">Task default agent behavior</option>
                    {#each agents as agent (agent.id)}
                      <option value={agent.id}>{agent.name}</option>
                    {/each}
                  </select>
                </label>
                <label>
                  Provider override
                  <select bind:value={taskProviderId}>
                    <option value="">Agent/default provider</option>
                    {#each providers as provider (provider.id)}
                      <option value={provider.id}>{provider.name}</option>
                    {/each}
                  </select>
                </label>
                <label>
                  Model override
                  <input bind:value={taskModel} />
                </label>
              {/if}
              <label>
                Prompt
                <textarea bind:value={taskPrompt} required></textarea>
              </label>
              <label>
                Schedule
                <select bind:value={taskScheduleMode}>
                  <option value="manual">manual</option>
                  <option value="one_time">one_time</option>
                  <option value="cron">cron</option>
                  <option value="interval">interval</option>
                </select>
              </label>
              {#if taskScheduleMode === 'cron'}
                <label>
                  Cron expression
                  <input bind:value={taskCronExpression} placeholder="0 * * * *" required />
                </label>
              {:else if taskScheduleMode === 'interval'}
                <label>
                  Interval seconds
                  <input bind:value={taskIntervalSeconds} min="1" type="number" />
                </label>
              {:else if taskScheduleMode === 'one_time'}
                <label>
                  Run at
                  <input bind:value={taskRunAt} placeholder="2026-06-22T12:00:00Z" required />
                </label>
              {/if}
              <label>
                Tool policy
                <select bind:value={taskToolPolicy}>
                  <option value="use_preapproved_tools_only">use_preapproved_tools_only</option>
                  <option value="fail_if_approval_required">fail_if_approval_required</option>
                </select>
              </label>
              <label>
                Maximum retries
                <input bind:value={taskMaxRetries} min="0" max="20" type="number" />
              </label>
              <label>
                Timeout, milliseconds
                <input bind:value={taskTimeoutMS} min="1000" type="number" />
              </label>
              <label>
                Concurrency policy
                <select bind:value={taskConcurrencyPolicy}>
                  <option value="allow">allow</option>
                  <option value="skip">skip</option>
                  <option value="replace">replace</option>
                </select>
              </label>
              <button disabled={submitting} type="submit">{editingTaskId ? 'Save task' : strings.tasks.add}</button>
            </form>
            <section class="panel">
              <div class="panel-heading">
                <h2>Tasks</h2>
                <button on:click={refreshTasks} type="button">Refresh</button>
              </div>
              {#if taskRecords.length === 0}
                <p>{strings.tasks.noTasks}</p>
              {:else}
                <div class="table-list">
                  {#each taskRecords as record (record.task.id)}
                    <article>
                      <div>
                        <strong>{record.task.name}</strong>
                        <span>
                          {record.task.task_type} / {record.task.state}
                          {record.task.system_managed ? ' / system-managed' : ''}
                        </span>
                        <span>
                          {record.schedule.mode}
                          {record.schedule.next_run_at
                            ? ` / next ${new Date(record.schedule.next_run_at).toLocaleString()}`
                            : ''}
                        </span>
                        <span>
                          retries {record.task.max_retries} / timeout {Math.round(record.task.timeout_ms / 1000)}s /
                          concurrency {record.task.concurrency_policy}
                        </span>
                        {#if record.task.agent_id || record.task.provider_id || record.task.model}
                          <span>
                            {record.task.agent_id ? 'agent configured' : ''}
                            {record.task.provider_id ? ' / provider override' : ''}
                            {record.task.model ? ` / ${record.task.model}` : ''}
                          </span>
                        {/if}
                      </div>
                      <div>
                        <button on:click={() => editTask(record)} type="button">Edit</button>
                        <button on:click={() => runTask(record.task.id)} type="button">{strings.tasks.runNow}</button>
                        {#if !record.task.system_managed}
                          <button on:click={() => deleteTask(record.task.id)} type="button">Delete</button>
                        {/if}
                      </div>
                    </article>
                  {/each}
                </div>
              {/if}
              <div class="panel-heading nested-heading">
                <h2>Runs</h2>
              </div>
              {#if taskRuns.length === 0}
                <p>{strings.tasks.noRuns}</p>
              {:else}
                <div class="table-list">
                  {#each taskRuns as run (run.id)}
                    <article>
                      <div>
                        <strong>{taskNameForRun(run)}</strong>
                        <span>{run.state} / attempt {run.attempt + 1} of {run.max_retries + 1}</span>
                        <span>Queued {new Date(run.queued_at).toLocaleString()}</span>
                        {#if run.result}
                          <span>{run.result}</span>
                        {/if}
                        {#if run.error_message}
                          <span>{run.error_message}</span>
                        {/if}
                      </div>
                      <div>
                        <button on:click={() => showTaskRunEvents(run.id)} type="button">{strings.tasks.events}</button>
                        {#if run.state === 'queued' || run.state === 'running' || run.state === 'waiting'}
                          <button on:click={() => cancelTaskRun(run.id)} type="button">{strings.tasks.cancel}</button>
                        {/if}
                        {#if run.state === 'failed' || run.state === 'timed_out' || run.state === 'cancelled'}
                          <button on:click={() => retryTaskRun(run.id)} type="button">{strings.tasks.retry}</button>
                        {/if}
                      </div>
                    </article>
                  {/each}
                </div>
              {/if}
              {#if taskRunEvents.length > 0}
                <div class="event-log">
                  {#each taskRunEvents as event (event.id)}
                    <p><strong>{event.level}</strong> {new Date(event.created_at).toLocaleString()} - {event.message}</p>
                  {/each}
                </div>
              {/if}
            </section>
          </section>
        {:else if activeView === strings.nav.mcp}
          <section class="providers-layout">
            <form class="panel" on:submit|preventDefault={createMCPServer}>
              <div class="panel-heading">
                <h2>{editingMCPServerId ? 'Edit MCP server' : strings.mcp.add}</h2>
                {#if editingMCPServerId}
                  <button on:click={resetMCPServerForm} type="button">Cancel edit</button>
                {/if}
              </div>
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
              <label>
                Startup timeout, milliseconds
                <input bind:value={mcpStartupTimeoutMS} min="1000" type="number" />
              </label>
              <label>
                Request timeout, milliseconds
                <input bind:value={mcpRequestTimeoutMS} min="1000" type="number" />
              </label>
              <label class="inline-check">
                <input bind:checked={mcpEnabled} type="checkbox" />
                Enabled
              </label>
              <button type="submit">{editingMCPServerId ? 'Save MCP server' : strings.mcp.add}</button>
            </form>
            <section class="panel">
              <div class="panel-heading">
                <h2>MCP servers</h2>
                <button on:click={refreshMCP} type="button">Refresh</button>
              </div>
              {#if mcpServers.length === 0}
                <p>{strings.mcp.noServers}</p>
              {:else}
                <div class="table-list">
                  {#each mcpServers as server (server.id)}
                    <article>
                      <div>
                        <strong>{server.name}</strong>
                        <span>{server.transport_type} / {server.health_status}</span>
                        <span>{server.enabled ? 'enabled' : 'disabled'} / timeout {server.request_timeout_ms} ms</span>
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
                          <span>{server.last_error}</span>
                        {/if}
                      </div>
                      <div>
                        <button on:click={() => editMCPServer(server)} type="button">Edit</button>
                        <button on:click={() => testMCPServer(server.id)} type="button">Test</button>
                        <button on:click={() => discoverMCPTools(server.id)} type="button">{strings.mcp.discover}</button>
                        <button on:click={() => deleteMCPServer(server.id)} type="button">Delete</button>
                      </div>
                    </article>
                  {/each}
                </div>
              {/if}
              <div class="panel-heading nested-heading">
                <h2>Tools</h2>
              </div>
              {#if mcpTools.length === 0}
                <p>{strings.mcp.noTools}</p>
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
                          on:change={(event) => updateToolPermission(tool.id, event.currentTarget.value)}
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
          </section>
        {:else if activeView === strings.nav.providers}
          <section class="providers-layout">
            <form class="panel" on:submit|preventDefault={createProvider}>
              <div class="panel-heading">
                <h2>{editingProviderId ? 'Edit provider' : strings.providers.add}</h2>
                {#if editingProviderId}
                  <button on:click={resetProviderForm} type="button">Cancel edit</button>
                {/if}
              </div>
              <p>{strings.providers.apiKeyHelp}</p>
              <label>
                Name
                <input bind:value={providerName} required />
              </label>
              <label>
                Base URL
                <input bind:value={providerBaseUrl} placeholder="https://bifrost.example.com" required />
              </label>
              <label>
                API key
                <input bind:value={providerApiKey} autocomplete="off" type="password" />
              </label>
              <label>
                Environment reference
                <input bind:value={providerApiKeyEnvRef} placeholder="env:BIFROST_API_KEY" />
              </label>
              <label>
                Organization header
                <input bind:value={providerOrganization} />
              </label>
              <label>
                Project header
                <input bind:value={providerProject} />
              </label>
              <label>
                Custom headers JSON
                <textarea bind:value={providerCustomHeaders} placeholder="JSON object with string header values"></textarea>
              </label>
              <label>
                Default model
                <input bind:value={providerDefaultModel} placeholder="gpt-4.1-mini" />
              </label>
              <label>
                Fallback model
                <input bind:value={providerFallbackModel} />
              </label>
              <label>
                Request timeout, milliseconds
                <input bind:value={providerTimeoutMS} min="1000" max="600000" type="number" />
              </label>
              <label class="inline-check">
                <input bind:checked={providerEnabled} type="checkbox" />
                Enabled
              </label>
              <button disabled={submitting} type="submit">
                {editingProviderId ? 'Save provider' : strings.providers.add}
              </button>
            </form>
            <section class="panel">
              <div class="panel-heading">
                <h2>{strings.providers.title}</h2>
                <button on:click={refreshProviders} type="button">Refresh</button>
              </div>
              {#if providers.length === 0}
                <p>{strings.providers.noProviders}</p>
              {:else}
                <div class="table-list">
                  {#each providers as provider (provider.id)}
                    <article>
                      <div>
                        <strong>{provider.name}</strong>
                        <span>{provider.base_url}</span>
                        <span>{provider.health_status}{provider.last_error ? `: ${provider.last_error}` : ''}</span>
                        {#if provider.default_model || provider.fallback_model}
                          <span>
                            {provider.default_model ? `default ${provider.default_model}` : ''}
                            {provider.fallback_model ? ` / fallback ${provider.fallback_model}` : ''}
                          </span>
                        {/if}
                        {#if provider.last_health_check_at}
                          <span>
                            Checked {new Date(provider.last_health_check_at).toLocaleString()}
                            {provider.health_latency_ms ? ` / ${provider.health_latency_ms} ms` : ''}
                          </span>
                        {/if}
                      </div>
                      <div>
                        <button on:click={() => editProvider(provider)} type="button">Edit</button>
                        <button on:click={() => testProvider(provider.id)} type="button">{strings.providers.test}</button>
                        <button on:click={() => refreshProviderModels(provider.id)} type="button">
                          {strings.providers.refreshModels}
                        </button>
                        <button on:click={() => deleteProvider(provider.id)} type="button">Delete</button>
                      </div>
                    </article>
                  {/each}
                </div>
              {/if}
            </section>
          </section>
        {:else}
          <section class="panel" aria-labelledby="screen-title">
            <p class="eyebrow">{strings.workspace.title}</p>
            <h2 id="screen-title">{activeView}</h2>
            <p>{strings.workspace.emptyScreen}</p>
          </section>
        {/if}
      {/if}
    </section>
  </main>
{/if}
