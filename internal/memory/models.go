package memory

import "time"

type Memory struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	OwnerUserID     string     `json:"owner_user_id"`
	AgentID         string     `json:"agent_id,omitempty"`
	ConversationID  string     `json:"conversation_id,omitempty"`
	Title           string     `json:"title"`
	Content         string     `json:"content"`
	Tags            []string   `json:"tags"`
	Scope           string     `json:"scope"`
	Importance      int        `json:"importance"`
	Pinned          bool       `json:"pinned"`
	Active          bool       `json:"active"`
	Source          string     `json:"source"`
	SourceMessageID string     `json:"source_message_id,omitempty"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	UseCount        int        `json:"use_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type MemoryInput struct {
	AgentID         string   `json:"agent_id,omitempty"`
	ConversationID  string   `json:"conversation_id,omitempty"`
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Tags            []string `json:"tags"`
	Scope           string   `json:"scope"`
	Importance      int      `json:"importance"`
	Pinned          bool     `json:"pinned"`
	Active          bool     `json:"active"`
	Source          string   `json:"source"`
	SourceMessageID string   `json:"source_message_id,omitempty"`
}

type PrincipalContext struct {
	WorkspaceID string
	UserID      string
}
