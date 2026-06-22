package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isyuricunha/nostos/internal/agents"
	"github.com/isyuricunha/nostos/internal/api"
	"github.com/isyuricunha/nostos/internal/auth"
	"github.com/isyuricunha/nostos/internal/chat"
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
	"github.com/isyuricunha/nostos/internal/feedback"
	"github.com/isyuricunha/nostos/internal/health"
	"github.com/isyuricunha/nostos/internal/mcp"
	"github.com/isyuricunha/nostos/internal/memory"
	"github.com/isyuricunha/nostos/internal/providers"
	"github.com/isyuricunha/nostos/internal/replies"
	"github.com/isyuricunha/nostos/internal/tasks"
)

func TestOwnerWorkspaceFlow(t *testing.T) {
	ctx := context.Background()
	mockProvider := newMockProvider(t)
	defer mockProvider.Close()
	mockMCP := newMockMCP(t)
	defer mockMCP.Close()

	cfg, store, cleanup := newE2EStore(t)
	defer cleanup()
	authRepo := auth.NewSQLRepository(store)
	authService := auth.NewService(authRepo, cfg)
	providerClient := providers.NewOpenAIClient()
	providerService := providers.NewService(cfg, providers.NewSQLRepository(store), authRepo, providerClient)
	agentService := agents.NewService(agents.NewSQLRepository(store))
	memoryService := memory.NewService(memory.NewSQLRepository(store))
	mcpService := mcp.NewService(cfg, mcp.NewSQLRepository(store), authRepo, mcp.NewClient())
	taskService := tasks.NewService(cfg, tasks.NewSQLRepository(store), authRepo).WithProviderClient(providerService, providerClient)
	chatService := chat.NewService(cfg, chat.NewSQLRepository(store), providerService, providerClient, agentService, memoryService).WithToolProvider(mcpService)
	replyService := replies.NewService(cfg, replies.NewSQLRepository(store), providerService, providerClient)
	router := api.NewRouter(api.RouterDeps{
		Config:    cfg,
		Health:    health.NewService(store, "0.1.0-test", "test", "test"),
		Auth:      api.AuthDeps{Config: cfg, Auth: authService},
		Providers: providerService,
		Chat:      chatService,
		Agents:    agentService,
		Memories:  memoryService,
		MCP:       mcpService,
		Tasks:     taskService,
		Feedback:  feedback.NewService(feedback.NewSQLRepository(store)),
		Replies:   replyService,
	})
	server := httptest.NewServer(router)
	defer server.Close()
	client := newAPIClient(t)

	var setup struct {
		User struct {
			ID          string `json:"id"`
			WorkspaceID string `json:"workspace_id"`
		} `json:"user"`
	}
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/setup", map[string]any{
		"email":            "owner@example.com",
		"display_name":     "Owner",
		"password":         "very-secure-password",
		"confirm_password": "very-secure-password",
	}, &setup)
	if setup.User.WorkspaceID == "" {
		t.Fatal("setup did not create a workspace")
	}
	if err := agentService.EnsureDefaultAgents(ctx); err != nil {
		t.Fatalf("ensure agents: %v", err)
	}
	if err := taskService.EnsureSystemTasks(ctx); err != nil {
		t.Fatalf("ensure tasks: %v", err)
	}
	if err := replyService.EnsureDefaultPresets(ctx); err != nil {
		t.Fatalf("ensure presets: %v", err)
	}

	var providerResp struct {
		Provider providers.Provider `json:"provider"`
	}
	apiKey := "test-api-key"
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/providers", map[string]any{
		"name":               "Mock",
		"base_url":           mockProvider.URL,
		"api_key":            apiKey,
		"enabled":            true,
		"request_timeout_ms": 5000,
		"default_model":      "mock-model",
	}, &providerResp)

	var agentResp struct {
		Agent agents.Agent `json:"agent"`
	}
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/agents", map[string]any{
		"name":                    "E2E Agent",
		"description":             "",
		"avatar":                  "sparkles",
		"system_prompt":           "Be concise.",
		"default_provider_id":     providerResp.Provider.ID,
		"default_model":           "mock-model",
		"temperature":             0.7,
		"max_tool_iterations":     4,
		"memory_access_mode":      "pinned_only",
		"tool_permission_default": "ask",
		"active":                  true,
	}, &agentResp)

	var conversationResp struct {
		Conversation chat.Conversation `json:"conversation"`
	}
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/conversations", map[string]any{
		"title":       "E2E Chat",
		"agent_id":    agentResp.Agent.ID,
		"provider_id": providerResp.Provider.ID,
		"model":       "mock-model",
	}, &conversationResp)

	var memoryResp struct {
		Memory memory.Memory `json:"memory"`
	}
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/memories", map[string]any{
		"title":      "Greeting",
		"content":    "Prefer concise greetings.",
		"tags":       []string{"style"},
		"scope":      "global",
		"importance": 80,
		"pinned":     true,
		"active":     true,
		"source":     "manual",
	}, &memoryResp)

	events := requestStream(t, client, server.URL+"/api/v1/conversations/"+conversationResp.Conversation.ID+"/runs", map[string]any{
		"content":     "Say hello",
		"provider_id": providerResp.Provider.ID,
		"model":       "mock-model",
	})
	if !contains(events, "run_completed") || !contains(events, "memories_used") {
		t.Fatalf("chat run did not complete with memory injection: %#v", events)
	}

	var messages struct {
		Messages []chat.Message `json:"messages"`
	}
	requestJSON(t, client, http.MethodGet, server.URL+"/api/v1/conversations/"+conversationResp.Conversation.ID+"/messages", nil, &messages)
	if len(messages.Messages) != 2 || messages.Messages[1].Content == "" {
		t.Fatalf("conversation messages were not persisted: %#v", messages.Messages)
	}

	requestJSON(t, client, http.MethodPut, server.URL+"/api/v1/messages/"+messages.Messages[1].ID+"/feedback", map[string]any{"rating": "positive"}, nil)
	requestJSON(t, client, http.MethodPut, server.URL+"/api/v1/messages/"+messages.Messages[1].ID+"/feedback", map[string]any{
		"rating": "negative",
		"reason": "Invented information",
	}, nil)
	regenEvents := requestStream(t, client, server.URL+"/api/v1/messages/"+messages.Messages[1].ID+"/regenerate", map[string]any{
		"provider_id":              providerResp.Provider.ID,
		"model":                    "mock-model",
		"regeneration_instruction": "Address negative feedback.",
	})
	if !contains(regenEvents, "run_completed") {
		t.Fatalf("regeneration did not complete: %#v", regenEvents)
	}

	var presets struct {
		Presets []replies.Preset `json:"presets"`
	}
	requestJSON(t, client, http.MethodGet, server.URL+"/api/v1/reply-presets", nil, &presets)
	var negativePreset string
	for _, preset := range presets.Presets {
		if preset.Name == "Negative" {
			negativePreset = preset.ID
		}
	}
	if negativePreset == "" {
		t.Fatal("negative reply preset was not available")
	}
	var draftResp struct {
		Draft replies.Draft `json:"draft"`
	}
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/reply-drafts", map[string]any{
		"source_message_id": messages.Messages[0].ID,
		"preset_id":         negativePreset,
		"provider_id":       providerResp.Provider.ID,
		"model":             "mock-model",
	}, &draftResp)
	if draftResp.Draft.GeneratedDraft == "" {
		t.Fatal("reply draft was empty")
	}

	var taskResp struct {
		Task tasks.Task `json:"task"`
	}
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/tasks", map[string]any{
		"name":               "Manual E2E Task",
		"task_type":          "agent",
		"state":              "enabled",
		"provider_id":        providerResp.Provider.ID,
		"model":              "mock-model",
		"prompt":             "Run task",
		"tool_policy":        "use_preapproved_tools_only",
		"max_retries":        1,
		"timeout_ms":         30000,
		"concurrency_policy": "skip",
		"schedule_mode":      "manual",
	}, &taskResp)
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/tasks/"+taskResp.Task.ID+"/run", nil, nil)
	if err := taskService.ClaimAndExecute(ctx, "e2e-worker"); err != nil {
		t.Fatalf("execute task: %v", err)
	}

	var mcpResp struct {
		Server mcp.Server `json:"server"`
	}
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/mcp-servers", map[string]any{
		"name":               "Mock MCP",
		"description":        "",
		"transport_type":     "http",
		"http_url":           mockMCP.URL,
		"http_headers":       map[string]string{},
		"environment":        map[string]string{},
		"enabled":            true,
		"startup_timeout_ms": 10000,
		"request_timeout_ms": 30000,
	}, &mcpResp)
	var toolsResp struct {
		Tools []mcp.Tool `json:"tools"`
	}
	requestJSON(t, client, http.MethodPost, server.URL+"/api/v1/mcp-servers/"+mcpResp.Server.ID+"/discover", nil, &toolsResp)
	if len(toolsResp.Tools) != 1 {
		t.Fatalf("tool discovery failed: %#v", toolsResp.Tools)
	}
	requestJSON(t, client, http.MethodPut, server.URL+"/api/v1/mcp-tools/"+toolsResp.Tools[0].ID+"/permission", map[string]any{"permission_mode": "allow"}, nil)
	toolEvents := requestStream(t, client, server.URL+"/api/v1/conversations/"+conversationResp.Conversation.ID+"/runs", map[string]any{
		"content":     "Use the status tool",
		"provider_id": providerResp.Provider.ID,
		"model":       "mock-model",
	})
	if !contains(toolEvents, "tool_result") || !contains(toolEvents, "run_completed") {
		t.Fatalf("tool-assisted chat did not complete: %#v", toolEvents)
	}
}

