package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/isyuricunha/nostos/internal/auth"
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
	"github.com/isyuricunha/nostos/internal/providers"
	"github.com/isyuricunha/nostos/internal/tasks"
)

func TestRunStreamsAndPersistsConversation(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "mock-model"}}})
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Hello"}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":" from mock"}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: {"usage":{"prompt_tokens":4,"completion_tokens":3,"total_tokens":7}}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: [DONE]`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg, store, user, cleanup := newChatTestContext(t)
	defer cleanup()
	authRepo := auth.NewSQLRepository(store)
	providerClient := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, providerClient)
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

	repo := NewSQLRepository(store)
	memoryProvider := &fakeMemoryProvider{snippets: []MemorySnippet{{ID: "mem_1", Title: "Greeting style", Content: "Prefer concise greetings.", Score: 2.4}}}
	service := NewService(cfg, repo, providerService, providerClient, fakeAgentResolver{}, memoryProvider)
	conversation, err := service.CreateConversation(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, Conversation{
		Title:      "Mock chat",
		ProviderID: provider.ID,
		Model:      "mock-model",
	})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	var events []string
	runID := ""
	err = service.Run(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, conversation.ID, RunInput{Content: "Say hello"}, func(event string, payload any) error {
		events = append(events, event)
		if event == "run_started" {
			if started, ok := payload.(map[string]any); ok {
				if run, ok := started["run"].(ChatRun); ok {
					runID = run.ID
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("run chat: %v", err)
	}
	if len(events) == 0 || events[0] != "run_started" {
		t.Fatalf("expected stream events, got %#v", events)
	}
	messages, err := service.ListMessages(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, conversation.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected user and assistant messages, got %d", len(messages))
	}
	if messages[1].Content != "Hello from mock" || messages[1].TotalTokens != 7 {
		t.Fatalf("assistant message was not persisted correctly: %#v", messages[1])
	}
	if memoryProvider.recordedRunID != runID || len(memoryProvider.recorded) != 1 {
		t.Fatalf("memory injection was not recorded: %#v", memoryProvider)
	}
}

func TestRunSendsPersistedMultiTurnHistoryToProvider(t *testing.T) {
	ctx := context.Background()
	var recorder requestRecorder
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		messages := recorder.record(t, r)
		w.Header().Set("Content-Type", "text/event-stream")
		if containsContent(messages, "What is my name?") {
			writeStreamContent(w, "Your name is Yuri.")
			return
		}
		writeStreamContent(w, "Nice to meet you, Yuri.")
	}))
	defer server.Close()

	cfg, store, user, cleanup := newChatTestContext(t)
	defer cleanup()
	authRepo := auth.NewSQLRepository(store)
	providerClient := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, providerClient)
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
	service := NewService(cfg, NewSQLRepository(store), providerService, providerClient, fakeAgentResolver{}, &fakeMemoryProvider{})
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	conversation, err := service.CreateConversation(ctx, principal, Conversation{Title: "Memory-free chat", ProviderID: provider.ID, Model: "mock-model"})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := service.Run(ctx, principal, conversation.ID, RunInput{Content: "My name is Yuri."}, noopSink); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := service.Run(ctx, principal, conversation.ID, RunInput{Content: "What is my name?"}, noopSink); err != nil {
		t.Fatalf("second run: %v", err)
	}
	requests := recorder.requests()
	if len(requests) != 2 {
		t.Fatalf("expected two provider requests, got %d", len(requests))
	}
	second := requests[1]
	if !containsContent(second, "My name is Yuri.") {
		t.Fatalf("second provider request did not include first user turn: %#v", second)
	}
	if !containsContent(second, "Nice to meet you, Yuri.") {
		t.Fatalf("second provider request did not include first assistant turn: %#v", second)
	}
	if countContent(second, "What is my name?") != 1 {
		t.Fatalf("current user message should appear exactly once: %#v", second)
	}
}

func TestBranchContextExcludesSiblingMessagesInProviderRequest(t *testing.T) {
	ctx := context.Background()
	var recorder requestRecorder
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		messages := recorder.record(t, r)
		w.Header().Set("Content-Type", "text/event-stream")
		switch {
		case containsContent(messages, "Edited branch route"):
			writeStreamContent(w, "Edited branch response.")
		case containsContent(messages, "Sibling-only detail"):
			writeStreamContent(w, "Sibling detail acknowledged.")
		default:
			writeStreamContent(w, "Root response.")
		}
	}))
	defer server.Close()

	cfg, store, user, cleanup := newChatTestContext(t)
	defer cleanup()
	authRepo := auth.NewSQLRepository(store)
	providerClient := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, providerClient)
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
	service := NewService(cfg, NewSQLRepository(store), providerService, providerClient, fakeAgentResolver{}, &fakeMemoryProvider{})
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	conversation, err := service.CreateConversation(ctx, principal, Conversation{Title: "Branch chat", ProviderID: provider.ID, Model: "mock-model"})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := service.Run(ctx, principal, conversation.ID, RunInput{Content: "Original route"}, noopSink); err != nil {
		t.Fatalf("first run: %v", err)
	}
	messages, err := service.ListMessages(ctx, principal, conversation.ID)
	if err != nil {
		t.Fatalf("list messages after first run: %v", err)
	}
	firstUserID := messages[0].ID
	if err := service.Run(ctx, principal, conversation.ID, RunInput{Content: "Sibling-only detail"}, noopSink); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if err := service.EditAndBranch(ctx, principal, firstUserID, RunInput{Content: "Edited branch route"}, noopSink); err != nil {
		t.Fatalf("edit branch run: %v", err)
	}
	requests := recorder.requests()
	if len(requests) != 3 {
		t.Fatalf("expected three provider requests, got %d", len(requests))
	}
	branchRequest := requests[2]
	if !containsContent(branchRequest, "Edited branch route") {
		t.Fatalf("branch request did not include edited branch message: %#v", branchRequest)
	}
	if containsContent(branchRequest, "Sibling-only detail") || containsContent(branchRequest, "Sibling detail acknowledged.") {
		t.Fatalf("branch request included sibling-only messages: %#v", branchRequest)
	}
}

func TestBuildPromptMessagesPreservesToolCallsAndResults(t *testing.T) {
	now := time.Now().UTC()
	result := BuildPromptMessages(ContextRequest{
		Conversation: Conversation{Summary: "The user is debugging service health."},
		Agent:        AgentContext{SystemPrompt: "Use tools carefully."},
		Messages: []Message{
			{ID: "msg_user", Role: RoleUser, Content: "Check API status", CreatedAt: now},
			{ID: "msg_assistant_tool", ParentMessageID: "msg_user", Role: RoleAssistant, ToolCalls: []providers.ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: providers.ToolCallFunction{
					Name:      "lookup_status",
					Arguments: `{"service":"api"}`,
				},
			}}, CreatedAt: now.Add(time.Second)},
			{ID: "msg_tool", ParentMessageID: "msg_assistant_tool", Role: RoleTool, ToolCallID: "call_1", Content: "api is healthy", CreatedAt: now.Add(2 * time.Second)},
			{ID: "msg_current", ParentMessageID: "msg_tool", Role: RoleUser, Content: "What did the tool say?", CreatedAt: now.Add(3 * time.Second)},
		},
		CurrentUserMessageID: "msg_current",
		RecentMessageLimit:   30,
		ContextThreshold:     60000,
	})
	if !containsToolCall(result.Messages, "call_1", "lookup_status") {
		t.Fatalf("assistant tool call was not reconstructed: %#v", result.Messages)
	}
	if !containsToolResult(result.Messages, "call_1", "api is healthy") {
		t.Fatalf("tool result was not reconstructed: %#v", result.Messages)
	}
	if countContent(result.Messages, "What did the tool say?") != 1 {
		t.Fatalf("current user message should appear exactly once: %#v", result.Messages)
	}
}

func TestConversationSummaryQueueWorkerAndInjection(t *testing.T) {
	ctx := context.Background()
	var recorder requestRecorder
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		messages := recorder.record(t, r)
		w.Header().Set("Content-Type", "text/event-stream")
		if containsContentSubstring(messages, "Older transcript to compact") {
			writeStreamContent(w, "Summary: Yuri prefers Go for backend services.")
			return
		}
		writeStreamContent(w, "Acknowledged.")
	}))
	defer server.Close()

	cfg, store, user, cleanup := newChatTestContext(t)
	defer cleanup()
	cfg.Chat.ContextThreshold = 20
	cfg.Chat.RecentMessageLimit = 1
	cfg.Tasks.DefaultTimeout = time.Minute
	cfg.Tasks.MaxRetries = 3
	authRepo := auth.NewSQLRepository(store)
	providerClient := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, providerClient)
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
	taskService := tasks.NewService(cfg, tasks.NewSQLRepository(store), authRepo).WithProviderClient(providerService, providerClient)
	if err := taskService.EnsureSystemTasks(ctx); err != nil {
		t.Fatalf("ensure system tasks: %v", err)
	}
	service := NewService(cfg, NewSQLRepository(store), providerService, providerClient, fakeAgentResolver{}, &fakeMemoryProvider{}).WithSummaryEnqueuer(taskService)
	taskService.WithConversationSummaryHandler(service)
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	taskPrincipal := tasks.PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	conversation, err := service.CreateConversation(ctx, principal, Conversation{Title: "Summary chat", ProviderID: provider.ID, Model: "mock-model"})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := service.Run(ctx, principal, conversation.ID, RunInput{Content: "My name is Yuri and I prefer Go for backend services."}, noopSink); err != nil {
		t.Fatalf("first run: %v", err)
	}
	queuedConversation, err := service.GetConversation(ctx, principal, conversation.ID)
	if err != nil {
		t.Fatalf("get queued conversation: %v", err)
	}
	if queuedConversation.SummaryStatus != "queued" {
		t.Fatalf("expected queued summary status, got %#v", queuedConversation)
	}
	if _, queued, err := service.QueueSummary(ctx, principal, conversation.ID); err != nil || queued {
		t.Fatalf("duplicate summary queue should be ignored while queued, queued=%v err=%v", queued, err)
	}
	summaryTask := findTaskRecord(t, taskService, taskPrincipal, "update_conversation_summaries")
	runs, err := taskService.ListRuns(ctx, taskPrincipal, summaryTask.Task.ID)
	if err != nil {
		t.Fatalf("list summary task runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one deduplicated summary run, got %d", len(runs))
	}
	if err := taskService.ClaimAndExecute(ctx, "summary-worker"); err != nil {
		t.Fatalf("execute summary task: %v", err)
	}
	summarized, err := service.GetConversation(ctx, principal, conversation.ID)
	if err != nil {
		t.Fatalf("get summarized conversation: %v", err)
	}
	if summarized.Summary != "Summary: Yuri prefers Go for backend services." || summarized.SummaryStatus != "idle" {
		t.Fatalf("summary was not stored correctly: %#v", summarized)
	}
	if summarized.SummarySourceStartMessageID == "" || summarized.SummarySourceEndMessageID == "" || summarized.SummaryProviderID != provider.ID || summarized.SummaryModel != "mock-model" || summarized.SummaryVersion != 1 {
		t.Fatalf("summary metadata was not stored: %#v", summarized)
	}
	storedMessages, err := service.ListMessages(ctx, principal, conversation.ID)
	if err != nil {
		t.Fatalf("list messages after summary: %v", err)
	}
	if len(storedMessages) != 2 {
		t.Fatalf("summary must not delete original messages, got %d", len(storedMessages))
	}
	if err := service.Run(ctx, principal, conversation.ID, RunInput{Content: "What context do you have?"}, noopSink); err != nil {
		t.Fatalf("run after summary: %v", err)
	}
	if !anyRequestContains(recorder.requests(), "Conversation summary:\nSummary: Yuri prefers Go for backend services.") {
		t.Fatalf("next chat request did not include stored summary: %#v", recorder.requests())
	}
	cleared, err := service.ClearSummary(ctx, principal, conversation.ID)
	if err != nil {
		t.Fatalf("clear summary: %v", err)
	}
	if cleared.Summary != "" || cleared.SummaryStatus != "idle" {
		t.Fatalf("summary was not cleared: %#v", cleared)
	}
	_, queued, err := service.QueueSummary(ctx, principal, conversation.ID)
	if err != nil {
		t.Fatalf("manual regenerate queue: %v", err)
	}
	if !queued {
		t.Fatal("expected manual regenerate to queue a new summary")
	}
}

func TestRunExecutesAllowedToolAndStreamsFollowup(t *testing.T) {
	ctx := context.Background()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		requestCount++
		var body struct {
			Tools    []providers.ChatTool    `json:"tools"`
			Messages []providers.ChatMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if requestCount == 1 {
			if len(body.Tools) != 1 || body.Tools[0].Function.Name != "lookup_status" {
				t.Fatalf("tool definitions were not sent: %#v", body.Tools)
			}
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup_status","arguments":"{\"service\":\"api\"}"}}]},"finish_reason":"tool_calls"}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: [DONE]`)
			return
		}
		hasToolResult := false
		for _, message := range body.Messages {
			if message.Role == "tool" && message.ToolCallID == "call_1" && message.Content == "api is healthy" {
				hasToolResult = true
			}
		}
		if !hasToolResult {
			t.Fatalf("tool result was not sent to follow-up model request: %#v", body.Messages)
		}
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"The API is healthy."}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()

	cfg, store, user, cleanup := newChatTestContext(t)
	defer cleanup()
	authRepo := auth.NewSQLRepository(store)
	providerClient := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, providerClient)
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
	agentID := "agent_test_tool"
	now := store.NowArg(time.Now().UTC())
	if _, err := store.DB.ExecContext(ctx, `INSERT INTO agents (id, workspace_id, name, system_prompt, default_provider_id, default_model, max_tool_iterations, memory_access_mode, tool_permission_default, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agentID, user.WorkspaceID, "Tool Agent", "Use tools when helpful.", provider.ID, "mock-model", 4, "pinned_only", "ask", true, now, now); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	service := NewService(cfg, NewSQLRepository(store), providerService, providerClient, fakeAgentResolver{}, &fakeMemoryProvider{}).WithToolProvider(fakeToolProvider{})
	conversation, err := service.CreateConversation(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, Conversation{
		Title:      "Tool chat",
		AgentID:    agentID,
		ProviderID: provider.ID,
		Model:      "mock-model",
	})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	var events []string
	err = service.Run(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, conversation.ID, RunInput{Content: "Check API status"}, func(event string, payload any) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("run chat with tool: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected two provider requests, got %d", requestCount)
	}
	messages, err := service.ListMessages(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, conversation.ID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 3 || messages[1].Content != "The API is healthy." || messages[2].Role != RoleTool || messages[2].Content != "api is healthy" {
		t.Fatalf("assistant follow-up was not persisted: %#v", messages)
	}
	if !containsEvent(events, "tool_result") {
		t.Fatalf("tool result event was not emitted: %#v", events)
	}
}

func TestAskToolPausesApprovesOnceAndResumes(t *testing.T) {
	ctx := context.Background()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		requestCount++
		var body struct {
			Messages []providers.ChatMessage `json:"messages"`
			Tools    []providers.ChatTool    `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if requestCount == 1 {
			if len(body.Tools) != 1 {
				t.Fatalf("ask tool was not exposed to provider: %#v", body.Tools)
			}
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"id":"call_ask","type":"function","function":{"name":"lookup_status","arguments":"{\"service\":\"api\"}"}}]},"finish_reason":"tool_calls"}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: [DONE]`)
			return
		}
		hasToolResult := false
		for _, message := range body.Messages {
			if message.Role == RoleTool && message.ToolCallID == "call_ask" && message.Content == "api is healthy" {
				hasToolResult = true
			}
		}
		if !hasToolResult {
			t.Fatalf("approved tool result was not sent on resume: %#v", body.Messages)
		}
		fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Resumed with tool result."}}]}`)
		fmt.Fprintln(w)
		fmt.Fprintln(w, `data: [DONE]`)
	}))
	defer server.Close()

	cfg, store, user, cleanup := newChatTestContext(t)
	defer cleanup()
	authRepo := auth.NewSQLRepository(store)
	providerClient := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, providerClient)
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
	agentID := seedChatAgent(t, ctx, store, user.WorkspaceID, provider.ID)
	executions := 0
	service := NewService(cfg, NewSQLRepository(store), providerService, providerClient, fakeAgentResolver{}, &fakeMemoryProvider{}).
		WithToolProvider(&fakeToolProvider{permission: ToolPermissionAsk, executions: &executions})
	conversation, err := service.CreateConversation(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, Conversation{
		Title:      "Approval chat",
		AgentID:    agentID,
		ProviderID: provider.ID,
		Model:      "mock-model",
	})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	var runID string
	var events []string
	if err := service.Run(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, conversation.ID, RunInput{Content: "Check status with approval"}, func(event string, payload any) error {
		events = append(events, event)
		if event == "run_started" {
			data, ok := payload.(map[string]any)
			if !ok {
				t.Fatalf("unexpected run_started payload: %#v", payload)
			}
			run, ok := data["run"].(ChatRun)
			if !ok {
				t.Fatalf("unexpected run payload: %#v", data["run"])
			}
			runID = run.ID
		}
		return nil
	}); err != nil {
		t.Fatalf("run chat: %v", err)
	}
	if !containsEvent(events, "tool_approval_required") || requestCount != 1 || executions != 0 {
		t.Fatalf("run did not pause for approval events=%#v requests=%d executions=%d", events, requestCount, executions)
	}
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	pending, err := service.ListPendingToolApprovals(ctx, principal)
	if err != nil {
		t.Fatalf("list pending approvals: %v", err)
	}
	if len(pending) != 1 || pending[0].ChatRunID != runID || pending[0].State != ToolCallWaitingForApproval {
		t.Fatalf("unexpected pending approvals: %#v", pending)
	}
	if _, err := service.ApproveToolCall(ctx, principal, pending[0].ID, ToolDecisionApproveOnce); err != nil {
		t.Fatalf("approve tool: %v", err)
	}
	var resumeEvents []string
	if err := service.ResumeRun(ctx, principal, runID, func(event string, payload any) error {
		resumeEvents = append(resumeEvents, event)
		return nil
	}); err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if executions != 1 || requestCount != 2 || !containsEvent(resumeEvents, "run_completed") {
		t.Fatalf("resume did not execute exactly once requests=%d executions=%d events=%#v", requestCount, executions, resumeEvents)
	}
	remaining, err := service.ListPendingToolApprovals(ctx, principal)
	if err != nil {
		t.Fatalf("list remaining approvals: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("approval remained pending: %#v", remaining)
	}
}

