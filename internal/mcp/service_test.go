package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/isyuricunha/nostos/internal/auth"
	"github.com/isyuricunha/nostos/internal/chat"
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
)

func TestHTTPDiscoveryEncryptsHeadersAndStoresTools(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization header was not decrypted")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "tools-list",
			"result": map[string]any{
				"tools": []map[string]any{{
					"name":        "lookup_memory",
					"description": "Looks up a memory.",
					"inputSchema": map[string]any{"type": "object"},
				}},
			},
		})
	}))
	defer server.Close()

	cfg, store, user, cleanup := newMCPTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store), NewClient())
	created, err := service.CreateServer(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ServerInput{
		Name:             "Mock MCP",
		TransportType:    "http",
		HTTPURL:          server.URL,
		HTTPHeaders:      map[string]string{"Authorization": "Bearer test-token"},
		Enabled:          true,
		RequestTimeoutMS: 5000,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	var encrypted sql.NullString
	query := "SELECT encrypted_http_headers FROM mcp_servers WHERE id = " + store.Placeholder(1)
	if err := store.DB.QueryRowContext(ctx, query, created.ID).Scan(&encrypted); err != nil {
		t.Fatalf("read encrypted headers: %v", err)
	}
	if !encrypted.Valid || encrypted.String == `{"Authorization":"Bearer test-token"}` {
		t.Fatal("MCP header secret was not encrypted")
	}
	tools, err := service.DiscoverTools(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, created.ID)
	if err != nil {
		t.Fatalf("discover tools: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "lookup_memory" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
	if err := service.UpdateToolPermission(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, tools[0].ID, "allow"); err != nil {
		t.Fatalf("update permission: %v", err)
	}
	listed, err := service.ListTools(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, created.ID)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if listed[0].PermissionMode != "allow" {
		t.Fatalf("permission was not updated: %#v", listed[0])
	}
}

func TestRuntimeToolsEnforceAgentAssignmentAndPermissions(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "tools-list",
			"result": map[string]any{"tools": []map[string]any{{
				"name":        "lookup_status",
				"description": "Looks up service status.",
				"inputSchema": map[string]any{"type": "object"},
			}}},
		})
	}))
	defer server.Close()

	cfg, store, user, cleanup := newMCPTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store), NewClient())
	mcpServer, err := service.CreateServer(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ServerInput{
		Name:             "Status",
		TransportType:    "http",
		HTTPURL:          server.URL,
		Enabled:          true,
		RequestTimeoutMS: 5000,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	tools, err := service.DiscoverTools(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, mcpServer.ID)
	if err != nil {
		t.Fatalf("discover tools: %v", err)
	}
	agentID := seedMCPAgent(t, ctx, store, user.WorkspaceID)
	unassigned, err := service.RuntimeTools(ctx, chat.ToolExposureRequest{
		WorkspaceID:            user.WorkspaceID,
		AgentID:                agentID,
		ConversationID:         "conversation_1",
		AgentDefaultPermission: chat.ToolPermissionAsk,
	})
	if err != nil {
		t.Fatalf("runtime tools unassigned: %v", err)
	}
	if len(unassigned) != 0 {
		t.Fatalf("unassigned server tools were exposed: %#v", unassigned)
	}
	if err := service.AssignAgentServers(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, agentID, []string{mcpServer.ID}); err != nil {
		t.Fatalf("assign server: %v", err)
	}
	assigned, err := service.RuntimeTools(ctx, chat.ToolExposureRequest{
		WorkspaceID:            user.WorkspaceID,
		AgentID:                agentID,
		ConversationID:         "conversation_1",
		AgentDefaultPermission: chat.ToolPermissionAsk,
	})
	if err != nil {
		t.Fatalf("runtime tools assigned: %v", err)
	}
	if len(assigned) != 1 || assigned[0].PermissionMode != chat.ToolPermissionAsk || assigned[0].ProviderName == tools[0].Name {
		t.Fatalf("assigned ask tool did not use stable provider mapping: %#v", assigned)
	}
	if err := service.SetAgentToolPermission(ctx, user.WorkspaceID, agentID, tools[0].ID, chat.ToolPermissionDeny); err != nil {
		t.Fatalf("set agent deny: %v", err)
	}
	denied, err := service.RuntimeTools(ctx, chat.ToolExposureRequest{WorkspaceID: user.WorkspaceID, AgentID: agentID, ConversationID: "conversation_1", AgentDefaultPermission: chat.ToolPermissionAsk})
	if err != nil {
		t.Fatalf("runtime tools denied: %v", err)
	}
	if len(denied) != 0 {
		t.Fatalf("agent denied tool was exposed: %#v", denied)
	}
	if err := service.SetAgentToolPermission(ctx, user.WorkspaceID, agentID, tools[0].ID, chat.ToolPermissionAllow); err != nil {
		t.Fatalf("set agent allow: %v", err)
	}
	allowed, err := service.RuntimeTools(ctx, chat.ToolExposureRequest{WorkspaceID: user.WorkspaceID, AgentID: agentID, ConversationID: "conversation_1", AgentDefaultPermission: chat.ToolPermissionAsk})
	if err != nil {
		t.Fatalf("runtime tools allowed: %v", err)
	}
	if len(allowed) != 1 || allowed[0].PermissionMode != chat.ToolPermissionAllow {
		t.Fatalf("agent allow was not effective: %#v", allowed)
	}
	if err := service.UpdateToolPermission(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, tools[0].ID, chat.ToolPermissionDeny); err != nil {
		t.Fatalf("set global deny: %v", err)
	}
	globalDenied, err := service.RuntimeTools(ctx, chat.ToolExposureRequest{WorkspaceID: user.WorkspaceID, AgentID: agentID, ConversationID: "conversation_1", AgentDefaultPermission: chat.ToolPermissionAllow})
	if err != nil {
		t.Fatalf("runtime tools global denied: %v", err)
	}
	if len(globalDenied) != 0 {
		t.Fatalf("global denied tool was exposed: %#v", globalDenied)
	}
}

func newMCPTestContext(t *testing.T) (config.Config, *database.Store, auth.User, func()) {
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
		Database:      config.DatabaseConfig{Driver: "sqlite", URL: filepath.Join(dir, "test.db")},
		Security:      config.SecurityConfig{EncryptionKey: key, SessionSecret: "test-session-secret-with-enough-length", SessionTTL: time.Hour},
	}
	store, err := database.Open(ctx, cfg.Database)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.RunMigrations(ctx, store, cfg.MigrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	authService := auth.NewService(auth.NewSQLRepository(store), cfg)
	user, err := authService.CreateOwner(ctx, auth.SetupInput{Email: "owner@example.com", Password: "very-secure-password", ConfirmPassword: "very-secure-password"})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	return cfg, store, user, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}
}

func seedMCPAgent(t *testing.T, ctx context.Context, store *database.Store, workspaceID string) string {
	t.Helper()
	agentID := "agent_mcp_test"
	now := store.NowArg(time.Now().UTC())
	if _, err := store.DB.ExecContext(ctx, `INSERT INTO agents (id, workspace_id, name, system_prompt, max_tool_iterations, memory_access_mode, tool_permission_default, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agentID, workspaceID, "MCP Agent", "Use assigned tools.", 4, "pinned_only", "ask", true, now, now); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	return agentID
}
