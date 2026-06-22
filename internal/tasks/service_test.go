package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestSystemMaintenanceTasksPerformRealWork(t *testing.T) {
	ctx := context.Background()
	cfg, store, user, cleanup := newTaskTestContext(t)
	defer cleanup()
	cfg.DataDir = t.TempDir()
	repo := NewSQLRepository(store)
	service := NewService(cfg, repo, auth.NewSQLRepository(store))

	authService := auth.NewService(auth.NewSQLRepository(store), cfg)
	if _, err := authService.CreateSessionForUser(ctx, user, "127.0.0.1", "test"); err != nil {
		t.Fatalf("create session: %v", err)
	}
	expired := time.Now().UTC().Add(-time.Hour)
	if _, err := store.DB.ExecContext(ctx, "UPDATE sessions SET expires_at = "+store.Placeholder(1), store.NowArg(expired)); err != nil {
		t.Fatalf("expire session: %v", err)
	}
	result, err := service.executeSystemTask(ctx, Task{Name: "cleanup_expired_sessions"})
	if err != nil {
		t.Fatalf("cleanup expired sessions: %v", err)
	}
	if !strings.Contains(result, "revoked=1") {
		t.Fatalf("unexpected cleanup result %q", result)
	}
	var revokedCount int
	if err := store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions WHERE revoked_at IS NOT NULL").Scan(&revokedCount); err != nil {
		t.Fatalf("count revoked sessions: %v", err)
	}
	if revokedCount != 1 {
		t.Fatalf("expected one revoked session, got %d", revokedCount)
	}

	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	task, _, err := service.CreateTask(ctx, principal, TaskInput{
		Name:              "Event task",
		TaskType:          TaskTypeSystem,
		State:             TaskEnabled,
		Prompt:            "cleanup",
		ScheduleMode:      "manual",
		ToolPolicy:        "use_preapproved_tools_only",
		ConcurrencyPolicy: "skip",
	})
	if err != nil {
		t.Fatalf("create event task: %v", err)
	}
	run, err := service.RunNow(ctx, principal, task.ID)
	if err != nil {
		t.Fatalf("run event task: %v", err)
	}
	if err := repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "info", Message: "scheduled occurrence enqueued"}); err != nil {
		t.Fatalf("append first event: %v", err)
	}
	if err := repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "info", Message: "scheduled occurrence enqueued"}); err != nil {
		t.Fatalf("append duplicate event: %v", err)
	}
	result, err = service.executeSystemTask(ctx, Task{Name: "compact_duplicate_task_scheduling_events"})
	if err != nil {
		t.Fatalf("compact duplicate events: %v", err)
	}
	if !strings.Contains(result, "compacted=1") {
		t.Fatalf("unexpected compact result %q", result)
	}
	if err := repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "info", Message: "old event"}); err != nil {
		t.Fatalf("append old event: %v", err)
	}
	old := time.Now().UTC().Add(-2 * time.Hour)
	if _, err := store.DB.ExecContext(ctx, "UPDATE task_run_events SET created_at = "+store.Placeholder(1)+" WHERE message = "+store.Placeholder(2), store.NowArg(old), "old event"); err != nil {
		t.Fatalf("age old event: %v", err)
	}
	result, err = service.executeSystemTask(ctx, Task{Name: "prune_old_task_run_events"})
	if err != nil {
		t.Fatalf("prune old events: %v", err)
	}
	if !strings.Contains(result, "pruned=1") {
		t.Fatalf("unexpected prune result %q", result)
	}

	tmpDir := filepath.Join(cfg.DataDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o750); err != nil {
		t.Fatalf("create tmp dir: %v", err)
	}
	tmpFile := filepath.Join(tmpDir, "old.tmp")
	if err := os.WriteFile(tmpFile, []byte("temporary"), 0o600); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}
	if err := os.Chtimes(tmpFile, old, old); err != nil {
		t.Fatalf("age tmp file: %v", err)
	}
	result, err = service.executeSystemTask(ctx, Task{Name: "cleanup_temporary_files"})
	if err != nil {
		t.Fatalf("cleanup temporary files: %v", err)
	}
	if !strings.Contains(result, "deleted=1") {
		t.Fatalf("unexpected tmp cleanup result %q", result)
	}
	if _, err := os.Stat(tmpFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file was not removed, stat err=%v", err)
	}
}

func TestSystemMaintenanceTaskDispatchesDomainHandlers(t *testing.T) {
	ctx := context.Background()
	cfg, store, _, cleanup := newTaskTestContext(t)
	defer cleanup()
	provider := &fakeProviderMaintenance{}
	chatHandler := &fakeChatMaintenance{}
	mcpHandler := &fakeMCPMaintenance{}
	summaryHandler := &fakeSummaryMaintenance{}
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store)).
		WithProviderClient(provider, nil).
		WithConversationSummaryHandler(summaryHandler).
		WithChatMaintenanceHandler(chatHandler).
		WithMCPHealthHandler(mcpHandler)
	for _, name := range []string{
		"update_conversation_summaries",
		"check_provider_health",
		"check_mcp_server_health",
		"cleanup_abandoned_chat_runs",
		"recalculate_conversation_titles",
		"recover_expired_task_leases",
	} {
		if _, err := service.executeSystemTask(ctx, Task{Name: name}); err != nil {
			t.Fatalf("execute %s: %v", name, err)
		}
	}
	if summaryHandler.calls != 1 || provider.calls != 1 || mcpHandler.calls != 1 || chatHandler.cleanupCalls != 1 || chatHandler.titleCalls != 1 {
		t.Fatalf("handlers were not dispatched correctly: summary=%d provider=%d mcp=%d chat_cleanup=%d chat_titles=%d",
			summaryHandler.calls, provider.calls, mcpHandler.calls, chatHandler.cleanupCalls, chatHandler.titleCalls)
	}
}

type fakeProviderMaintenance struct {
	calls int
}

func (f *fakeProviderMaintenance) ResolveForChat(ctx context.Context, workspaceID string, providerID string) (providers.Provider, string, error) {
	return providers.Provider{}, "", nil
}

func (f *fakeProviderMaintenance) CheckProviderHealth(ctx context.Context, limit int) (string, error) {
	f.calls++
	return "provider health checked=0 healthy=0 unhealthy=0", nil
}

type fakeChatMaintenance struct {
	cleanupCalls int
	titleCalls   int
}

func (f *fakeChatMaintenance) CleanupAbandonedChatRuns(ctx context.Context) (string, error) {
	f.cleanupCalls++
	return "abandoned chat runs cleaned=0", nil
}

func (f *fakeChatMaintenance) RecalculateConversationTitles(ctx context.Context, limit int) (string, error) {
	f.titleCalls++
	return "conversation titles recalculated=0", nil
}

type fakeMCPMaintenance struct {
	calls int
}

func (f *fakeMCPMaintenance) CheckMCPServerHealth(ctx context.Context, limit int) (string, error) {
	f.calls++
	return "MCP server health checked=0 healthy=0 unhealthy=0", nil
}

type fakeSummaryMaintenance struct {
	calls int
}

func (f *fakeSummaryMaintenance) UpdateConversationSummaries(ctx context.Context, limit int) (string, error) {
	f.calls++
	return "conversation summaries processed=0 failed=0", nil
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
