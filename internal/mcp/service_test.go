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

	"github.com/yuricunha/nostos/internal/auth"
	"github.com/yuricunha/nostos/internal/config"
	"github.com/yuricunha/nostos/internal/database"
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
