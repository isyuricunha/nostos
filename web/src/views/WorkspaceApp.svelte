<script lang="ts">
  import { onMount } from 'svelte';
  import DOMPurify from 'dompurify';
  import { marked } from 'marked';
  import AppShell from '../app/shell/AppShell.svelte';
  import Notice from '../components/common/Notice.svelte';
  import AgentsView from './AgentsView.svelte';
  import AuthView from './AuthView.svelte';
  import ChatView from './ChatView.svelte';
  import MCPView from './MCPView.svelte';
  import MemoriesView from './MemoriesView.svelte';
  import ProvidersView from './ProvidersView.svelte';
  import SettingsView from './SettingsView.svelte';
  import TasksView from './TasksView.svelte';
  import { deleteJSON, getJSON, postJSON, postStream, putJSON } from '../lib/api';
  import { formatHeaderText, parseHeaderText } from '../lib/provider-form';
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
    ModelRefreshResponse,
    ModelRolesResponse,
    ModelRoleBinding,
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
    TaskToolCall,
    TasksResponse,
    ToolApprovalsResponse,
    ToolCall,
    ToolCallResponse,
    ToolCard,
    User,
    UserResponse
  } from '../lib/types';
  import { strings } from '../strings';

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
  let modelRoles: ModelRoleBinding[] = [];
  let conversations: Conversation[] = [];
  let messages: Message[] = [];
  let agents: Agent[] = [];
  let memories: Memory[] = [];
  let mcpServers: MCPServer[] = [];
  let mcpTools: MCPTool[] = [];
  let taskRecords: TaskRecord[] = [];
  let taskRuns: TaskRun[] = [];
  let taskRunEvents: TaskRunEvent[] = [];
  let taskRunToolCalls: TaskToolCall[] = [];
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
  let chatRoleProviderId = '';
  let chatRoleModel = '';
  let utilityRoleProviderId = '';
  let utilityRoleModel = '';
  let visionRoleProviderId = '';
  let visionRoleModel = '';
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
      refreshModels(),
      refreshModelRoles(),
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
    }
  }

  async function refreshModels(providerId = ''): Promise<void> {
    const providerQuery = providerId ? `&provider_id=${encodeURIComponent(providerId)}` : '';
    const response = await getJSON<ModelsResponse>(`/api/v1/models?limit=500&include_unavailable=true${providerQuery}`);
    providerModels = response.models ?? [];
    if ((!selectedProviderId || !selectedModel) && providerModels.length > 0) {
      selectedProviderId = selectedProviderId || providerModels[0].provider_id;
      selectedModel = selectedModel || providerModels[0].model_id;
    }
  }

  async function refreshModelRoles(): Promise<void> {
    const response = await getJSON<ModelRolesResponse>('/api/v1/model-roles');
    modelRoles = response.roles ?? [];
    const chatRole = modelRoles.find((role) => role.role === 'chat' && role.position === 0);
    const utilityRole = modelRoles.find((role) => role.role === 'utility' && role.position === 0);
    const visionRole = modelRoles.find((role) => role.role === 'vision' && role.position === 0);
    chatRoleProviderId = chatRole?.provider_id ?? chatRoleProviderId;
    chatRoleModel = chatRole?.model_id ?? chatRoleModel;
    utilityRoleProviderId = utilityRole?.provider_id ?? utilityRoleProviderId;
    utilityRoleModel = utilityRole?.model_id ?? utilityRoleModel;
    visionRoleProviderId = visionRole?.provider_id ?? visionRoleProviderId;
    visionRoleModel = visionRole?.model_id ?? visionRoleModel;
  }

  async function saveModelRole(role: 'chat' | 'utility' | 'vision', providerId: string, modelId: string): Promise<void> {
    if (!providerId || !modelId) {
      errorMessage = 'Select both a provider and model before saving this role.';
      return;
    }
    const response = await putJSON<ModelRolesResponse>(`/api/v1/model-roles/${role}`, {
      models: [{ provider_id: providerId, model_id: modelId }]
    });
    modelRoles = response.roles ?? [];
    notice = `${role} model saved.`;
    await refreshModelRoles();
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
      selectedProviderId = providerId;
      await postJSON<ModelRefreshResponse>(`/api/v1/providers/${providerId}/models/refresh`);
      notice = 'Model refresh started.';
      for (let attempt = 0; attempt < 60; attempt += 1) {
        await new Promise((resolve) => window.setTimeout(resolve, 1000));
        const status = await getJSON<ModelRefreshResponse>(`/api/v1/providers/${providerId}/models/refresh-status`);
        if (status.refresh.state === 'succeeded') {
          await refreshModels();
          const firstModel = providerModels.find((model) => model.provider_id === providerId);
          if (firstModel) {
            selectedModel = firstModel.model_id;
          }
          await refreshProviders();
          notice = 'Models refreshed.';
          return;
        }
        if (status.refresh.state === 'failed') {
          throw new Error(status.refresh.error_message || 'Model refresh failed.');
        }
      }
      await refreshModels();
      await refreshProviders();
      notice = 'Model refresh is still running.';
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
      await refreshModels();
    }
  }

  async function renameConversation(conversation: Conversation): Promise<void> {
    const title = window.prompt('Rename conversation', conversation.title);
    if (!title || title.trim() === conversation.title) {
      return;
    }
    const response = await putJSON<ConversationResponse>(`/api/v1/conversations/${conversation.id}`, { title: title.trim() });
    conversations = conversations.map((item) => (item.id === conversation.id ? response.conversation : item));
  }

  async function archiveConversation(conversation: Conversation): Promise<void> {
    const response = await putJSON<ConversationResponse>(`/api/v1/conversations/${conversation.id}`, { archive: true });
    conversations = conversations.map((item) => (item.id === conversation.id ? response.conversation : item));
    if (selectedConversationId === conversation.id) {
      selectedConversationId = '';
      messages = [];
    }
  }

  async function deleteConversation(conversation: Conversation): Promise<void> {
    if (!confirm(`Delete "${conversation.title}"?`)) {
      return;
    }
    await deleteJSON<{ ok: boolean }>(`/api/v1/conversations/${conversation.id}`);
    conversations = conversations.filter((item) => item.id !== conversation.id);
    if (selectedConversationId === conversation.id) {
      selectedConversationId = '';
      messages = [];
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
    taskRunToolCalls = response.tool_calls ?? [];
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
  <AuthView
    errorMessage={errorMessage}
    mode="loading"
    notice={notice}
    onLogin={submitLogin}
    onSetup={submitSetup}
    submitting={submitting}
  />
{:else if setupAvailable}
  <AuthView
    bind:setupConfirmPassword
    bind:setupDisplayName
    bind:setupEmail
    bind:setupPassword
    errorMessage={errorMessage}
    mode="setup"
    notice={notice}
    onLogin={submitLogin}
    onSetup={submitSetup}
    submitting={submitting}
  />
{:else if !user}
  <AuthView
    bind:loginEmail
    bind:loginPassword
    errorMessage={errorMessage}
    mode="login"
    notice={notice}
    onLogin={submitLogin}
    onSetup={submitSetup}
    submitting={submitting}
  />
{:else}
  <AppShell
    bind:activeView
    {conversations}
    {navItems}
    onArchiveConversation={archiveConversation}
    onCreateConversation={createConversation}
    onDeleteConversation={deleteConversation}
    onLogout={logout}
    onRenameConversation={renameConversation}
    onSelectConversation={selectConversation}
    {selectedConversationId}
    {status}
    {submitting}
    {user}
  >
    {#if notice}
      <Notice tone="success">{notice}</Notice>
    {/if}
    {#if errorMessage}
      <Notice tone="error">{errorMessage}</Notice>
    {/if}

      {#if activeView === strings.nav.settings}
        <SettingsView
          bind:replyPresetDescription
          bind:replyPresetInstruction
          bind:replyPresetName
          {feedbackStats}
          onCreateReplyPreset={createReplyPreset}
          onRefreshDiagnostics={refreshDiagnostics}
          onRefreshFeedbackStats={refreshFeedbackStats}
          onRefreshSessions={refreshSessions}
          onResetReplyPresets={resetReplyPresets}
          onRevokeSession={revokeSession}
          onSaveModelRole={saveModelRole}
          onToggleReplyPreset={toggleReplyPreset}
          {providerModels}
          {providers}
          {replyPresets}
          {sessions}
          {status}
          {submitting}
          bind:chatRoleModel
          bind:chatRoleProviderId
          bind:utilityRoleModel
          bind:utilityRoleProviderId
          bind:visionRoleModel
          bind:visionRoleProviderId
          {user}
        />
      {:else}
        {#if activeView === strings.nav.chat}
          <ChatView
            {activeRunId}
            {agents}
            bind:composer
            {feedbackByMessage}
            {messages}
            bind:negativeFeedbackReason
            onApproveToolCall={approveToolCall}
            onClearFeedback={clearFeedback}
            onClearSummary={clearSummary}
            onDenyToolCall={denyToolCall}
            onGenerateReplyDraft={generateReplyDraft}
            onInsertReplyDraft={insertReplyDraft}
            onRefreshModels={refreshModels}
            onRefreshPendingToolApprovals={refreshPendingToolApprovals}
            onRegenerateSummary={regenerateSummary}
            onRegenerateWithFeedback={regenerateWithFeedback}
            onRememberMessage={rememberMessage}
            onSelectReplySource={selectReplySource}
            onSendMessage={sendMessage}
            onStopGeneration={stopGeneration}
            onSubmitFeedback={submitFeedback}
            {pendingToolApprovals}
            {providerModels}
            {providers}
            bind:replyCustomInstruction
            bind:replyDraft
            {replyPresets}
            renderMarkdown={renderMarkdown}
            {runMemories}
            {selectedConversation}
            bind:selectedAgentId
            bind:selectedModel
            bind:selectedProviderId
            bind:selectedReplyPresetId
            bind:selectedReplySourceId
            {submitting}
            {toolCards}
          />
        {:else if activeView === strings.nav.agents}
          <AgentsView
            bind:agentActive
            bind:agentAvatar
            bind:agentDefaultModel
            bind:agentDefaultProviderId
            bind:agentDescription
            bind:agentFallbackModel
            bind:agentMaxToolIterations
            bind:agentMemoryMode
            bind:agentName
            bind:agentPrompt
            bind:agentTemperature
            bind:agentToolPermissionDefault
            {agents}
            editingAgentId={editingAgentId}
            onCancelEdit={resetAgentForm}
            onDelete={deleteAgent}
            onDuplicate={duplicateAgent}
            onEdit={editAgent}
            onRefresh={refreshAgents}
            onSubmit={createAgent}
            {providers}
          />
        {:else if activeView === strings.nav.memories}
          <MemoriesView
            bind:memoryActive
            bind:memoryContent
            bind:memoryImportance
            bind:memoryPinned
            bind:memoryScope
            bind:memoryTags
            bind:memoryTitle
            editingMemoryId={editingMemoryId}
            {memories}
            onCancelEdit={resetMemoryForm}
            onDelete={deleteMemory}
            onEdit={editMemory}
            onRefresh={refreshMemories}
            onSubmit={createMemory}
          />
        {:else if activeView === strings.nav.tasks}
          <TasksView
            {agents}
            bind:taskAgentId
            bind:taskConcurrencyPolicy
            bind:taskCronExpression
            bind:taskDescription
            editingTaskId={editingTaskId}
            bind:taskIntervalSeconds
            bind:taskMaxRetries
            bind:taskModel
            bind:taskName
            bind:taskPrompt
            bind:taskProviderId
            {taskRecords}
            bind:taskRunAt
            {taskRunEvents}
            {taskRunToolCalls}
            taskNameForRun={taskNameForRun}
            {taskRuns}
            bind:taskScheduleMode
            bind:taskState
            bind:taskTimeoutMS
            bind:taskToolPolicy
            bind:taskType
            onCancelEdit={resetTaskForm}
            onCancelRun={cancelTaskRun}
            onDelete={deleteTask}
            onEdit={editTask}
            onRefresh={refreshTasks}
            onRetryRun={retryTaskRun}
            onRunTask={runTask}
            onShowEvents={showTaskRunEvents}
            onSubmit={createTask}
            {providers}
            {submitting}
          />
        {:else if activeView === strings.nav.mcp}
          <MCPView
            bind:mcpArguments
            bind:mcpAuthorization
            bind:mcpCommand
            bind:mcpDescription
            bind:mcpEnabled
            bind:mcpEnvironment
            bind:mcpHttpHeaders
            bind:mcpHttpUrl
            bind:mcpName
            bind:mcpRequestTimeoutMS
            {mcpServers}
            bind:mcpStartupTimeoutMS
            {mcpTools}
            bind:mcpTransport
            bind:mcpWorkingDirectory
            editingMCPServerId={editingMCPServerId}
            onCancelEdit={resetMCPServerForm}
            onDelete={deleteMCPServer}
            onDiscoverTools={discoverMCPTools}
            onEdit={editMCPServer}
            onRefresh={refreshMCP}
            onSubmit={createMCPServer}
            onTest={testMCPServer}
            onUpdateToolPermission={updateToolPermission}
          />
        {:else if activeView === strings.nav.providers}
          <ProvidersView
            bind:providerApiKey
            bind:providerApiKeyEnvRef
            bind:providerBaseUrl
            bind:providerCustomHeaders
            bind:providerDefaultModel
            bind:providerEnabled
            bind:providerFallbackModel
            bind:providerName
            bind:providerOrganization
            bind:providerProject
            bind:providerTimeoutMS
            editingProviderId={editingProviderId}
            onCancelEdit={resetProviderForm}
            onDelete={deleteProvider}
            onEdit={editProvider}
            onRefresh={refreshProviders}
            onRefreshModels={refreshProviderModels}
            onSubmit={createProvider}
            onTest={testProvider}
            {providers}
            {submitting}
          />
        {:else}
          <section class="panel" aria-labelledby="screen-title">
            <p class="eyebrow">{strings.workspace.title}</p>
            <h2 id="screen-title">{activeView}</h2>
            <p>{strings.workspace.emptyScreen}</p>
          </section>
        {/if}
      {/if}
  </AppShell>
{/if}