func TestToolLoopSupportsMultipleIterations(t *testing.T) {
	ctx := context.Background()
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		requestCount++
		var body struct {
			Messages []providers.ChatMessage `json:"messages"`
			Tools    []providers.ChatTool    `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if len(body.Tools) != 1 {
			t.Fatalf("tool definitions were not preserved on iteration %d: %#v", requestCount, body.Tools)
		}
		switch requestCount {
		case 1:
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":%q,\"arguments\":\"{\\\"service\\\":\\\"api\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n", body.Tools[0].Function.Name)
			fmt.Fprintln(w, `data: [DONE]`)
		case 2:
			if !providerMessagesIncludeToolResult(body.Messages, "call_1", "api is healthy") {
				t.Fatalf("first tool result missing from second request: %#v", body.Messages)
			}
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"id\":\"call_2\",\"type\":\"function\",\"function\":{\"name\":%q,\"arguments\":\"{\\\"service\\\":\\\"api\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n", body.Tools[0].Function.Name)
			fmt.Fprintln(w, `data: [DONE]`)
		case 3:
			if !providerMessagesIncludeToolResult(body.Messages, "call_1", "api is healthy") || !providerMessagesIncludeToolResult(body.Messages, "call_2", "api is healthy") {
				t.Fatalf("tool results missing from final request: %#v", body.Messages)
			}
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Both checks passed."}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: [DONE]`)
		default:
			t.Fatalf("unexpected provider request count %d", requestCount)
		}
	}))
	defer server.Close()

	cfg, store, user, cleanup := newChatTestContext(t)
	defer cleanup()
	authRepo := auth.NewSQLRepository(store)
	providerClient := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, providerClient)
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
	agentID := seedChatAgent(t, ctx, store, user.WorkspaceID, provider.ID)
	executions := 0
	service := NewService(cfg, NewSQLRepository(store), providerService, providerClient, fakeAgentResolver{}, &fakeMemoryProvider{}).
		WithToolProvider(&fakeToolProvider{permission: ToolPermissionAllow, executions: &executions})
	conversation, err := service.CreateConversation(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, Conversation{
		Title:      "Loop chat",
		AgentID:    agentID,
		ProviderID: provider.ID,
		Model:      "mock-model",
	})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	var events []string
	if err := service.Run(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, conversation.ID, RunInput{Content: "Check twice"}, func(event string, payload any) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("run chat: %v", err)
	}
	if requestCount != 3 || executions != 2 || !containsEvent(events, "run_completed") {
		t.Fatalf("multi-iteration tool loop failed requests=%d executions=%d events=%#v", requestCount, executions, events)
	}
	calls, err := NewSQLRepository(store).ListToolCallsForRun(ctx, latestRunID(t, ctx, store))
	if err != nil {
		t.Fatalf("list tool calls: %v", err)
	}
	if len(calls) != 2 || calls[0].State != ToolCallSucceeded || calls[1].State != ToolCallSucceeded {
		t.Fatalf("tool calls were not persisted as succeeded: %#v", calls)
	}
}

