package mcp

import "time"

type Server struct {
	ID               string     `json:"id"`
	WorkspaceID      string     `json:"workspace_id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	TransportType    string     `json:"transport_type"`
	Command          string     `json:"command,omitempty"`
	Arguments        []string   `json:"arguments"`
	WorkingDirectory string     `json:"working_directory,omitempty"`
	EnvironmentKeys  []string   `json:"environment_keys,omitempty"`
	HTTPURL          string     `json:"http_url,omitempty"`
	HTTPHeaderKeys   []string   `json:"http_header_keys,omitempty"`
	Enabled          bool       `json:"enabled"`
	StartupTimeoutMS int        `json:"startup_timeout_ms"`
	RequestTimeoutMS int        `json:"request_timeout_ms"`
	HealthStatus     string     `json:"health_status"`
	LastError        string     `json:"last_error,omitempty"`
	LastConnectedAt  *time.Time `json:"last_connected_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ServerInput struct {
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	TransportType    string            `json:"transport_type"`
	Command          string            `json:"command,omitempty"`
	Arguments        []string          `json:"arguments"`
	WorkingDirectory string            `json:"working_directory,omitempty"`
	Environment      map[string]string `json:"environment"`
	HTTPURL          string            `json:"http_url,omitempty"`
	HTTPHeaders      map[string]string `json:"http_headers"`
	Enabled          bool              `json:"enabled"`
	StartupTimeoutMS int               `json:"startup_timeout_ms"`
	RequestTimeoutMS int               `json:"request_timeout_ms"`
}

type ServerSecret struct {
	Environment map[string]string
	HTTPHeaders map[string]string
}

type Tool struct {
	ID             string    `json:"id"`
	ServerID       string    `json:"server_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	InputSchema    string    `json:"input_schema"`
	PermissionMode string    `json:"permission_mode"`
	DiscoveredAt   time.Time `json:"discovered_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type DiscoveredTool struct {
	Name        string
	Description string
	InputSchema any
}

type PrincipalContext struct {
	WorkspaceID string
	UserID      string
	IPAddress   string
	UserAgent   string
}
