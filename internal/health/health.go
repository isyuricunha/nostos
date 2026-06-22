package health

import (
	"context"
	"time"

	"github.com/yuricunha/nostos/internal/database"
)

type Service struct {
	store          *database.Store
	version        string
	buildCommit    string
	buildTimestamp string
	startedAt      time.Time
}

type Status struct {
	Ready          bool              `json:"ready"`
	Version        string            `json:"version"`
	BuildCommit    string            `json:"build_commit"`
	BuildTimestamp string            `json:"build_timestamp"`
	StartedAt      string            `json:"started_at"`
	Database       ComponentStatus   `json:"database"`
	Components     map[string]string `json:"components"`
}

type ComponentStatus struct {
	OK      bool   `json:"ok"`
	Driver  string `json:"driver,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewService(store *database.Store, version, buildCommit, buildTimestamp string) *Service {
	return &Service{
		store:          store,
		version:        version,
		buildCommit:    buildCommit,
		buildTimestamp: buildTimestamp,
		startedAt:      time.Now().UTC(),
	}
}

func (s *Service) Live() map[string]any {
	return map[string]any{
		"ok":         true,
		"version":    s.version,
		"started_at": s.startedAt.Format(time.RFC3339Nano),
	}
}

func (s *Service) Ready(ctx context.Context) Status {
	status := Status{
		Ready:          true,
		Version:        s.version,
		BuildCommit:    s.buildCommit,
		BuildTimestamp: s.buildTimestamp,
		StartedAt:      s.startedAt.Format(time.RFC3339Nano),
		Components: map[string]string{
			"worker_heartbeat":    "not_configured",
			"scheduler_heartbeat": "not_configured",
			"provider_health":     "not_configured",
			"mcp_health":          "not_configured",
		},
	}
	if s.store == nil || s.store.DB == nil {
		status.Ready = false
		status.Database = ComponentStatus{OK: false, Message: "database is not configured"}
		return status
	}
	if err := s.store.DB.PingContext(ctx); err != nil {
		status.Ready = false
		status.Database = ComponentStatus{OK: false, Driver: string(s.store.Dialect), Message: err.Error()}
		return status
	}
	status.Database = ComponentStatus{OK: true, Driver: string(s.store.Dialect)}
	return status
}
