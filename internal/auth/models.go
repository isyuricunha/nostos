package auth

import "time"

const (
	AuditLoginSuccess       = "login_success"
	AuditLoginFailure       = "login_failure"
	AuditLogout             = "logout"
	AuditSessionRevoked     = "session_revoked"
	AuditSettingsChanged    = "settings_changed"
	AuditProviderCreated    = "provider_created"
	AuditProviderUpdated    = "provider_updated"
	AuditProviderDeleted    = "provider_deleted"
	DefaultWorkspaceName    = "Personal Workspace"
	SessionCookieName       = "nostos_session"
	CSRFCookieName          = "nostos_csrf"
	CSRFHeaderName          = "X-CSRF-Token"
	MinimumPasswordLength   = 12
	maxFailedLoginAttempts  = 5
	loginThrottleWindow     = 15 * time.Minute
	loginThrottleLockout    = 15 * time.Minute
	defaultSessionTokenSize = 32
)

type User struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Email       string     `json:"email"`
	DisplayName string     `json:"display_name"`
	Role        string     `json:"role"`
	DisabledAt  *time.Time `json:"disabled_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Session struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	TokenHash     string     `json:"-"`
	CSRFTokenHash string     `json:"-"`
	IPAddress     string     `json:"ip_address,omitempty"`
	UserAgent     string     `json:"user_agent,omitempty"`
	ExpiresAt     time.Time  `json:"expires_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Principal struct {
	User    User
	Session Session
}

type SetupInput struct {
	Email           string
	DisplayName     string
	Password        string
	ConfirmPassword string
	IPAddress       string
	UserAgent       string
}

type LoginInput struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent string
}

type AuthTokens struct {
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
}

type AuthResult struct {
	User   User
	Tokens AuthTokens
}

type AuditEvent struct {
	ID          string
	WorkspaceID string
	ActorUserID string
	EventType   string
	IPAddress   string
	UserAgent   string
	Metadata    map[string]any
	CreatedAt   time.Time
}
