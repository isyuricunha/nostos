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
	"sync"
	"testing"
	"time"

	"github.com/isyuricunha/nostos/internal/auth"
	"github.com/isyuricunha/nostos/internal/chat"
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

func TestAgentTaskUsesAgentMemoriesAndAllowedTool(t *testing.T) {
	ctx := context.Background()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			requestCount++
			var body struct {
				Messages []providers.ChatMessage `json:"messages"`
				Tools    []providers.ChatTool    `json:"tools"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode provider request: %v", err)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			if requestCount == 1 {
				if !taskMessagesContain(body.Messages, "Use the task agent.") || !taskMessagesContain(body.Messages, "Memory: prefer tool-backed checks") {
					t.Fatalf("agent prompt or memory missing from provider request: %#v", body.Messages)
				}
				if len(body.Tools) != 1 || body.Tools[0].Function.Name != "lookup_status" {
					t.Fatalf("allowed task tool missing: %#v", body.Tools)
				}
				fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"id":"task_call_1","type":"function","function":{"name":"lookup_status","arguments":"{\"service\":\"api\"}"}}]},"finish_reason":"tool_calls"}]}`)
				fmt.Fprintln(w)
				fmt.Fprintln(w, `data: [DONE]`)
				return
			}
			if !taskMessagesContainToolResult(body.Messages, "task_call_1", "api is healthy") {
				t.Fatalf("tool result missing from follow-up request: %#v", body.Messages)
			}
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Task used tool."}}]}`)
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
	agentID := seedTaskAgent(t, ctx, store, user.WorkspaceID, provider.ID)
	executions := 0
	service := NewService(cfg, NewSQLRepository(store), authRepo).
		WithProviderClient(providerService, client).
		WithAgentRuntime(
			fakeTaskAgentResolver{agent: chat.AgentContext{
				ID:                    agentID,
				SystemPrompt:          "Use the task agent.",
				DefaultProviderID:     provider.ID,
				DefaultModel:          "mock-model",
				MemoryAccessMode:      "pinned_only",
				ToolPermissionDefault: chat.ToolPermissionAsk,
				MaxToolIterations:     4,
				Temperature:           0.7,
				Active:                true,
			}},
			fakeTaskMemoryProvider{snippets: []chat.MemorySnippet{{ID: "memory_1", Title: "Memory", Content: "prefer tool-backed checks", Score: 1}}},
			&fakeTaskToolProvider{permission: chat.ToolPermissionAllow, executions: &executions},
		)
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	task, _, err := service.CreateTask(ctx, principal, TaskInput{
		Name:              "Agent task",
		TaskType:          TaskTypeAgent,
		State:             TaskEnabled,
		AgentID:           agentID,
		Prompt:            "Check API status.",
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
	if len(runs) != 1 || runs[0].State != RunSucceeded || runs[0].Result != "Task used tool." || executions != 1 || requestCount != 2 {
		t.Fatalf("unexpected agent task result runs=%#v executions=%d requests=%d", runs, executions, requestCount)
	}
	record, err := service.GetRunRecord(ctx, principal, runs[0].ID)
	if err != nil {
		t.Fatalf("get run record: %v", err)
	}
	if !taskEventsContain(record.Events, "Task injected 1 explicit memories.") || !taskEventsContain(record.Events, "Executing tool lookup_status.") {
		t.Fatalf("expected memory and tool events, got %#v", record.Events)
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

func TestScheduleOccurrenceClaimHasSingleWinner(t *testing.T) {
	ctx := context.Background()
	cfg, store, user, cleanup := newTaskTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store))
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	task, _, err := service.CreateTask(ctx, principal, TaskInput{
		Name:              "Single winner schedule",
		TaskType:          TaskTypeSystem,
		State:             TaskEnabled,
		Prompt:            "cleanup_expired_sessions",
		ScheduleMode:      "interval",
		IntervalSeconds:   3600,
		ToolPolicy:        "use_preapproved_tools_only",
		ConcurrencyPolicy: "allow",
	})
	if err != nil {
		t.Fatalf("create scheduled task: %v", err)
	}
	past := time.Now().UTC().Add(-time.Minute)
	if _, err := store.DB.ExecContext(ctx, "UPDATE task_schedules SET next_run_at = "+store.Placeholder(1)+" WHERE task_id = "+store.Placeholder(2), store.NowArg(past), task.ID); err != nil {
		t.Fatalf("force due schedule: %v", err)
	}
	repo := NewSQLRepository(store)
	schedule, err := repo.GetSchedule(ctx, task.ID)
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if schedule.NextRunAt == nil {
		t.Fatal("expected due schedule")
	}
	occurrence := *schedule.NextRunAt
	schedule.LastEnqueuedOccurrence = occurrence.UTC().Format(time.RFC3339)
	schedule.NextRunAt = nextRun(schedule, time.Now().UTC(), cfg.Timezone)

	var wg sync.WaitGroup
	results := make(chan bool, 2)
	errorsCh := make(chan error, 2)
	for index := 0; index < 2; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, err := repo.ClaimScheduleOccurrence(ctx, schedule, occurrence)
			if err != nil {
				errorsCh <- err
				return
			}
			results <- claimed
		}()
	}
	wg.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		t.Fatalf("claim schedule occurrence: %v", err)
	}
	winners := 0
	for claimed := range results {
		if claimed {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("expected exactly one schedule claim winner, got %d", winners)
	}
}

