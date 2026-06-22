package tasks

import "time"

const (
	TaskTypeAgent  = "agent"
	TaskTypeSystem = "system"

	TaskDraft    = "draft"
	TaskEnabled  = "enabled"
	TaskDisabled = "disabled"

	RunQueued    = "queued"
	RunClaimed   = "claimed"
	RunRunning   = "running"
	RunWaiting   = "waiting"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
	RunCancelled = "cancelled"
	RunTimedOut  = "timed_out"
)

type Task struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspace_id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	TaskType          string    `json:"task_type"`
	State             string    `json:"state"`
	SystemManaged     bool      `json:"system_managed"`
	AgentID           string    `json:"agent_id,omitempty"`
	ProviderID        string    `json:"provider_id,omitempty"`
	Model             string    `json:"model,omitempty"`
	Prompt            string    `json:"prompt"`
	ToolPolicy        string    `json:"tool_policy"`
	MaxRetries        int       `json:"max_retries"`
	TimeoutMS         int       `json:"timeout_ms"`
	ConcurrencyPolicy string    `json:"concurrency_policy"`
	Result            string    `json:"result,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Schedule struct {
	ID                     string     `json:"id"`
	TaskID                 string     `json:"task_id"`
	Mode                   string     `json:"mode"`
	CronExpression         string     `json:"cron_expression,omitempty"`
	IntervalSeconds        int        `json:"interval_seconds,omitempty"`
	RunAt                  *time.Time `json:"run_at,omitempty"`
	Timezone               string     `json:"timezone"`
	Enabled                bool       `json:"enabled"`
	NextRunAt              *time.Time `json:"next_run_at,omitempty"`
	LastEnqueuedOccurrence string     `json:"last_enqueued_occurrence,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type Run struct {
	ID             string     `json:"id"`
	TaskID         string     `json:"task_id"`
	ScheduleID     string     `json:"schedule_id,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
	State          string     `json:"state"`
	Attempt        int        `json:"attempt"`
	MaxRetries     int        `json:"max_retries"`
	TimeoutMS      int        `json:"timeout_ms"`
	LeaseOwner     string     `json:"lease_owner,omitempty"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty"`
	QueuedAt       time.Time  `json:"queued_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Result         string     `json:"result,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type TaskRecord struct {
	Task     Task     `json:"task"`
	Schedule Schedule `json:"schedule"`
}

type RunRecord struct {
	Run    Run     `json:"run"`
	Events []Event `json:"events"`
}

type Event struct {
	ID        string    `json:"id"`
	RunID     string    `json:"task_run_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type TaskInput struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	TaskType          string `json:"task_type"`
	State             string `json:"state"`
	AgentID           string `json:"agent_id,omitempty"`
	ProviderID        string `json:"provider_id,omitempty"`
	Model             string `json:"model,omitempty"`
	Prompt            string `json:"prompt"`
	ToolPolicy        string `json:"tool_policy"`
	MaxRetries        int    `json:"max_retries"`
	TimeoutMS         int    `json:"timeout_ms"`
	ConcurrencyPolicy string `json:"concurrency_policy"`
	ScheduleMode      string `json:"schedule_mode"`
	CronExpression    string `json:"cron_expression,omitempty"`
	IntervalSeconds   int    `json:"interval_seconds,omitempty"`
	RunAt             string `json:"run_at,omitempty"`
	Timezone          string `json:"timezone"`
}

type PrincipalContext struct {
	WorkspaceID string
	UserID      string
	IPAddress   string
	UserAgent   string
}
