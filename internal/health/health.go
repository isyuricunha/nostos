package health

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/isyuricunha/nostos/internal/database"
)

const staleHeartbeatAfter = 2 * time.Minute

type Service struct {
	store          *database.Store
	migrationsDir  string
	version        string
	buildCommit    string
	buildTimestamp string
	startedAt      time.Time
}

type Status struct {
	Ready            bool                        `json:"ready"`
	Version          string                      `json:"version"`
	BuildCommit      string                      `json:"build_commit"`
	BuildTimestamp   string                      `json:"build_timestamp"`
	StartedAt        string                      `json:"started_at"`
	Database         ComponentStatus             `json:"database"`
	Migration        database.MigrationStatus    `json:"migration"`
	Components       map[string]string           `json:"components"`
	ComponentDetails map[string]RuntimeComponent `json:"component_details"`
	QueueDepth       int                         `json:"queue_depth"`
	ActiveTaskRuns   int                         `json:"active_task_runs"`
	ActiveChatRuns   int                         `json:"active_chat_runs"`
	Providers        HealthCounts                `json:"providers"`
	MCP              HealthCounts                `json:"mcp"`
}

type ComponentStatus struct {
	OK      bool   `json:"ok"`
	Driver  string `json:"driver,omitempty"`
	Message string `json:"message,omitempty"`
}

type RuntimeComponent struct {
	Status        string `json:"status"`
	Message       string `json:"message,omitempty"`
	InstanceID    string `json:"instance_id,omitempty"`
	LastSeenAt    string `json:"last_seen_at,omitempty"`
	AgeSeconds    int64  `json:"age_seconds,omitempty"`
	Stale         bool   `json:"stale"`
	StaleAfterSec int64  `json:"stale_after_seconds,omitempty"`
}

type HealthCounts struct {
	Configured int `json:"configured"`
	Healthy    int `json:"healthy"`
	Unhealthy  int `json:"unhealthy"`
	Unknown    int `json:"unknown"`
}

func NewService(store *database.Store, version, buildCommit, buildTimestamp string, migrationsDir ...string) *Service {
	dir := ""
	if len(migrationsDir) > 0 {
		dir = migrationsDir[0]
	}
	return &Service{
		store:          store,
		migrationsDir:  dir,
		version:        version,
		buildCommit:    buildCommit,
		buildTimestamp: buildTimestamp,
		startedAt:      time.Now().UTC(),
	}
}

func (s *Service) Live() map[string]any {
	return map[string]any{
		"ok":              true,
		"version":         s.version,
		"build_commit":    s.buildCommit,
		"build_timestamp": s.buildTimestamp,
		"started_at":      s.startedAt.Format(time.RFC3339Nano),
	}
}

func (s *Service) Ready(ctx context.Context) Status {
	now := time.Now().UTC()
	status := Status{
		Ready:            true,
		Version:          s.version,
		BuildCommit:      s.buildCommit,
		BuildTimestamp:   s.buildTimestamp,
		StartedAt:        s.startedAt.Format(time.RFC3339Nano),
		Components:       map[string]string{"server": "healthy"},
		ComponentDetails: map[string]RuntimeComponent{},
	}
	if s.store == nil || s.store.DB == nil {
		status.Ready = false
		status.Database = ComponentStatus{OK: false, Message: "database is not configured"}
		status.Components["database"] = "not_configured"
		return status
	}
	if err := s.store.DB.PingContext(ctx); err != nil {
		status.Ready = false
		status.Database = ComponentStatus{OK: false, Driver: string(s.store.Dialect), Message: sanitizeMessage(err.Error())}
		status.Components["database"] = "unhealthy"
		return status
	}
	status.Database = ComponentStatus{OK: true, Driver: string(s.store.Dialect)}
	status.Components["database"] = "healthy"

	if s.migrationsDir != "" {
		migrationStatus, err := database.CheckMigrations(ctx, s.store, s.migrationsDir)
		if err != nil {
			status.Ready = false
			status.Components["migration"] = "unhealthy"
			status.Migration = database.MigrationStatus{Current: false}
		} else {
			status.Migration = migrationStatus
			if migrationStatus.Current {
				status.Components["migration"] = "healthy"
			} else {
				status.Ready = false
				status.Components["migration"] = "degraded"
			}
		}
	} else {
		status.Components["migration"] = "unknown"
	}

	status.ComponentDetails["worker"] = s.runtimeComponent(ctx, "worker", now)
	status.Components["worker_heartbeat"] = status.ComponentDetails["worker"].Status
	status.ComponentDetails["scheduler"] = s.runtimeComponent(ctx, "scheduler", now)
	status.Components["scheduler_heartbeat"] = status.ComponentDetails["scheduler"].Status

	status.QueueDepth = s.countRuns(ctx, "task_runs", []string{"queued"})
	status.ActiveTaskRuns = s.countRuns(ctx, "task_runs", []string{"claimed", "running", "waiting"})
	status.ActiveChatRuns = s.countRuns(ctx, "chat_runs", []string{"pending", "streaming", "waiting_for_tool_approval"})
	status.Providers = s.healthCounts(ctx, "providers")
	status.MCP = s.healthCounts(ctx, "mcp_servers")
	status.Components["provider_health"] = aggregateHealth(status.Providers)
	status.Components["mcp_health"] = aggregateHealth(status.MCP)
	return status
}