func TestConcurrentClaimsExecuteRunsInParallel(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	active := 0
	maxActive := 0
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		started <- struct{}{}
		<-release
		mu.Lock()
		active--
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"done"}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()

	cfg, store, user, cleanup := newTaskTestContext(t)
	defer cleanup()
	service, provider := newProviderBackedTaskService(t, ctx, cfg, store, user, server.URL)
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	for index := 0; index < 2; index++ {
		task, _, err := service.CreateTask(ctx, principal, TaskInput{
			Name:              fmt.Sprintf("Blocking %d", index),
			TaskType:          TaskTypeAgent,
			State:             TaskEnabled,
			ProviderID:        provider.ID,
			Model:             "mock-model",
			Prompt:            "Run blocking task.",
			ScheduleMode:      "manual",
			ToolPolicy:        "use_preapproved_tools_only",
			ConcurrencyPolicy: "skip",
			MaxRetries:        0,
			TimeoutMS:         30000,
		})
		if err != nil {
			t.Fatalf("create task: %v", err)
		}
		if _, err := service.RunNow(ctx, principal, task.ID); err != nil {
			t.Fatalf("run task: %v", err)
		}
	}
	var wg sync.WaitGroup
	for index := 0; index < 2; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			if err := service.ClaimAndExecute(ctx, fmt.Sprintf("worker-%d", index)); err != nil {
				t.Errorf("claim and execute: %v", err)
			}
		}(index)
	}
	<-started
	<-started
	close(release)
	wg.Wait()
	mu.Lock()
	observed := maxActive
	mu.Unlock()
	if observed < 2 {
		t.Fatalf("expected overlapping task execution, max active=%d", observed)
	}
}

