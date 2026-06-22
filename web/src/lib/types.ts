export type User = {
  id: string;
  email: string;
  display_name: string;
  role: string;
  workspace_id: string;
};

export type Session = {
  id: string;
  ip_address?: string;
  user_agent?: string;
  expires_at: string;
  created_at: string;
};

export type Provider = {
  id: string;
  name: string;
  base_url: string;
  enabled: boolean;
  request_timeout_ms: number;
  organization_header?: string;
  project_header?: string;
  custom_headers: Record<string, string>;
  default_model?: string;
  fallback_model?: string;
  health_status: string;
  last_health_check_at?: string;
  health_latency_ms?: number;
  last_error?: string;
  api_key_env_ref?: string;
};

export type ProviderModel = {
  id: string;
  provider_id: string;
  model_id: string;
};

export type Conversation = {
  id: string;
  title: string;
  agent_id?: string;
  provider_id?: string;
  model?: string;
  summary?: string;
  summary_status?: string;
  summary_error?: string;
  summary_updated_at?: string;
  archived_at?: string;
  updated_at: string;
};

export type Message = {
  id: string;
  role: 'system' | 'user' | 'assistant' | 'tool';
  content: string;
  provider_id?: string;
  model?: string;
  total_tokens?: number;
  created_at: string;
};

export type Agent = {
  id: string;
  name: string;
  description: string;
  avatar: string;
  system_prompt: string;
  default_provider_id?: string;
  default_model?: string;
  fallback_model?: string;
  temperature: number;
  memory_access_mode: string;
  max_tool_iterations: number;
  tool_permission_default: string;
  active: boolean;
};

export type Memory = {
  id: string;
  title: string;
  content: string;
  tags: string[];
  scope: string;
  importance: number;
  pinned: boolean;
  active: boolean;
  source: string;
  use_count: number;
};

export type MemorySnippet = {
  id: string;
  title: string;
  content: string;
  score: number;
};

export type ToolCard = {
  id: string;
  name: string;
  state: string;
  result?: string;
};

export type ToolCall = {
  id: string;
  chat_run_id: string;
  provider_tool_call_id?: string;
  provider_name?: string;
  name: string;
  input: string;
  output?: string;
  output_truncated?: boolean;
  state: string;
  approval_state: string;
  error_message?: string;
  created_at: string;
};

export type MCPServer = {
  id: string;
  name: string;
  transport_type: string;
  command?: string;
  http_url?: string;
  enabled: boolean;
  health_status: string;
  last_error?: string;
};

export type MCPTool = {
  id: string;
  server_id: string;
  name: string;
  description: string;
  permission_mode: string;
};

export type Task = {
  id: string;
  name: string;
  description: string;
  task_type: string;
  state: string;
  system_managed: boolean;
  provider_id?: string;
  model?: string;
  prompt: string;
  tool_policy: string;
  max_retries: number;
  timeout_ms: number;
  concurrency_policy: string;
};

export type TaskSchedule = {
  id: string;
  task_id: string;
  mode: string;
  cron_expression?: string;
  interval_seconds?: number;
  run_at?: string;
  timezone: string;
  enabled: boolean;
  next_run_at?: string;
};

export type TaskRecord = {
  task: Task;
  schedule: TaskSchedule;
};

export type TaskRun = {
  id: string;
  task_id: string;
  state: string;
  attempt: number;
  max_retries: number;
  queued_at: string;
  started_at?: string;
  completed_at?: string;
  result?: string;
  error_message?: string;
};

export type TaskRunEvent = {
  id: string;
  task_run_id: string;
  level: string;
  message: string;
  created_at: string;
};

export type MessageFeedback = {
  id: string;
  message_id: string;
  rating: 'positive' | 'negative';
  reason?: string;
  comment?: string;
};

export type FeedbackStats = {
  positive: number;
  negative: number;
};

export type ReplyPreset = {
  id: string;
  name: string;
  description: string;
  prompt_instruction: string;
  icon: string;
  sort_order: number;
  active: boolean;
  system_default: boolean;
};

export type ReplyDraft = {
  id: string;
  source_message_id: string;
  preset_id?: string;
  preset_name: string;
  custom_instruction: string;
  generated_draft: string;
  provider_id?: string;
  model?: string;
  created_at: string;
};

export type ReadyStatus = {
  ready: boolean;
  version: string;
  database: {
    ok: boolean;
    driver?: string;
    message?: string;
  };
  components: Record<string, string>;
};

export type SetupStatus = {
  available: boolean;
};

export type UserResponse = {
  user: User;
};

export type SessionsResponse = {
  sessions: Session[];
};

export type ProvidersResponse = {
  providers: Provider[];
};

export type ProviderResponse = {
  provider: Provider;
};

export type ModelsResponse = {
  models: ProviderModel[];
};

export type ConversationsResponse = {
  conversations: Conversation[];
};

export type ConversationResponse = {
  conversation: Conversation;
};

export type MessagesResponse = {
  messages: Message[];
};

export type AgentsResponse = {
  agents: Agent[];
};

export type AgentResponse = {
  agent: Agent;
};

export type MemoriesResponse = {
  memories: Memory[];
};

export type MemoryResponse = {
  memory: Memory;
};

export type MCPServersResponse = {
  servers: MCPServer[];
};

export type MCPServerResponse = {
  server: MCPServer;
};

export type MCPToolsResponse = {
  tools: MCPTool[];
};

export type ToolApprovalsResponse = {
  tool_calls: ToolCall[];
};

export type ToolCallResponse = {
  tool_call: ToolCall;
};

export type TasksResponse = {
  tasks: TaskRecord[];
};

export type TaskResponse = {
  task: Task;
  schedule: TaskSchedule;
};

export type TaskRunsResponse = {
  runs: TaskRun[];
};

export type TaskRunResponse = {
  run: TaskRun;
};

export type TaskRunRecordResponse = {
  run: TaskRun;
  events: TaskRunEvent[];
};

export type FeedbackResponse = {
  feedback: MessageFeedback;
};

export type FeedbackListResponse = {
  feedback: MessageFeedback[];
};

export type FeedbackStatsResponse = {
  stats: FeedbackStats;
};

export type ReplyPresetsResponse = {
  presets: ReplyPreset[];
};

export type ReplyPresetResponse = {
  preset: ReplyPreset;
};

export type ReplyDraftResponse = {
  draft: ReplyDraft;
};
