package chat

import (
	"time"

	"github.com/isyuricunha/nostos/internal/providers"
)

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"

	RunPending             = "pending"
	RunStreaming           = "streaming"
	RunWaitingForApproval  = "waiting_for_tool_approval"
	RunCompleted           = "completed"
	RunFailed              = "failed"
	RunCancelled           = "cancelled"
	DefaultConversationTTL = 30
)

type Conversation struct {
	ID                          string     `json:"id"`
	WorkspaceID                 string     `json:"workspace_id"`
	OwnerUserID                 string     `json:"owner_user_id"`
	AgentID                     string     `json:"agent_id,omitempty"`
	ProviderID                  string     `json:"provider_id,omitempty"`
	Model                       string     `json:"model,omitempty"`
	Title                       string     `json:"title"`
	Summary                     string     `json:"summary"`
	SummaryUpdatedAt            *time.Time `json:"summary_updated_at,omitempty"`
	SummaryStatus               string     `json:"summary_status"`
	SummaryError                string     `json:"summary_error,omitempty"`
	SummarySourceStartMessageID string     `json:"summary_source_start_message_id,omitempty"`
	SummarySourceEndMessageID   string     `json:"summary_source_end_message_id,omitempty"`
	SummaryProviderID           string     `json:"summary_provider_id,omitempty"`
	SummaryModel                string     `json:"summary_model,omitempty"`
	SummaryGeneratedAt          *time.Time `json:"summary_generated_at,omitempty"`
	SummaryEstimatedInputTokens int        `json:"summary_estimated_input_tokens,omitempty"`
	SummaryVersion              int        `json:"summary_version"`
	ArchivedAt                  *time.Time `json:"archived_at,omitempty"`
	DeletedAt                   *time.Time `json:"deleted_at,omitempty"`
	CreatedAt                   time.Time  `json:"created_at"`
	UpdatedAt                   time.Time  `json:"updated_at"`
}

type Message struct {
	ID               string               `json:"id"`
	ConversationID   string               `json:"conversation_id"`
	BranchID         string               `json:"branch_id,omitempty"`
	ParentMessageID  string               `json:"parent_message_id,omitempty"`
	Role             string               `json:"role"`
	Content          string               `json:"content"`
	ToolCallID       string               `json:"tool_call_id,omitempty"`
	ToolCalls        []providers.ToolCall `json:"tool_calls,omitempty"`
	ProviderID       string               `json:"provider_id,omitempty"`
	Model            string               `json:"model,omitempty"`
	PromptTokens     int                  `json:"prompt_tokens,omitempty"`
	CompletionTokens int                  `json:"completion_tokens,omitempty"`
	TotalTokens      int                  `json:"total_tokens,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

type ChatRun struct {
	ID                    string     `json:"id"`
	ConversationID        string     `json:"conversation_id"`
	UserMessageID         string     `json:"user_message_id,omitempty"`
	AssistantMessageID    string     `json:"assistant_message_id,omitempty"`
	BranchID              string     `json:"branch_id,omitempty"`
	ProviderID            string     `json:"provider_id,omitempty"`
	Model                 string     `json:"model,omitempty"`
	State                 string     `json:"state"`
	ErrorCode             string     `json:"error_code,omitempty"`
	ErrorMessage          string     `json:"error_message,omitempty"`
	CancellationRequested *time.Time `json:"cancellation_requested_at,omitempty"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	PromptTokens          int        `json:"prompt_tokens,omitempty"`
	CompletionTokens      int        `json:"completion_tokens,omitempty"`
	TotalTokens           int        `json:"total_tokens,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type Branch struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversation_id"`
	ParentMessageID string    `json:"parent_message_id,omitempty"`
	SourceMessageID string    `json:"source_message_id,omitempty"`
	Name            string    `json:"name"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"created_at"`
}

type PrincipalContext struct {
	WorkspaceID string
	UserID      string
}