type requestRecorder struct {
	mu       sync.Mutex
	messages [][]providers.ChatMessage
}

func (r *requestRecorder) record(t *testing.T, request *http.Request) []providers.ChatMessage {
	t.Helper()
	var body struct {
		Messages []providers.ChatMessage `json:"messages"`
	}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		t.Fatalf("decode provider request: %v", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := append([]providers.ChatMessage{}, body.Messages...)
	r.messages = append(r.messages, copied)
	return copied
}

func (r *requestRecorder) requests() [][]providers.ChatMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]providers.ChatMessage, len(r.messages))
	for index := range r.messages {
		out[index] = append([]providers.ChatMessage{}, r.messages[index]...)
	}
	return out
}

func noopSink(event string, payload any) error {
	return nil
}

func writeStreamContent(w http.ResponseWriter, content string) {
	fmt.Fprintf(w, `data: {"choices":[{"delta":{"content":%q}}]}`+"\n\n", content)
	fmt.Fprintln(w, `data: [DONE]`)
}

func containsContent(messages []providers.ChatMessage, content string) bool {
	return countContent(messages, content) > 0
}

func containsContentSubstring(messages []providers.ChatMessage, content string) bool {
	for _, message := range messages {
		if strings.Contains(message.Content, content) {
			return true
		}
	}
	return false
}

