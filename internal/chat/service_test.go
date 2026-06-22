package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/isyuricunha/nostos/internal/auth"
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
	"github.com/isyuricunha/nostos/internal/providers"
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
	service := NewService(cfg, NewSQLRepository(store), providerService, providerClient, fakeAgentResolver{}, &fakeMemoryProvider{}).WithToolProvider(fakeToolProvider{})
	conversation, err := service.CreateConversation(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, Conversation{
		Title:      "Tool chat",
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
	if len(messages) != 2 || messages[1].Content != "The API is healthy." {
		t.Fatalf("assistant follow-up was not persisted: %#v", messages)
	}
	if !containsEvent(events, "tool_result") {
		t.Fatalf("tool result event was not emitted: %#v", events)
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

type fakeAgentResolver struct{}

func (fakeAgentResolver) GetChatAgent(ctx context.Context, workspaceID string, agentID string) (AgentContext, error) {
	return AgentContext{
		ID:               agentID,
		SystemPrompt:     "Use the selected memories.",
		MemoryAccessMode: "pinned_only",
	}, nil
}

type fakeMemoryProvider struct {
	snippets      []MemorySnippet
	recordedRunID string
	recorded      []MemorySnippet
}

type fakeToolProvider struct{}

func (fakeToolProvider) AllowedChatTools(ctx context.Context, workspaceID string) ([]providers.ChatTool, error) {
	return []providers.ChatTool{{
		Type: "function",
		Function: providers.ChatToolFunction{
			Name:        "lookup_status",
			Description: "Look up service status.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"service":{"type":"string"}}}`),
		},
	}}, nil
}

func (fakeToolProvider) ExecuteAllowedTool(ctx context.Context, workspaceID string, name string, arguments string) (string, error) {
	if name != "lookup_status" || arguments != `{"service":"api"}` {
		return "", fmt.Errorf("unexpected tool call %s %s", name, arguments)
	}
	return "api is healthy", nil
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
