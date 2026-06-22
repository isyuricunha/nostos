package agents

import "time"

const DefaultAgentName = "General Assistant"

type Agent struct {
	ID                    string    `json:"id"`
	WorkspaceID           string    `json:"workspace_id"`
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	Avatar                string    `json:"avatar"`
	SystemPrompt          string    `json:"system_prompt"`
	DefaultProviderID     string    `json:"default_provider_id,omitempty"`
	DefaultModel          string    `json:"default_model,omitempty"`
	FallbackModel         string    `json:"fallback_model,omitempty"`
	Temperature           float64   `json:"temperature"`
	MaxToolIterations     int       `json:"max_tool_iterations"`
	MemoryAccessMode      string    `json:"memory_access_mode"`
	ToolPermissionDefault string    `json:"tool_permission_default"`
	Active                bool      `json:"active"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type AgentInput struct {
	Name                  string  `json:"name"`
	Description           string  `json:"description"`
	Avatar                string  `json:"avatar"`
	SystemPrompt          string  `json:"system_prompt"`
	DefaultProviderID     string  `json:"default_provider_id,omitempty"`
	DefaultModel          string  `json:"default_model,omitempty"`
	FallbackModel         string  `json:"fallback_model,omitempty"`
	Temperature           float64 `json:"temperature"`
	MaxToolIterations     int     `json:"max_tool_iterations"`
	MemoryAccessMode      string  `json:"memory_access_mode"`
	ToolPermissionDefault string  `json:"tool_permission_default"`
	Active                bool    `json:"active"`
}

type PrincipalContext struct {
	WorkspaceID string
	UserID      string
}

type ChatAgent struct {
	ID                string
	Name              string
	SystemPrompt      string
	DefaultProviderID string
	DefaultModel      string
	FallbackModel     string
	MemoryAccessMode  string
}