func providerMessagesIncludeToolResult(messages []providers.ChatMessage, toolCallID string, content string) bool {
	for _, message := range messages {
		if message.Role == RoleTool && message.ToolCallID == toolCallID && message.Content == content {
			return true
		}
	}
	return false
}

func latestRunID(t *testing.T, ctx context.Context, store *database.Store) string {
	t.Helper()
	var runID string
	if err := store.DB.QueryRowContext(ctx, `SELECT id FROM chat_runs ORDER BY created_at DESC LIMIT 1`).Scan(&runID); err != nil {
		t.Fatalf("read latest run: %v", err)
	}
	return runID
}

func anyRequestContains(requests [][]providers.ChatMessage, content string) bool {
	for _, request := range requests {
		if containsContentSubstring(request, content) {
			return true
		}
	}
	return false
}

func countContent(messages []providers.ChatMessage, content string) int {
	count := 0
	for _, message := range messages {
		if message.Content == content {
			count++
		}
	}
	return count
}

func containsToolCall(messages []providers.ChatMessage, id string, name string) bool {
	for _, message := range messages {
		for _, call := range message.ToolCalls {
			if call.ID == id && call.Function.Name == name {
				return true
			}
		}
	}
	return false
}

func containsToolResult(messages []providers.ChatMessage, id string, content string) bool {
	for _, message := range messages {
		if message.Role == RoleTool && message.ToolCallID == id && message.Content == content {
			return true
		}
	}
	return false
}

