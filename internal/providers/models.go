package providers

import "time"

type Provider struct {
	ID                string            `json:"id"`
	WorkspaceID       string            `json:"workspace_id"`
	Name              string            `json:"name"`
	BaseURL           string            `json:"base_url"`
	APIKeyEnvRef      string            `json:"api_key_env_ref,omitempty"`
	Organization      string            `json:"organization_header,omitempty"`
	Project           string            `json:"project_header,omitempty"`
	CustomHeaders     map[string]string `json:"custom_headers"`
	Enabled           bool              `json:"enabled"`
	RequestTimeoutMS  int               `json:"request_timeout_ms"`
	DefaultModel      string            `json:"default_model,omitempty"`
	FallbackModel     string            `json:"fallback_model,omitempty"`
	HealthStatus      string            `json:"health_status"`
	LastHealthCheckAt *time.Time        `json:"last_health_check_at,omitempty"`
	HealthLatencyMS   int               `json:"health_latency_ms,omitempty"`
	LastError         string            `json:"last_error,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

type ProviderSecret struct {
	EncryptedAPIKey string
	APIKeyEnvRef    string
}

type ProviderInput struct {
	Name             string            `json:"name"`
	BaseURL          string            `json:"base_url"`
	APIKey           *string           `json:"api_key,omitempty"`
	APIKeyEnvRef     string            `json:"api_key_env_ref,omitempty"`
	Organization     string            `json:"organization_header,omitempty"`
	Project          string            `json:"project_header,omitempty"`
	CustomHeaders    map[string]string `json:"custom_headers"`
	Enabled          bool              `json:"enabled"`
	RequestTimeoutMS int               `json:"request_timeout_ms"`
	DefaultModel     string            `json:"default_model,omitempty"`
	FallbackModel    string            `json:"fallback_model,omitempty"`
}

type Model struct {
	ID          string    `json:"id"`
	ProviderID  string    `json:"provider_id"`
	ModelID     string    `json:"model_id"`
	DisplayName string    `json:"display_name,omitempty"`
	Source      string    `json:"source"`
	Active      bool      `json:"active"`
	RefreshedAt time.Time `json:"refreshed_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PrincipalContext struct {
	WorkspaceID string
	UserID      string
	IPAddress   string
	UserAgent   string
}