type AgentContext struct {
	ID                    string
	Name                  string
	Description           string
	SystemPrompt          string
	DefaultProviderID     string
	DefaultModel          string
	FallbackModel         string
	Temperature           float64
	MaxToolIterations     int
	MemoryAccessMode      string
	ToolPermissionDefault string
	Active                bool
}

type MemoryRequest struct {
	WorkspaceID    string
	UserID         string
	AgentID        string
	ConversationID string
	AccessMode     string
	Query          string
}

const (
	ToolCallPending            = "pending"
	ToolCallWaitingForApproval = "waiting_for_approval"
	ToolCallApproved           = "approved"
	ToolCallRunning            = "running"
	ToolCallSucceeded          = "succeeded"
	ToolCallDenied             = "denied"
	ToolCallFailed             = "failed"
	ToolCallCancelled          = "cancelled"
	ToolCallTimedOut           = "timed_out"

	ToolApprovalNotRequired = "not_required"
	ToolApprovalPending     = "pending"
	ToolApprovalApproved    = "approved"
	ToolApprovalDenied      = "denied"

	ToolPermissionDeny  = "deny"
	ToolPermissionAsk   = "ask"
	ToolPermissionAllow = "allow"

	ToolDecisionApproveOnce         = "approve_once"
	ToolDecisionApproveConversation = "approve_conversation"
	ToolDecisionAllowAgent          = "allow_agent"
	ToolDecisionDeny                = "deny"
	ToolDecisionDenyDisableTool     = "deny_disable_tool"
)

type RuntimeTool struct {
	ID             string
	ServerID       string
	Name           string
	ProviderName   string
	Description    string
	InputSchema    string
	PermissionMode string
}

type ToolExposureRequest struct {
	WorkspaceID            string
	AgentID                string
	ConversationID         string
	AgentDefaultPermission string
}

type ToolExecutionRequest struct {
	WorkspaceID     string
	ToolID          string
	Arguments       string
	Timeout         time.Duration
	MaxResultBytes  int
	ProviderName    string
	ToolDisplayName string
}

type ToolExecutionResult struct {
	Content   string
	Truncated bool
}

type ToolCallRecord struct {
	ID                 string     `json:"id"`
	ChatRunID          string     `json:"chat_run_id"`
	MessageID          string     `json:"message_id,omitempty"`
	ToolID             string     `json:"tool_id,omitempty"`
	ProviderToolCallID string     `json:"provider_tool_call_id,omitempty"`
	ProviderName       string     `json:"provider_name,omitempty"`
	Name               string     `json:"name"`
	Input              string     `json:"input"`
	Output             string     `json:"output,omitempty"`
	OutputTruncated    bool       `json:"output_truncated"`
	State              string     `json:"state"`
	ApprovalState      string     `json:"approval_state"`
	ErrorCode          string     `json:"error_code,omitempty"`
	ErrorMessage       string     `json:"error_message,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type ToolApprovalRecord struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	ToolCallID     string    `json:"tool_call_id,omitempty"`
	ToolID         string    `json:"tool_id,omitempty"`
	AgentID        string    `json:"agent_id,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	ActorUserID    string    `json:"actor_user_id,omitempty"`
	Decision       string    `json:"decision"`
	CreatedAt      time.Time `json:"created_at"`
}

type MemorySnippet struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

type ToolResult struct {
	Text      string
	Truncated bool
}

type RunInput struct {
	Content                 string `json:"content"`
	ProviderID              string `json:"provider_id,omitempty"`
	Model                   string `json:"model,omitempty"`
	RegenerationInstruction string `json:"regeneration_instruction,omitempty"`
}

type UpdateConversationInput struct {
	Title   string  `json:"title,omitempty"`
	Archive *bool   `json:"archive,omitempty"`
	Summary *string `json:"summary,omitempty"`
}

type SummaryUpdate struct {
	Summary              string
	Status               string
	Error                string
	SourceStartMessageID string
	SourceEndMessageID   string
	ProviderID           string
	Model                string
	GeneratedAt          *time.Time
	EstimatedInputTokens int
	IncrementVersion     bool
}