func findTaskRecord(t *testing.T, service *tasks.Service, principal tasks.PrincipalContext, name string) tasks.TaskRecord {
	t.Helper()
	records, err := service.ListTaskRecords(context.Background(), principal)
	if err != nil {
		t.Fatalf("list task records: %v", err)
	}
	for _, record := range records {
		if record.Task.Name == name {
			return record
		}
	}
	t.Fatalf("task %q not found", name)
	return tasks.TaskRecord{}
}

type fakeAgentResolver struct{}

func (fakeAgentResolver) GetChatAgent(ctx context.Context, workspaceID string, agentID string) (AgentContext, error) {
	return AgentContext{
		ID:                    agentID,
		SystemPrompt:          "Use the selected memories.",
		MemoryAccessMode:      "pinned_only",
		ToolPermissionDefault: "ask",
		MaxToolIterations:     4,
		Temperature:           0.7,
		Active:                true,
	}, nil
}

type fakeMemoryProvider struct {
	snippets      []MemorySnippet
	recordedRunID string
	recorded      []MemorySnippet
}

type fakeToolProvider struct {
	permission string
	executions *int
}

func (f fakeToolProvider) RuntimeTools(ctx context.Context, request ToolExposureRequest) ([]RuntimeTool, error) {
	mode := f.permission
	if mode == "" {
		mode = ToolPermissionAllow
	}
	return []RuntimeTool{{
		ID:             "",
		ServerID:       "server_1",
		Name:           "lookup_status",
		ProviderName:   "lookup_status",
		Description:    "Look up service status.",
		InputSchema:    `{"type":"object","properties":{"service":{"type":"string"}}}`,
		PermissionMode: mode,
	}}, nil
}

