package providers

import "time"

type Provider struct {
	ID                        string            `json:"id"`
	WorkspaceID               string            `json:"workspace_id"`
	Name                      string            `json:"name"`
	BaseURL                   string            `json:"base_url"`
	APIKeyEnvRef              string            `json:"api_key_env_ref,omitempty"`
	Organization              string            `json:"organization_header,omitempty"`
	Project                   string            `json:"project_header,omitempty"`
	CustomHeaders             map[string]string `json:"custom_headers"`
	Enabled                   bool              `json:"enabled"`
	RequestTimeoutMS          int               `json:"request_timeout_ms"`
	DefaultModel              string            `json:"default_model,omitempty"`
	FallbackModel             string            `json:"fallback_model,omitempty"`
	HealthStatus              string            `json:"health_status"`
	LastHealthCheckAt         *time.Time        `json:"last_health_check_at,omitempty"`
	HealthLatencyMS           int               `json:"health_latency_ms,omitempty"`
	LastError                 string            `json:"last_error,omitempty"`
	ModelRefreshState         string            `json:"model_refresh_state"`
	ModelRefreshStartedAt     *time.Time        `json:"model_refresh_started_at,omitempty"`
	ModelRefreshCompletedAt   *time.Time        `json:"model_refresh_completed_at,omitempty"`
	ModelRefreshDurationMS    int               `json:"model_refresh_duration_ms,omitempty"`
	ModelRefreshErrorCategory string            `json:"model_refresh_error_category,omitempty"`
	ModelRefreshErrorMessage  string            `json:"model_refresh_error_message,omitempty"`
	ModelCount                int               `json:"model_count,omitempty"`
	AvailableModelCount       int               `json:"available_model_count,omitempty"`
	UnavailableModelCount     int               `json:"unavailable_model_count,omitempty"`
	CreatedAt                 time.Time         `json:"created_at"`
	UpdatedAt                 time.Time         `json:"updated_at"`
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
	ID                    string         `json:"id"`
	WorkspaceID           string         `json:"workspace_id"`
	ProviderID            string         `json:"provider_id"`
	ProviderName          string         `json:"provider_name,omitempty"`
	ModelID               string         `json:"model_id"`
	DisplayName           string         `json:"display_name,omitempty"`
	Source                string         `json:"source"`
	Active                bool           `json:"active"`
	Enabled               bool           `json:"enabled"`
	ManuallyAdded         bool           `json:"manually_added"`
	Available             bool           `json:"available"`
	RefreshedAt           time.Time      `json:"refreshed_at,omitempty"`
	FirstSeenAt           *time.Time     `json:"first_seen_at,omitempty"`
	LastSeenAt            *time.Time     `json:"last_seen_at,omitempty"`
	LastSuccessfulProbeAt *time.Time     `json:"last_successful_probe_at,omitempty"`
	LastFailedProbeAt     *time.Time     `json:"last_failed_probe_at,omitempty"`
	LastErrorCategory     string         `json:"last_error_category,omitempty"`
	LastSafeErrorMessage  string         `json:"last_safe_error_message,omitempty"`
	Capabilities          []string       `json:"capabilities"`
	CapabilitySource      string         `json:"capability_source"`
	Metadata              map[string]any `json:"metadata,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

type ModelQuery struct {
	WorkspaceID        string
	ProviderID         string
	Search             string
	Limit              int
	Offset             int
	Role               string
	IncludeUnavailable bool
}

type ModelInput struct {
	ProviderID       string         `json:"provider_id"`
	ModelID          string         `json:"model_id"`
	DisplayName      string         `json:"display_name,omitempty"`
	Enabled          bool           `json:"enabled"`
	Available        bool           `json:"available"`
	Capabilities     []string       `json:"capabilities"`
	CapabilitySource string         `json:"capability_source,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type ModelPatch struct {
	DisplayName          *string  `json:"display_name,omitempty"`
	Enabled              *bool    `json:"enabled,omitempty"`
	Available            *bool    `json:"available,omitempty"`
	Capabilities         []string `json:"capabilities,omitempty"`
	CapabilitySource     string   `json:"capability_source,omitempty"`
	LastErrorCategory    *string  `json:"last_error_category,omitempty"`
	LastSafeErrorMessage *string  `json:"last_safe_error_message,omitempty"`
}

type ModelRefreshStatus struct {
	ProviderID            string     `json:"provider_id"`
	State                 string     `json:"state"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	DurationMS            int        `json:"duration_ms,omitempty"`
	ErrorCategory         string     `json:"error_category,omitempty"`
	ErrorMessage          string     `json:"error_message,omitempty"`
	CachedModelCount      int        `json:"cached_model_count"`
	AvailableModelCount   int        `json:"available_model_count"`
	UnavailableModelCount int        `json:"unavailable_model_count"`
}

type ModelRoleBinding struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Role         string    `json:"role"`
	Position     int       `json:"position"`
	ProviderID   string    `json:"provider_id"`
	ProviderName string    `json:"provider_name,omitempty"`
	ModelID      string    `json:"model_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ModelRoleInput struct {
	Models []ModelRoleReference `json:"models"`
}

type ModelRoleReference struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
}

type RoleResolution struct {
	Provider Provider
	APIKey   string
	ModelID  string
	Role     string
	Reason   string
}

const (
	ModelRoleChat    = "chat"
	ModelRoleUtility = "utility"
	ModelRoleVision  = "vision"
)

type PrincipalContext struct {
	WorkspaceID string
	UserID      string
	IPAddress   string
	UserAgent   string
}