func TestRunningTaskRenewsLease(t *testing.T) {
	ctx := context.Background()
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		close(started)
		<-release
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"done"}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()

	cfg, store, user, cleanup := newTaskTestContext(t)
	defer cleanup()
	cfg.Tasks.DefaultTimeout = 100 * time.Millisecond
	service, provider := newProviderBackedTaskService(t, ctx, cfg, store, user, server.URL)
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	task, _, err := service.CreateTask(ctx, principal, TaskInput{
		Name:              "Renewing task",
		TaskType:          TaskTypeAgent,
		State:             TaskEnabled,
		ProviderID:        provider.ID,
		Model:             "mock-model",
		Prompt:            "Run long enough for renewal.",
		ScheduleMode:      "manual",
		ToolPolicy:        "use_preapproved_tools_only",
		ConcurrencyPolicy: "skip",
		MaxRetries:        0,
		TimeoutMS:         1000,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	queued, err := service.RunNow(ctx, principal, task.ID)
	if err != nil {
		t.Fatalf("run task: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- service.ClaimAndExecute(ctx, "renew-worker")
	}()
	<-started
	time.Sleep(260 * time.Millisecond)
	running, err := NewSQLRepository(store).GetRun(ctx, queued.ID)
	if err != nil {
		t.Fatalf("get running run: %v", err)
	}
	if running.State != RunRunning || running.LeaseExpiresAt == nil || !running.LeaseExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("lease was not renewed: %#v", running)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("claim and execute: %v", err)
	}
}

func TestConcurrencyPoliciesSkipAndReplace(t *testing.T) {
	ctx := context.Background()
	cfg, store, user, cleanup := newTaskTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store))
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	skipTask, _, err := service.CreateTask(ctx, principal, TaskInput{
		Name:              "Skip overlap",
		TaskType:          TaskTypeSystem,
		State:             TaskEnabled,
		Prompt:            "cleanup_expired_sessions",
		ScheduleMode:      "manual",
		ToolPolicy:        "use_preapproved_tools_only",
		ConcurrencyPolicy: "skip",
	})
	if err != nil {
		t.Fatalf("create skip task: %v", err)
	}
	if _, err := service.RunNow(ctx, principal, skipTask.ID); err != nil {
		t.Fatalf("first skip run: %v", err)
	}
	if _, err := service.RunNow(ctx, principal, skipTask.ID); err == nil {
		t.Fatal("expected second skip run to be rejected while first is queued")
	}
	replaceTask, _, err := service.CreateTask(ctx, principal, TaskInput{
		Name:              "Replace overlap",
		TaskType:          TaskTypeSystem,
		State:             TaskEnabled,
		Prompt:            "cleanup_expired_sessions",
		ScheduleMode:      "manual",
		ToolPolicy:        "use_preapproved_tools_only",
		ConcurrencyPolicy: "replace",
	})
	if err != nil {
		t.Fatalf("create replace task: %v", err)
	}
	first, err := service.RunNow(ctx, principal, replaceTask.ID)
	if err != nil {
		t.Fatalf("first replace run: %v", err)
	}
	second, err := service.RunNow(ctx, principal, replaceTask.ID)
	if err != nil {
		t.Fatalf("second replace run: %v", err)
	}
	if first.ID == second.ID {
		t.Fatal("replace policy did not enqueue a new run")
	}
	replaced, err := NewSQLRepository(store).GetRun(ctx, first.ID)
	if err != nil {
		t.Fatalf("get replaced run: %v", err)
	}
	if replaced.State != RunCancelled {
		t.Fatalf("replace policy did not cancel previous run: %#v", replaced)
	}
}