func newE2EStore(t *testing.T) (config.Config, *database.Store, func()) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cfg := config.Config{
		AppEnv:        "development",
		BaseURL:       "http://localhost",
		Timezone:      "UTC",
		MigrationsDir: filepath.Join("..", "..", "migrations"),
		Database:      config.DatabaseConfig{Driver: "sqlite", URL: filepath.Join(dir, "e2e.db")},
		Security: config.SecurityConfig{
			EncryptionKey: key,
			SessionSecret: "test-session-secret-with-enough-length",
			SessionTTL:    time.Hour,
		},
		Tasks: config.TaskConfig{DefaultTimeout: time.Minute, MaxRetries: 1},
		Chat:  config.ChatConfig{DefaultTimeout: time.Minute, RecentMessageLimit: 30, MaxToolIterations: 4},
	}
	store, err := database.Open(ctx, cfg.Database)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.RunMigrations(ctx, store, cfg.MigrationsDir); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return cfg, store, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}
}

func newAPIClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	return &http.Client{Jar: jar}
}

func requestJSON(t *testing.T, client *http.Client, method string, url string, body any, target any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode request: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	addCSRF(req, client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send request %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("request %s %s returned %d: %s", method, url, resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}

func requestStream(t *testing.T, client *http.Client, url string, body any) []string {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode stream request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("create stream request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	addCSRF(req, client)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send stream request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("stream returned %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var events []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "event:") {
			events = append(events, strings.TrimSpace(strings.TrimPrefix(line, "event:")))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	return events
}

func addCSRF(req *http.Request, client *http.Client) {
	if client.Jar == nil || req.URL == nil {
		return
	}
	for _, cookie := range client.Jar.Cookies(req.URL) {
		if cookie.Name == auth.CSRFCookieName {
			req.Header.Set(auth.CSRFHeaderName, cookie.Value)
		}
	}
}

func newMockProvider(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "mock-model"}}})
		case "/v1/chat/completions":
			var body struct {
				Messages []providers.ChatMessage `json:"messages"`
				Tools    []providers.ChatTool    `json:"tools"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode provider request: %v", err)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			lastContent := ""
			if len(body.Messages) > 0 {
				lastContent = body.Messages[len(body.Messages)-1].Content
			}
			if len(body.Tools) > 0 && strings.Contains(lastContent, "status tool") {
				fmt.Fprintln(w, `data: {"choices":[{"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup_status","arguments":"{\"service\":\"api\"}"}}]},"finish_reason":"tool_calls"}]}`)
				fmt.Fprintln(w)
				fmt.Fprintln(w, `data: [DONE]`)
				return
			}
			if lastContent == "api is healthy" {
				fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Tool says the API is healthy."}}]}`)
			} else if strings.Contains(lastContent, "Run task") {
				fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Task completed."}}]}`)
			} else if strings.Contains(lastContent, "Reply intent") {
				fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Not today."}}]}`)
			} else if strings.Contains(lastContent, "Regeneration instruction") {
				fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Regenerated answer."}}]}`)
			} else {
				fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Hello from the mock provider."}}]}`)
			}
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: [DONE]`)
		default:
			t.Fatalf("unexpected provider path %s", r.URL.Path)
		}
	}))
}

func newMockMCP(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode MCP request: %v", err)
		}
		switch payload.Method {
		case "tools/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "tools-list",
				"result": map[string]any{"tools": []map[string]any{{
					"name":        "lookup_status",
					"description": "Look up service status.",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"service": map[string]string{"type": "string"}}},
				}}},
			})
		case "tools/call":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      "tools-call",
				"result":  map[string]any{"content": []map[string]string{{"type": "text", "text": "api is healthy"}}},
			})
		default:
			t.Fatalf("unexpected MCP method %s", payload.Method)
		}
	}))
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
