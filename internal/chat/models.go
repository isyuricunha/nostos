package chat

import "time"

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
	ID               string     `json:"id"`
	WorkspaceID      string     `json:"workspace_id"`
	OwnerUserID      string     `json:"owner_user_id"`
	AgentID          string     `json:"agent_id,omitempty"`
	ProviderID       string     `json:"provider_id,omitempty"`
	Model            string     `json:"model,omitempty"`
	Title            string     `json:"title"`
	Summary          string     `json:"summary"`
	SummaryUpdatedAt *time.Time `json:"summary_updated_at,omitempty"`
	ArchivedAt       *time.Time `json:"archived_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Message struct {
	ID               string    `json:"id"`
	ConversationID   string    `json:"conversation_id"`
	BranchID         string    `json:"branch_id,omitempty"`
	ParentMessageID  string    `json:"parent_message_id,omitempty"`
	Role             string    `json:"role"`
	Content          string    `json:"content"`
	ProviderID       string    `json:"provider_id,omitempty"`
	Model            string    `json:"model,omitempty"`
	PromptTokens     int       `json:"prompt_tokens,omitempty"`
	CompletionTokens int       `json:"completion_tokens,omitempty"`
	TotalTokens      int       `json:"total_tokens,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
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
	ID                string
	Name              string
	SystemPrompt      string
	DefaultProviderID string
	DefaultModel      string
	FallbackModel     string
	MemoryAccessMode  string
}

type MemoryRequest struct {
	WorkspaceID    string
	UserID         string
	AgentID        string
	ConversationID string
	AccessMode     string
	Query          string
}

type MemorySnippet struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
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
