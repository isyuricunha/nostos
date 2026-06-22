package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/isyuricunha/nostos/internal/auth"
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
	"github.com/isyuricunha/nostos/internal/providers"
)

func TestAgentTaskRunExecutesThroughProvider(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Task"}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":" complete"}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: [DONE]`)
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "mock-model"}}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg, store, user, cleanup := newTaskTestContext(t)
	defer cleanup()
	authRepo := auth.NewSQLRepository(store)
	client := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, client)
	apiKey := "test-api-key"
	provider, err := providerService.Create(ctx, providers.PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, providers.ProviderInput{
		Name:             "Mock",
		BaseURL:          server.URL,
		APIKey:           &apiKey,
		Enabled:          true,
		RequestTimeoutMS: 5000,
		DefaultModel:     "mock-model",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	service := NewService(cfg, NewSQLRepository(store), authRepo).WithProviderClient(providerService, client)
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	task, _, err := service.CreateTask(ctx, principal, TaskInput{
		Name:              "Daily summary",
		TaskType:          TaskTypeAgent,
		State:             TaskEnabled,
		ProviderID:        provider.ID,
		Model:             "mock-model",
		Prompt:            "Summarize the day.",
		ScheduleMode:      "manual",
		ToolPolicy:        "use_preapproved_tools_only",
		ConcurrencyPolicy: "skip",
		MaxRetries:        1,
		TimeoutMS:         30000,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if _, err := service.RunNow(ctx, principal, task.ID); err != nil {
		t.Fatalf("run task: %v", err)
	}
	if err := service.ClaimAndExecute(ctx, "worker-test"); err != nil {
		t.Fatalf("claim and execute: %v", err)
	}
	runs, err := service.ListRuns(ctx, principal, task.ID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 || runs[0].State != RunSucceeded || runs[0].Result != "Task complete" {
		t.Fatalf("unexpected run result: %#v", runs)
	}
	record, err := service.GetRunRecord(ctx, principal, runs[0].ID)
	if err != nil {
		t.Fatalf("get run record: %v", err)
	}
	if len(record.Events) != 2 {
		t.Fatalf("expected start and success events, got %#v", record.Events)
	}
}

func TestScheduleEnqueueAndLeaseRecovery(t *testing.T) {
	ctx := context.Background()
	cfg, store, user, cleanup := newTaskTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store))
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	task, _, err := service.CreateTask(ctx, principal, TaskInput{
		Name:              "Maintenance",
		TaskType:          TaskTypeSystem,
		State:             TaskEnabled,
		Prompt:            "cleanup_expired_sessions",
		ScheduleMode:      "interval",
		IntervalSeconds:   3600,
		ToolPolicy:        "use_preapproved_tools_only",
		ConcurrencyPolicy: "skip",
		MaxRetries:        1,
		TimeoutMS:         30000,
	})
	if err != nil {
		t.Fatalf("create scheduled task: %v", err)
	}
	past := time.Now().UTC().Add(-time.Minute)
	query := "UPDATE task_schedules SET next_run_at = " + store.Placeholder(1) + " WHERE task_id = " + store.Placeholder(2)
	if _, err := store.DB.ExecContext(ctx, query, store.NowArg(past), task.ID); err != nil {
		t.Fatalf("force due schedule: %v", err)
	}
	if err := service.EnqueueDueSchedules(ctx); err != nil {
		t.Fatalf("enqueue due schedules: %v", err)
	}
	if err := service.EnqueueDueSchedules(ctx); err != nil {
		t.Fatalf("enqueue due schedules again: %v", err)
	}
	runs, err := service.ListRuns(ctx, principal, task.ID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 || runs[0].State != RunQueued {
		t.Fatalf("expected one queued run, got %#v", runs)
	}
	repo := NewSQLRepository(store)
	claimed, err := repo.ClaimRun(ctx, "worker-test", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("claim run: %v", err)
	}
	expired := time.Now().UTC().Add(-time.Minute)
	updateLease := "UPDATE task_runs SET lease_expires_at = " + store.Placeholder(1) + " WHERE id = " + store.Placeholder(2)
	if _, err := store.DB.ExecContext(ctx, updateLease, store.NowArg(expired), claimed.ID); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	if err := service.RecoverExpiredLeases(ctx); err != nil {
		t.Fatalf("recover leases: %v", err)
	}
	recovered, err := repo.GetRun(ctx, claimed.ID)
	if err != nil {
		t.Fatalf("get recovered run: %v", err)
	}
	if recovered.State != RunQueued || recovered.LeaseOwner != "" || recovered.LeaseExpiresAt != nil {
		t.Fatalf("run was not recovered: %#v", recovered)
	}
}

func TestInvalidSchedulesAreRejected(t *testing.T) {
	ctx := context.Background()
	cfg, store, user, cleanup := newTaskTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store))
	_, _, err := service.CreateTask(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, TaskInput{
		Name:              "Broken cron",
		TaskType:          TaskTypeSystem,
		State:             TaskEnabled,
		Prompt:            "cleanup",
		ScheduleMode:      "cron",
		CronExpression:    "not cron",
		ToolPolicy:        "use_preapproved_tools_only",
		ConcurrencyPolicy: "skip",
	})
	if err == nil {
		t.Fatal("expected invalid cron expression to be rejected")
	}
}

func newTaskTestContext(t *testing.T) (config.Config, *database.Store, auth.User, func()) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cfg := config.Config{
		AppEnv:        "development",
		BaseURL:       "http://localhost:7000",
		Timezone:      "UTC",
		MigrationsDir: filepath.Join("..", "..", "migrations"),
		Database: config.DatabaseConfig{
			Driver: "sqlite",
			URL:    filepath.Join(dir, "test.db"),
		},
		Security: config.SecurityConfig{
			EncryptionKey: key,
			SessionSecret: "test-session-secret-with-enough-length",
			SessionTTL:    time.Hour,
		},
		Tasks: config.TaskConfig{
			DefaultTimeout: time.Minute,
			MaxRetries:     1,
			RunRetention:   time.Hour,
		},
		Chat: config.ChatConfig{RecentMessageLimit: 30, DefaultTimeout: time.Minute},
	}
	store, err := database.Open(ctx, cfg.Database)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.RunMigrations(ctx, store, cfg.MigrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	authService := auth.NewService(auth.NewSQLRepository(store), cfg)
	user, err := authService.CreateOwner(ctx, auth.SetupInput{
		Email:           "owner@example.com",
		Password:        "very-secure-password",
		ConfirmPassword: "very-secure-password",
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	return cfg, store, user, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}
}
