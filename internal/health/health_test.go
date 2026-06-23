package health

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
)

func TestReadyReportsRuntimeHeartbeatsAndCounts(t *testing.T) {
	ctx := context.Background()
	store, cleanup := openHealthTestStore(t, ctx)
	defer cleanup()

	if err := RecordHeartbeat(ctx, store, "worker", "worker-1", "healthy", ""); err != nil {
		t.Fatalf("record worker heartbeat: %v", err)
	}
	if err := RecordHeartbeat(ctx, store, "scheduler", "worker-1", "healthy", ""); err != nil {
		t.Fatalf("record scheduler heartbeat: %v", err)
	}

	service := NewService(store, "0.1.0", "abc123", "2026-06-22T00:00:00Z", filepath.Join("..", "..", "migrations"))
	status := service.Ready(ctx)
	if !status.Ready {
		t.Fatalf("expected ready status: %#v", status)
	}
	if status.Components["worker_heartbeat"] != "healthy" || status.Components["scheduler_heartbeat"] != "healthy" {
		t.Fatalf("expected healthy heartbeats: %#v", status.Components)
	}
	if status.Providers.Configured != 0 || status.Components["provider_health"] != "not_configured" {
		t.Fatalf("expected provider not_configured only when no providers exist: %#v", status)
	}
	if status.Migration.Total == 0 || !status.Migration.Current {
		t.Fatalf("expected current migration status: %#v", status.Migration)
	}
}

func TestReadyReportsStaleHeartbeat(t *testing.T) {
	ctx := context.Background()
	store, cleanup := openHealthTestStore(t, ctx)
	defer cleanup()

	if err := RecordHeartbeat(ctx, store, "worker", "worker-1", "healthy", ""); err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	old := time.Now().UTC().Add(-10 * time.Minute)
	if _, err := store.DB.ExecContext(ctx, `UPDATE runtime_statuses SET last_seen_at = ? WHERE component = ?`, store.NowArg(old), "worker"); err != nil {
		t.Fatalf("age heartbeat: %v", err)
	}

	service := NewService(store, "0.1.0", "abc123", "2026-06-22T00:00:00Z", filepath.Join("..", "..", "migrations"))
	status := service.Ready(ctx)
	if status.Components["worker_heartbeat"] != "stale" {
		t.Fatalf("expected stale worker heartbeat: %#v", status.ComponentDetails["worker"])
	}
	if !status.ComponentDetails["worker"].Stale {
		t.Fatalf("expected stale detail: %#v", status.ComponentDetails["worker"])
	}
}

func openHealthTestStore(t *testing.T, ctx context.Context) (*database.Store, func()) {
	t.Helper()
	store, err := database.Open(ctx, config.DatabaseConfig{Driver: "sqlite", URL: filepath.Join(t.TempDir(), "health.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.RunMigrations(ctx, store, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return store, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}
}