func (f fakeToolProvider) ExecuteRuntimeTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error) {
	if request.ProviderName != "lookup_status" || request.Arguments != `{"service":"api"}` {
		return ToolExecutionResult{}, fmt.Errorf("unexpected tool call %s %s", request.ProviderName, request.Arguments)
	}
	if f.executions != nil {
		*f.executions++
	}
	return ToolExecutionResult{Content: "api is healthy"}, nil
}

func (fakeToolProvider) SetAgentToolPermission(ctx context.Context, workspaceID string, agentID string, toolID string, mode string) error {
	return nil
}

func (fakeToolProvider) DisableTool(ctx context.Context, workspaceID string, toolID string) error {
	return nil
}

func containsEvent(events []string, expected string) bool {
	for _, event := range events {
		if event == expected {
			return true
		}
	}
	return false
}

func (f *fakeMemoryProvider) SelectForRun(ctx context.Context, request MemoryRequest) ([]MemorySnippet, error) {
	return f.snippets, nil
}

func (f *fakeMemoryProvider) RecordRunMemories(ctx context.Context, runID string, memories []MemorySnippet) error {
	f.recordedRunID = runID
	f.recorded = memories
	return nil
}

func (f *fakeMemoryProvider) UsedByRun(ctx context.Context, runID string) ([]MemorySnippet, error) {
	return f.recorded, nil
}

func seedChatAgent(t *testing.T, ctx context.Context, store *database.Store, workspaceID string, providerID string) string {
	t.Helper()
	agentID := "agent_test_tool_" + strings.ReplaceAll(providerID, "-", "_")
	now := store.NowArg(time.Now().UTC())
	if _, err := store.DB.ExecContext(ctx, `INSERT INTO agents (id, workspace_id, name, system_prompt, default_provider_id, default_model, max_tool_iterations, memory_access_mode, tool_permission_default, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agentID, workspaceID, "Tool Agent", "Use tools when helpful.", providerID, "mock-model", 4, "pinned_only", "ask", true, now, now); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	return agentID
}

func newChatTestContext(t *testing.T) (config.Config, *database.Store, auth.User, func()) {
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