func RecordHeartbeat(ctx context.Context, store *database.Store, component string, instanceID string, status string, message string) error {
	if store == nil || store.DB == nil {
		return nil
	}
	now := time.Now().UTC()
	if status == "" {
		status = "healthy"
	}
	if store.Dialect == database.Postgres {
		query := `INSERT INTO runtime_statuses (component, status, instance_id, message, metadata, last_seen_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, '{}'::jsonb, $5, $6, $7)
ON CONFLICT (component) DO UPDATE SET status = EXCLUDED.status, instance_id = EXCLUDED.instance_id,
message = EXCLUDED.message, last_seen_at = EXCLUDED.last_seen_at, updated_at = EXCLUDED.updated_at`
		_, err := store.DB.ExecContext(ctx, query, component, status, nullableString(instanceID), nullableString(message), store.NowArg(now), store.NowArg(now), store.NowArg(now))
		return err
	}
	query := `INSERT INTO runtime_statuses (component, status, instance_id, message, metadata, last_seen_at, created_at, updated_at)
VALUES (?, ?, ?, ?, '{}', ?, ?, ?)
ON CONFLICT(component) DO UPDATE SET status = excluded.status, instance_id = excluded.instance_id,
message = excluded.message, last_seen_at = excluded.last_seen_at, updated_at = excluded.updated_at`
	_, err := store.DB.ExecContext(ctx, query, component, status, nullableString(instanceID), nullableString(message), store.NowArg(now), store.NowArg(now), store.NowArg(now))
	return err
}

func (s *Service) runtimeComponent(ctx context.Context, component string, now time.Time) RuntimeComponent {
	item := RuntimeComponent{Status: "unknown", StaleAfterSec: int64(staleHeartbeatAfter.Seconds())}
	query := `SELECT status, instance_id, message, last_seen_at FROM runtime_statuses WHERE component = ` + s.store.Placeholder(1)
	var status string
	var instanceID sql.NullString
	var message sql.NullString
	var lastSeenRaw any
	err := s.store.DB.QueryRowContext(ctx, query, component).Scan(&status, &instanceID, &message, &lastSeenRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return item
	}
	if err != nil {
		item.Status = "unknown"
		item.Message = sanitizeMessage(err.Error())
		return item
	}
	lastSeen, err := database.ParseTime(lastSeenRaw)
	if err != nil {
		item.Status = "unknown"
		item.Message = "invalid heartbeat timestamp"
		return item
	}
	age := now.Sub(lastSeen)
	item.Status = status
	item.InstanceID = instanceID.String
	item.Message = sanitizeMessage(message.String)
	item.LastSeenAt = lastSeen.Format(time.RFC3339Nano)
	item.AgeSeconds = int64(age.Seconds())
	item.Stale = age > staleHeartbeatAfter
	if item.Stale {
		item.Status = "stale"
	}
	return item
}

func (s *Service) countRuns(ctx context.Context, table string, states []string) int {
	if len(states) == 0 {
		return 0
	}
	placeholders := make([]string, 0, len(states))
	args := make([]any, 0, len(states))
	for index, state := range states {
		placeholders = append(placeholders, s.store.Placeholder(index+1))
		args = append(args, state)
	}
	query := `SELECT COUNT(*) FROM ` + table + ` WHERE state IN (` + join(placeholders) + `)`
	var count int
	if err := s.store.DB.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0
	}
	return count
}

func (s *Service) healthCounts(ctx context.Context, table string) HealthCounts {
	query := `SELECT health_status, COUNT(*) FROM ` + table + ` WHERE enabled = ` + s.store.Placeholder(1) + ` GROUP BY health_status`
	rows, err := s.store.DB.QueryContext(ctx, query, true)
	if err != nil {
		return HealthCounts{}
	}
	defer rows.Close()
	var counts HealthCounts
	for rows.Next() {
		var healthStatus string
		var count int
		if err := rows.Scan(&healthStatus, &count); err != nil {
			return counts
		}
		counts.Configured += count
		switch healthStatus {
		case "healthy":
			counts.Healthy += count
		case "unhealthy":
			counts.Unhealthy += count
		default:
			counts.Unknown += count
		}
	}
	return counts
}

func aggregateHealth(counts HealthCounts) string {
	if counts.Configured == 0 {
		return "not_configured"
	}
	if counts.Unhealthy > 0 {
		return "degraded"
	}
	if counts.Unknown > 0 {
		return "unknown"
	}
	return "healthy"
}

func join(values []string) string {
	out := ""
	for index, value := range values {
		if index > 0 {
			out += ", "
		}
		out += value
	}
	return out
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func sanitizeMessage(value string) string {
	if len(value) > 300 {
		return value[:300]
	}
	return value
}