func TestOneTimeScheduleDisablesAfterEnqueue(t *testing.T) {
	ctx := context.Background()
	cfg, store, user, cleanup := newTaskTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store))
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	runAt := time.Now().UTC().Add(-time.Minute)
	task, _, err := service.CreateTask(ctx, principal, TaskInput{
		Name:              "One time",
		TaskType:          TaskTypeSystem,
		State:             TaskEnabled,
		Prompt:            "cleanup_expired_sessions",
		ScheduleMode:      "one_time",
		RunAt:             runAt.Format(time.RFC3339),
		ToolPolicy:        "use_preapproved_tools_only",
		ConcurrencyPolicy: "allow",
	})
	if err != nil {
		t.Fatalf("create one-time task: %v", err)
	}
	if err := service.EnqueueDueSchedules(ctx); err != nil {
		t.Fatalf("enqueue due one-time: %v", err)
	}
	schedule, err := NewSQLRepository(store).GetSchedule(ctx, task.ID)
	if err != nil {
		t.Fatalf("get schedule: %v", err)
	}
	if schedule.Enabled || schedule.NextRunAt != nil {
		t.Fatalf("one-time schedule was not disabled: %#v", schedule)
	}
	runs, err := service.ListRuns(ctx, principal, task.ID)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one run for one-time schedule, got %#v", runs)
	}
	if err := service.EnqueueDueSchedules(ctx); err != nil {
		t.Fatalf("enqueue due one-time again: %v", err)
	}
	runs, err = service.ListRuns(ctx, principal, task.ID)
	if err != nil {
		t.Fatalf("list runs again: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("one-time schedule enqueued more than once: %#v", runs)
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

type fakeTaskAgentResolver struct {
	agent chat.AgentContext
}

func (f fakeTaskAgentResolver) GetChatAgent(ctx context.Context, workspaceID string, agentID string) (chat.AgentContext, error) {
	return f.agent, nil
}

type fakeTaskMemoryProvider struct {
	snippets []chat.MemorySnippet
}

func (f fakeTaskMemoryProvider) SelectForRun(ctx context.Context, request chat.MemoryRequest) ([]chat.MemorySnippet, error) {
	return f.snippets, nil
}

type fakeTaskToolProvider struct {
	permission string
	executions *int
}

func (f fakeTaskToolProvider) RuntimeTools(ctx context.Context, request chat.ToolExposureRequest) ([]chat.RuntimeTool, error) {
	mode := f.permission
	if mode == "" {
		mode = chat.ToolPermissionAllow
	}
	return []chat.RuntimeTool{{
		ID:             "task_tool_1",
		Name:           "lookup_status",
		ProviderName:   "lookup_status",
		Description:    "Look up service status.",
		InputSchema:    `{"type":"object","properties":{"service":{"type":"string"}}}`,
		PermissionMode: mode,
	}}, nil
}

func (f fakeTaskToolProvider) ExecuteRuntimeTool(ctx context.Context, request chat.ToolExecutionRequest) (chat.ToolExecutionResult, error) {
	if request.ProviderName != "lookup_status" || request.Arguments != `{"service":"api"}` {
		return chat.ToolExecutionResult{}, fmt.Errorf("unexpected tool execution: %#v", request)
	}
	if f.executions != nil {
		*f.executions++
	}
	return chat.ToolExecutionResult{Content: "api is healthy"}, nil
}

func seedTaskAgent(t *testing.T, ctx context.Context, store *database.Store, workspaceID string, providerID string) string {
	t.Helper()
	agentID := "task_agent_test"
	now := store.NowArg(time.Now().UTC())
	if _, err := store.DB.ExecContext(ctx, `INSERT INTO agents (id, workspace_id, name, system_prompt, default_provider_id, default_model, max_tool_iterations, memory_access_mode, tool_permission_default, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agentID, workspaceID, "Task Agent", "Use the task agent.", providerID, "mock-model", 4, "pinned_only", "ask", true, now, now); err != nil {
		t.Fatalf("seed task agent: %v", err)
	}
	return agentID
}

func newProviderBackedTaskService(t *testing.T, ctx context.Context, cfg config.Config, store *database.Store, user auth.User, providerURL string) (*Service, providers.Provider) {
	t.Helper()
	authRepo := auth.NewSQLRepository(store)
	client := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, client)
	apiKey := "test-api-key"
	provider, err := providerService.Create(ctx, providers.PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, providers.ProviderInput{
		Name:             "Mock " + idSuffix(),
		BaseURL:          providerURL,
		APIKey:           &apiKey,
		Enabled:          true,
		RequestTimeoutMS: 5000,
		DefaultModel:     "mock-model",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	return NewService(cfg, NewSQLRepository(store), authRepo).WithProviderClient(providerService, client), provider
}

func idSuffix() string {
	return strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
}

func taskMessagesContain(messages []providers.ChatMessage, value string) bool {
	for _, message := range messages {
		if strings.Contains(message.Content, value) {
			return true
		}
	}
	return false
}

func taskMessagesContainToolResult(messages []providers.ChatMessage, toolCallID string, content string) bool {
	for _, message := range messages {
		if message.Role == "tool" && message.ToolCallID == toolCallID && message.Content == content {
			return true
		}
	}
	return false
}

func taskEventsContain(events []Event, message string) bool {
	for _, event := range events {
		if event.Message == message {
			return true
		}
	}
	return false
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
