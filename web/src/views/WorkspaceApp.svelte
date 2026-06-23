<script lang="ts">
  import { onMount } from 'svelte';
  import DOMPurify from 'dompurify';
  import { marked } from 'marked';
  import AppShell from '../app/shell/AppShell.svelte';
  import Notice from '../components/common/Notice.svelte';
  import ToastCenter, { type AppToast, type ToastType } from '../components/common/ToastCenter.svelte';
  import WorkspaceWindow from '../components/common/WorkspaceWindow.svelte';
  import type { IconName } from '../components/common/Icon.svelte';
  import AgentsView from './AgentsView.svelte';
  import AuthView from './AuthView.svelte';
  import ChatView from './ChatView.svelte';
  import MCPView from './MCPView.svelte';
  import MemoriesView from './MemoriesView.svelte';
  import SettingsView from './SettingsView.svelte';
  import TasksView from './TasksView.svelte';
  import { deleteJSON, getJSON, patchStream, postJSON, postStream, putJSON } from '../lib/api';
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
	    ModelRoleDraft,
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
    strings.nav.mcp
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
  let chatRoleEntries: ModelRoleDraft[] = emptyRoleEntries();
  let utilityRoleEntries: ModelRoleDraft[] = emptyRoleEntries();
  let visionRoleEntries: ModelRoleDraft[] = emptyRoleEntries();
  let composer = '';
  let activeRunId = '';
  let activeView: string = strings.nav.chat;
  let minimizedView = '';
  let loading = true;
  let submitting = false;
  let notice = '';
  let errorMessage = '';
  let streamState: 'idle' | 'connecting' | 'streaming' | 'completed' | 'failed' | 'cancelled' = 'idle';
  let toasts: AppToast[] = [];
  let actionStates: Record<string, string> = {};
  let toastCounter = 0;

  $: selectedConversation = conversations.find((conversation) => conversation.id === selectedConversationId);

  function emptyRoleEntries(): ModelRoleDraft[] {
    return [
      { provider_id: '', model_id: '' },
      { provider_id: '', model_id: '' },
      { provider_id: '', model_id: '' }
    ];
  }

  function roleEntriesFor(role: 'chat' | 'utility' | 'vision'): ModelRoleDraft[] {
    const bindings = modelRoles
      .filter((binding) => binding.role === role)
      .sort((left, right) => left.position - right.position)
      .slice(0, 3)
      .map((binding) => ({ provider_id: binding.provider_id, model_id: binding.model_id }));
    while (bindings.length < 3) {
      bindings.push({ provider_id: '', model_id: '' });
    }
    return bindings;
  }

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

  function addToast(type: ToastType, message: string, options: Partial<AppToast> & { timeoutMS?: number } = {}): string {
    const id = `toast-${Date.now()}-${toastCounter}`;
    toastCounter += 1;
    const toast: AppToast = {
      id,
      type,
      message,
      actionLabel: options.actionLabel,
      onAction: options.onAction,
      persistent: options.persistent
    };
    toasts = [toast, ...toasts].slice(0, 5);
    const timeoutMS =
      options.timeoutMS ??
      (type === 'success' ? 3200 : type === 'info' ? 4200 : type === 'warning' ? 6500 : type === 'loading' ? 0 : 0);
    if (timeoutMS > 0 && !toast.persistent) {
      window.setTimeout(() => dismissToast(id), timeoutMS);
    }
    return id;
  }

  function dismissToast(toastId: string): void {
    toasts = toasts.filter((toast) => toast.id !== toastId);
  }

  function notifySuccess(message: string): void {
    notice = message;
    addToast('success', message);
  }

  function notifyInfo(message: string): void {
    notice = message;
    addToast('info', message);
  }

  function notifyError(message: string, retry: (() => void | Promise<void>) | undefined = undefined): void {
    errorMessage = message;
    addToast('error', message, {
      actionLabel: retry ? 'Retry' : undefined,
      onAction: retry,
      persistent: true
    });
  }

  function setActionState(key: string, state: string): void {
    actionStates = { ...actionStates, [key]: state };
  }

  function clearActionState(key: string, state: string, delayMS = 1800): void {
    window.setTimeout(() => {
      if (actionStates[key] !== state) return;
      const next = { ...actionStates };
      delete next[key];
      actionStates = next;
    }, delayMS);
  }

  function applyProviderRefreshStatus(refresh: ModelRefreshResponse['refresh']): void {
    providers = providers.map((provider) =>
      provider.id === refresh.provider_id
        ? {
            ...provider,
            model_refresh_state: refresh.state,
            model_refresh_started_at: refresh.started_at,
            model_refresh_completed_at: refresh.completed_at,
            model_refresh_duration_ms: refresh.duration_ms,
            model_refresh_error_category: refresh.error_category,
            model_refresh_error_message: refresh.error_message,
            model_count: refresh.cached_model_count,
            available_model_count: refresh.available_model_count,
            unavailable_model_count: refresh.unavailable_model_count
          }
        : provider
    );
  }

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
    const response = await getJSON<ModelsResponse>(`/api/v1/models?limit=1000&include_unavailable=true${providerQuery}`);
    providerModels = response.models ?? [];
    if ((!selectedProviderId || !selectedModel) && providerModels.length > 0) {
      selectedProviderId = selectedProviderId || providerModels[0].provider_id;
      selectedModel = selectedModel || providerModels[0].model_id;
    }
  }

  async function refreshModelRoles(): Promise<void> {
    const response = await getJSON<ModelRolesResponse>('/api/v1/model-roles');
    modelRoles = response.roles ?? [];
    chatRoleEntries = roleEntriesFor('chat');
    utilityRoleEntries = roleEntriesFor('utility');
    visionRoleEntries = roleEntriesFor('vision');
  }

  async function saveModelRole(role: 'chat' | 'utility' | 'vision', entries: ModelRoleDraft[]): Promise<void> {
    const stateKey = `model-role:${role}`;
    const hasIncompleteEntry = entries.some((entry) => {
      const hasProvider = Boolean(entry.provider_id.trim());
      const hasModel = Boolean(entry.model_id.trim());
      return hasProvider !== hasModel;
    });
    if (hasIncompleteEntry) {
      setActionState(stateKey, 'failed');
      notifyError('Each model role entry must include both a provider and a model ID.');
      return;
    }
    const models = entries
      .map((entry) => ({ provider_id: entry.provider_id.trim(), model_id: entry.model_id.trim() }))
      .filter((entry) => entry.provider_id && entry.model_id);
    setActionState(stateKey, 'saving');
    try {
      const response = await putJSON<ModelRolesResponse>(`/api/v1/model-roles/${role}`, {
        models
      });
      modelRoles = response.roles ?? [];
      await refreshModelRoles();
      setActionState(stateKey, 'saved');
      clearActionState(stateKey, 'saved');
      notifySuccess(`${role} model chain saved.`);
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => saveModelRole(role, entries));
    }
  }

  async function createProvider(): Promise<void> {
    const stateKey = 'provider-form';
    submitting = true;
    setActionState(stateKey, 'saving');
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
      providers = providerBeingEdited
        ? providers.map((provider) => (provider.id === response.provider.id ? response.provider : provider))
        : [response.provider, ...providers];
      resetProviderForm();
      selectedProviderId = response.provider.id;
      selectedModel = response.provider.default_model ?? '';
      await refreshProviders();
      setActionState(stateKey, 'saved');
      clearActionState(stateKey, 'saved');
      notifySuccess(providerBeingEdited ? 'Provider updated.' : 'Provider saved.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), createProvider);
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
    setActionState(`provider:${providerId}:delete`, 'deleting');
    errorMessage = '';
    try {
      await deleteJSON<{ ok: boolean }>(`/api/v1/providers/${providerId}`);
      providers = providers.filter((provider) => provider.id !== providerId);
      providerModels = providerModels.filter((model) => model.provider_id !== providerId);
      if (selectedProviderId === providerId) {
        selectedProviderId = '';
        selectedModel = '';
      }
      if (editingProviderId === providerId) {
        resetProviderForm();
      }
      setActionState(`provider:${providerId}:delete`, 'deleted');
      clearActionState(`provider:${providerId}:delete`, 'deleted');
      notifySuccess('Provider deleted.');
    } catch (error) {
      setActionState(`provider:${providerId}:delete`, 'failed');
      notifyError(messageFromError(error), () => deleteProvider(providerId));
    } finally {
      submitting = false;
    }
  }

  async function toggleProviderEnabled(provider: Provider): Promise<void> {
    submitting = true;
    setActionState(`provider:${provider.id}:toggle`, provider.enabled ? 'disabling' : 'enabling');
    errorMessage = '';
    try {
      const response = await putJSON<ProviderResponse>(`/api/v1/providers/${provider.id}`, {
        name: provider.name,
        base_url: provider.base_url,
        api_key_env_ref: provider.api_key_env_ref ?? '',
        organization_header: provider.organization_header ?? '',
        project_header: provider.project_header ?? '',
        custom_headers: provider.custom_headers ?? {},
        enabled: !provider.enabled,
        request_timeout_ms: provider.request_timeout_ms,
        default_model: provider.default_model ?? '',
        fallback_model: provider.fallback_model ?? ''
      });
      providers = providers.map((item) => (item.id === provider.id ? response.provider : item));
      setActionState(`provider:${provider.id}:toggle`, response.provider.enabled ? 'enabled' : 'disabled');
      clearActionState(`provider:${provider.id}:toggle`, response.provider.enabled ? 'enabled' : 'disabled');
      notifySuccess(response.provider.enabled ? 'Provider enabled.' : 'Provider disabled.');
    } catch (error) {
      setActionState(`provider:${provider.id}:toggle`, 'failed');
      notifyError(messageFromError(error), () => toggleProviderEnabled(provider));
    } finally {
      submitting = false;
    }
  }

  async function testProvider(providerId: string): Promise<void> {
    submitting = true;
    const stateKey = `provider:${providerId}:test`;
    setActionState(stateKey, 'testing');
    errorMessage = '';
    try {
      await postJSON<{ ok: boolean }>(`/api/v1/providers/${providerId}/test`);
      await refreshProviders();
      setActionState(stateKey, 'connected');
      clearActionState(stateKey, 'connected');
      notifySuccess('Provider connection succeeded.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => testProvider(providerId));
    } finally {
      submitting = false;
    }
  }

  async function refreshProviderModels(providerId: string): Promise<void> {
    submitting = true;
    const stateKey = `provider:${providerId}:models`;
    setActionState(stateKey, 'refreshing');
    errorMessage = '';
    try {
      selectedProviderId = providerId;
      const started = await postJSON<ModelRefreshResponse>(`/api/v1/providers/${providerId}/models/refresh`);
      applyProviderRefreshStatus(started.refresh);
      notifyInfo('Model refresh started.');
      for (let attempt = 0; attempt < 60; attempt += 1) {
        await new Promise((resolve) => window.setTimeout(resolve, 1000));
        const status = await getJSON<ModelRefreshResponse>(`/api/v1/providers/${providerId}/models/refresh-status`);
        applyProviderRefreshStatus(status.refresh);
        if (status.refresh.state === 'succeeded') {
          await refreshModels();
          const firstModel = providerModels.find((model) => model.provider_id === providerId);
          if (firstModel) {
            selectedModel = firstModel.model_id;
          }
          await refreshProviders();
          setActionState(stateKey, 'succeeded');
          clearActionState(stateKey, 'succeeded');
          notifySuccess('Models refreshed.');
          return;
        }
        if (status.refresh.state === 'failed') {
          throw new Error(status.refresh.error_message || 'Model refresh failed.');
        }
      }
      await refreshModels();
      await refreshProviders();
      setActionState(stateKey, 'refreshing');
      notifyInfo('Model refresh is still running.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => refreshProviderModels(providerId));
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

  async function unarchiveConversation(conversation: Conversation): Promise<void> {
    const response = await putJSON<ConversationResponse>(`/api/v1/conversations/${conversation.id}`, { archive: false });
    conversations = conversations.map((item) => (item.id === conversation.id ? response.conversation : item));
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
    streamState = 'connecting';
    try {
      await postStream(
        `/api/v1/conversations/${selectedConversationId}/runs`,
        { content, provider_id: selectedProviderId, model: selectedModel },
        handleChatEvent
      );
      activeRunId = '';
      const finalStreamState = streamState as string;
      if (finalStreamState !== 'failed' && finalStreamState !== 'cancelled') {
        streamState = 'completed';
      }
      await Promise.all([refreshConversations(), selectConversation(selectedConversationId)]);
    } catch (error) {
      if (!activeRunId && !messages.some((message) => message.content === content && message.role === 'user')) {
        composer = content;
      }
      activeRunId = '';
      streamState = 'failed';
      notifyError(messageFromError(error), sendMessage);
    }
  }

  async function stopGeneration(): Promise<void> {
    if (!activeRunId) {
      return;
    }
    try {
      await postJSON<{ ok: boolean }>(`/api/v1/chat-runs/${activeRunId}/cancel`);
      streamState = 'cancelled';
      notifyInfo('Generation stopped.');
    } catch (error) {
      notifyError(messageFromError(error), stopGeneration);
    }
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
      streamState = 'streaming';
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
      streamState = event === 'run_completed' ? 'completed' : event === 'run_cancelled' ? 'cancelled' : 'failed';
      if (event === 'run_failed') {
        notifyError('Generation failed. The conversation was kept so you can retry.');
      }
      if (event === 'run_cancelled') {
        notifyInfo('Generation stopped.');
      }
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
    const stateKey = 'agent-form';
    setActionState(stateKey, 'saving');
    try {
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
      setActionState(stateKey, 'saved');
      clearActionState(stateKey, 'saved');
      notifySuccess(agentBeingEdited ? 'Agent updated.' : 'Agent saved.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), createAgent);
    }
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
    setActionState(`agent:${agentId}:duplicate`, 'saving');
    try {
      const response = await postJSON<AgentResponse>(`/api/v1/agents/${agentId}/duplicate`);
      agents = [response.agent, ...agents];
      setActionState(`agent:${agentId}:duplicate`, 'saved');
      clearActionState(`agent:${agentId}:duplicate`, 'saved');
      notifySuccess('Agent duplicated.');
    } catch (error) {
      setActionState(`agent:${agentId}:duplicate`, 'failed');
      notifyError(messageFromError(error), () => duplicateAgent(agentId));
    }
  }

  async function deleteAgent(agentId: string): Promise<void> {
    if (!confirm('Delete this agent? Existing conversations keep their messages.')) {
      return;
    }
    setActionState(`agent:${agentId}:delete`, 'deleting');
    try {
      await deleteJSON<{ ok: boolean }>(`/api/v1/agents/${agentId}`);
      agents = agents.filter((agent) => agent.id !== agentId);
      if (selectedAgentId === agentId) {
        selectedAgentId = agents[0]?.id ?? '';
      }
      setActionState(`agent:${agentId}:delete`, 'deleted');
      clearActionState(`agent:${agentId}:delete`, 'deleted');
      notifySuccess('Agent deleted.');
    } catch (error) {
      setActionState(`agent:${agentId}:delete`, 'failed');
      notifyError(messageFromError(error), () => deleteAgent(agentId));
    }
  }

  async function toggleAgentActive(agent: Agent): Promise<void> {
    const stateKey = `agent:${agent.id}:toggle`;
    setActionState(stateKey, agent.active ? 'disabling' : 'enabling');
    try {
      const response = await putJSON<AgentResponse>(`/api/v1/agents/${agent.id}`, {
        name: agent.name,
        description: agent.description,
        avatar: agent.avatar,
        system_prompt: agent.system_prompt,
        default_provider_id: agent.default_provider_id ?? '',
        default_model: agent.default_model ?? '',
        fallback_model: agent.fallback_model ?? '',
        temperature: agent.temperature,
        max_tool_iterations: agent.max_tool_iterations,
        memory_access_mode: agent.memory_access_mode,
        tool_permission_default: agent.tool_permission_default,
        active: !agent.active
      });
      agents = agents.map((item) => (item.id === agent.id ? response.agent : item));
      setActionState(stateKey, response.agent.active ? 'enabled' : 'disabled');
      clearActionState(stateKey, response.agent.active ? 'enabled' : 'disabled');
      notifySuccess(response.agent.active ? 'Agent enabled.' : 'Agent disabled.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => toggleAgentActive(agent));
    }
  }

  function testAgent(agent: Agent): void {
    selectedAgentId = agent.id;
    minimizedView = '';
    activeView = strings.nav.chat;
    notice = `Testing ${agent.name} in chat.`;
  }

  async function refreshMemories(): Promise<void> {
    const response = await getJSON<MemoriesResponse>('/api/v1/memories');
    memories = response.memories ?? [];
  }

  async function createMemory(): Promise<void> {
    const stateKey = 'memory-form';
    setActionState(stateKey, 'saving');
    try {
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
      setActionState(stateKey, 'saved');
      clearActionState(stateKey, 'saved');
      notifySuccess(memoryBeingEdited ? 'Memory updated.' : 'Memory saved.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), createMemory);
    }
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
    setActionState(`memory:${memoryId}:delete`, 'deleting');
    try {
      await deleteJSON<{ ok: boolean }>(`/api/v1/memories/${memoryId}`);
      if (editingMemoryId === memoryId) {
        resetMemoryForm();
      }
      memories = memories.filter((memory) => memory.id !== memoryId);
      setActionState(`memory:${memoryId}:delete`, 'deleted');
      clearActionState(`memory:${memoryId}:delete`, 'deleted');
      notifySuccess('Memory deleted.');
    } catch (error) {
      setActionState(`memory:${memoryId}:delete`, 'failed');
      notifyError(messageFromError(error), () => deleteMemory(memoryId));
    }
  }

  async function updateMemoryFlags(memory: Memory, flags: Pick<Memory, 'active' | 'pinned'>): Promise<void> {
    const response = await putJSON<MemoryResponse>(`/api/v1/memories/${memory.id}`, {
      title: memory.title,
      content: memory.content,
      tags: memory.tags,
      scope: memory.scope,
      importance: memory.importance,
      pinned: flags.pinned,
      active: flags.active,
      source: memory.source,
      source_message_id: memory.source_message_id ?? '',
      agent_id: memory.agent_id ?? '',
      conversation_id: memory.conversation_id ?? ''
    });
    memories = memories.map((item) => (item.id === memory.id ? response.memory : item));
  }

  async function toggleMemoryActive(memory: Memory): Promise<void> {
    const stateKey = `memory:${memory.id}:active`;
    setActionState(stateKey, memory.active ? 'disabling' : 'enabling');
    try {
      await updateMemoryFlags(memory, { active: !memory.active, pinned: memory.pinned });
      setActionState(stateKey, memory.active ? 'disabled' : 'enabled');
      clearActionState(stateKey, memory.active ? 'disabled' : 'enabled');
      notifySuccess(memory.active ? 'Memory disabled.' : 'Memory enabled.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => toggleMemoryActive(memory));
    }
  }

  async function toggleMemoryPinned(memory: Memory): Promise<void> {
    const stateKey = `memory:${memory.id}:pinned`;
    setActionState(stateKey, memory.pinned ? 'unpinning' : 'pinning');
    try {
      await updateMemoryFlags(memory, { active: memory.active, pinned: !memory.pinned });
      setActionState(stateKey, memory.pinned ? 'unpinned' : 'pinned');
      clearActionState(stateKey, memory.pinned ? 'unpinned' : 'pinned');
      notifySuccess(memory.pinned ? 'Memory unpinned.' : 'Memory pinned.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => toggleMemoryPinned(memory));
    }
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
    notifySuccess('Memory created from message.');
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
    const stateKey = 'mcp-form';
    setActionState(stateKey, 'saving');
    try {
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
      setActionState(stateKey, 'saved');
      clearActionState(stateKey, 'saved');
      notifySuccess(serverBeingEdited ? 'MCP server updated.' : 'MCP server saved.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), createMCPServer);
    }
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
    setActionState(`mcp:${serverId}:delete`, 'deleting');
    try {
      await deleteJSON<{ ok: boolean }>(`/api/v1/mcp-servers/${serverId}`);
      if (editingMCPServerId === serverId) {
        resetMCPServerForm();
      }
      mcpServers = mcpServers.filter((server) => server.id !== serverId);
      mcpTools = mcpTools.filter((tool) => tool.server_id !== serverId);
      setActionState(`mcp:${serverId}:delete`, 'deleted');
      clearActionState(`mcp:${serverId}:delete`, 'deleted');
      notifySuccess('MCP server deleted.');
    } catch (error) {
      setActionState(`mcp:${serverId}:delete`, 'failed');
      notifyError(messageFromError(error), () => deleteMCPServer(serverId));
    }
  }

  async function discoverMCPTools(serverId: string): Promise<void> {
    const stateKey = `mcp:${serverId}:discover`;
    setActionState(stateKey, 'discovering');
    try {
      const response = await postJSON<MCPToolsResponse>(`/api/v1/mcp-servers/${serverId}/discover`);
      mcpTools = [...(response.tools ?? []), ...mcpTools.filter((tool) => tool.server_id !== serverId)];
      await refreshMCP();
      setActionState(stateKey, 'succeeded');
      clearActionState(stateKey, 'succeeded');
      notifySuccess('MCP tools discovered.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => discoverMCPTools(serverId));
    }
  }

  async function testMCPServer(serverId: string): Promise<void> {
    const stateKey = `mcp:${serverId}:test`;
    setActionState(stateKey, 'testing');
    try {
      await postJSON<MCPToolsResponse>(`/api/v1/mcp-servers/${serverId}/test`);
      await refreshMCP();
      setActionState(stateKey, 'connected');
      clearActionState(stateKey, 'connected');
      notifySuccess('MCP server connection tested.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => testMCPServer(serverId));
    }
  }

  async function updateToolPermission(toolId: string, permissionMode: string): Promise<void> {
    const previousTools = mcpTools;
    mcpTools = mcpTools.map((tool) => (tool.id === toolId ? { ...tool, permission_mode: permissionMode } : tool));
    try {
      await putJSON<{ ok: boolean }>(`/api/v1/mcp-tools/${toolId}/permission`, { permission_mode: permissionMode });
      notifySuccess('Tool permission updated.');
    } catch (error) {
      mcpTools = previousTools;
      notifyError(messageFromError(error), () => updateToolPermission(toolId, permissionMode));
    }
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
    const stateKey = 'task-form';
    submitting = true;
    setActionState(stateKey, 'saving');
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
      setActionState(stateKey, 'saved');
      clearActionState(stateKey, 'saved');
      notifySuccess(taskBeingEdited ? 'Task updated.' : 'Task saved.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), createTask);
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
    setActionState(`task:${taskId}:delete`, 'deleting');
    try {
      await deleteJSON<{ ok: boolean }>(`/api/v1/tasks/${taskId}`);
      if (editingTaskId === taskId) {
        resetTaskForm();
      }
      taskRecords = taskRecords.filter((record) => record.task.id !== taskId);
      taskRuns = taskRuns.filter((run) => run.task_id !== taskId);
      setActionState(`task:${taskId}:delete`, 'deleted');
      clearActionState(`task:${taskId}:delete`, 'deleted');
      notifySuccess('Task deleted.');
    } catch (error) {
      setActionState(`task:${taskId}:delete`, 'failed');
      notifyError(messageFromError(error), () => deleteTask(taskId));
    }
  }

  async function toggleTaskState(record: TaskRecord): Promise<void> {
    const nextState = record.task.state === 'enabled' ? 'disabled' : 'enabled';
    const stateKey = `task:${record.task.id}:toggle`;
    setActionState(stateKey, nextState === 'enabled' ? 'enabling' : 'disabling');
    try {
      const response = await putJSON<TaskResponse>(`/api/v1/tasks/${record.task.id}`, {
        name: record.task.name,
        description: record.task.description,
        task_type: record.task.task_type,
        state: nextState,
        agent_id: record.task.agent_id ?? '',
        provider_id: record.task.provider_id ?? '',
        model: record.task.model ?? '',
        prompt: record.task.prompt,
        tool_policy: record.task.tool_policy,
        max_retries: record.task.max_retries,
        timeout_ms: record.task.timeout_ms,
        concurrency_policy: record.task.concurrency_policy,
        schedule_mode: record.schedule.mode,
        cron_expression: record.schedule.cron_expression ?? '',
        interval_seconds: record.schedule.interval_seconds ?? 0,
        run_at: record.schedule.run_at ?? '',
        timezone: record.schedule.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
      });
      const nextRecord = { task: response.task, schedule: response.schedule };
      taskRecords = taskRecords.map((item) => (item.task.id === response.task.id ? nextRecord : item));
      setActionState(stateKey, nextState);
      clearActionState(stateKey, nextState);
      notifySuccess(nextState === 'enabled' ? 'Task enabled.' : 'Task disabled.');
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => toggleTaskState(record));
    }
  }

  async function runTask(taskId: string): Promise<void> {
    const stateKey = `task:${taskId}:run`;
    setActionState(stateKey, 'queued');
    try {
      const response = await postJSON<TaskRunResponse>(`/api/v1/tasks/${taskId}/run`);
      taskRuns = [response.run, ...taskRuns];
      notifyInfo('Task queued.');
      void pollTaskRun(response.run.id, stateKey);
    } catch (error) {
      setActionState(stateKey, 'failed');
      notifyError(messageFromError(error), () => runTask(taskId));
    }
  }

  async function cancelTaskRun(runId: string): Promise<void> {
    try {
      await postJSON<{ ok: boolean }>(`/api/v1/task-runs/${runId}/cancel`);
      taskRuns = taskRuns.map((run) => (run.id === runId ? { ...run, state: 'cancelled', completed_at: new Date().toISOString() } : run));
      notifyInfo('Task run cancelled.');
    } catch (error) {
      notifyError(messageFromError(error), () => cancelTaskRun(runId));
    }
  }

  async function retryTaskRun(runId: string): Promise<void> {
    setActionState(`task-run:${runId}:retry`, 'queued');
    try {
      const response = await postJSON<TaskRunResponse>(`/api/v1/task-runs/${runId}/retry`);
      taskRuns = [response.run, ...taskRuns];
      notifyInfo('Task retry queued.');
      void pollTaskRun(response.run.id, `task-run:${runId}:retry`);
    } catch (error) {
      setActionState(`task-run:${runId}:retry`, 'failed');
      notifyError(messageFromError(error), () => retryTaskRun(runId));
    }
  }

  async function pollTaskRun(runId: string, stateKey: string): Promise<void> {
    for (let attempt = 0; attempt < 90; attempt += 1) {
      await new Promise((resolve) => window.setTimeout(resolve, 1500));
      try {
        const response = await getJSON<TaskRunRecordResponse>(`/api/v1/task-runs/${runId}`);
        taskRuns = taskRuns.map((run) => (run.id === runId ? response.run : run));
        setActionState(stateKey, response.run.state);
        if (!['queued', 'claimed', 'running', 'waiting'].includes(response.run.state)) {
          clearActionState(stateKey, response.run.state);
          if (response.run.state === 'succeeded') {
            notifySuccess('Task completed.');
          } else if (response.run.state === 'failed' || response.run.state === 'timed_out') {
            notifyError(response.run.error_message || 'Task failed.', () => retryTaskRun(runId));
          }
          return;
        }
      } catch (error) {
        setActionState(stateKey, 'failed');
        notifyError(messageFromError(error), () => pollTaskRun(runId, stateKey));
        return;
      }
    }
    notifyInfo('Task is still running.');
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

  async function editMessage(message: Message): Promise<void> {
    const content = window.prompt('Edit message and branch from here', message.content);
    if (!content || content.trim() === message.content.trim()) {
      return;
    }
    await patchStream(
      `/api/v1/messages/${message.id}`,
      { content: content.trim(), provider_id: selectedProviderId, model: selectedModel },
      handleChatEvent
    );
    activeRunId = '';
    await Promise.all([selectConversation(selectedConversationId), refreshConversations()]);
  }

  async function regenerateWithFeedback(message: Message, instructionOverride = ''): Promise<void> {
    const feedback = feedbackByMessage[message.id];
    const instruction =
      instructionOverride ||
      (feedback?.rating === 'negative'
        ? `Address this feedback reason: ${feedback.reason || negativeFeedbackReason}. Preserve the original user intent and produce a better answer.`
        : 'Regenerate the response with a clearer and more useful answer.');
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

  function windowIcon(view: string): IconName {
    if (view === strings.nav.agents) return 'agent';
    if (view === strings.nav.memories) return 'brain';
    if (view === strings.nav.tasks) return 'tasks';
    if (view === strings.nav.mcp) return 'tools';
    if (view === strings.nav.settings) return 'gear';
    return 'window';
  }

  function windowSize(view: string): { width: number; height: number } {
    if (view === strings.nav.settings) return { width: 980, height: 700 };
    if (view === strings.nav.tasks) return { width: 860, height: 660 };
    if (view === strings.nav.mcp) return { width: 860, height: 640 };
    if (view === strings.nav.agents) return { width: 820, height: 640 };
    return { width: 800, height: 620 };
  }

  function closeWorkspaceWindow(): void {
    minimizedView = '';
    activeView = strings.nav.chat;
  }

  function minimizeWorkspaceWindow(): void {
    minimizedView = activeView;
    activeView = strings.nav.chat;
  }

  function restoreWorkspaceWindow(view: string): void {
    minimizedView = '';
    activeView = view;
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
    {minimizedView}
    {navItems}
    onArchiveConversation={archiveConversation}
    onCreateConversation={createConversation}
    onDeleteConversation={deleteConversation}
    onLogout={logout}
    onRenameConversation={renameConversation}
    onRestoreWindow={restoreWorkspaceWindow}
    onSelectConversation={selectConversation}
    onUnarchiveConversation={unarchiveConversation}
    {selectedConversationId}
    {submitting}
    {user}
  >
    <ToastCenter {toasts} onDismiss={dismissToast} />

    <div class="workspace-notices">
      {#if notice}
        <Notice tone="success">{notice}</Notice>
      {/if}
      {#if errorMessage}
        <Notice tone="error">{errorMessage}</Notice>
      {/if}
    </div>

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
      onEditMessage={editMessage}
      onGenerateReplyDraft={generateReplyDraft}
      onInsertReplyDraft={insertReplyDraft}
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
      {streamState}
      {submitting}
      {toolCards}
    />

    {#if activeView !== strings.nav.chat && minimizedView !== activeView}
      <WorkspaceWindow
        height={windowSize(activeView).height}
        icon={windowIcon(activeView)}
        id={`workspace-${activeView.toLowerCase().replaceAll(' ', '-')}`}
        onActivate={() => undefined}
        onClose={closeWorkspaceWindow}
        onMinimize={minimizeWorkspaceWindow}
        title={activeView}
        width={windowSize(activeView).width}
      >
        {#if activeView === strings.nav.settings}
          <SettingsView
            bind:replyPresetDescription
            bind:replyPresetInstruction
            bind:replyPresetName
            {feedbackStats}
            {actionStates}
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
            bind:chatRoleEntries
            bind:utilityRoleEntries
            bind:visionRoleEntries
            {user}
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
            onCancelProviderEdit={resetProviderForm}
            onDeleteProvider={deleteProvider}
            onEditProvider={editProvider}
            onRefreshProviders={refreshProviders}
            onRefreshProviderModels={refreshProviderModels}
            onSubmitProvider={createProvider}
            onTestProvider={testProvider}
            onToggleProviderEnabled={toggleProviderEnabled}
          />
        {:else if activeView === strings.nav.agents}
          <AgentsView
            {actionStates}
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
            onTest={testAgent}
            onToggleActive={toggleAgentActive}
            {providerModels}
            {providers}
          />
        {:else if activeView === strings.nav.memories}
          <MemoriesView
            {actionStates}
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
            onToggleActive={toggleMemoryActive}
            onTogglePinned={toggleMemoryPinned}
          />
        {:else if activeView === strings.nav.tasks}
          <TasksView
            {actionStates}
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
            onToggleState={toggleTaskState}
            {providerModels}
            {providers}
            {submitting}
          />
        {:else if activeView === strings.nav.mcp}
          <MCPView
            {actionStates}
            {pendingToolApprovals}
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
        {:else}
          <section class="window-panel">
            <p>{strings.workspace.emptyScreen}</p>
          </section>
        {/if}
      </WorkspaceWindow>
    {/if}
  </AppShell>
{/if}
