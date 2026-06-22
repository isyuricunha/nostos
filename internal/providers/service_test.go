package providers

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
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
)

func TestProviderCreateEncryptsSecretAndRefreshesModels(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Fatalf("authorization header was not set")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "mock-model"}},
		})
	}))
	defer server.Close()

	cfg, store, user, cleanup := newProviderTestContext(t)
	defer cleanup()
	authRepo := auth.NewSQLRepository(store)
	service := NewService(cfg, NewSQLRepository(store), authRepo, NewOpenAIClient())
	apiKey := "test-api-key"

	provider, err := service.Create(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ProviderInput{
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
	if provider.APIKeyEnvRef != "" {
		t.Fatal("raw API key should not be exposed")
	}
	var encrypted sql.NullString
	query := "SELECT encrypted_api_key FROM providers WHERE id = " + store.Placeholder(1)
	if err := store.DB.QueryRowContext(ctx, query, provider.ID).Scan(&encrypted); err != nil {
		t.Fatalf("read encrypted key: %v", err)
	}
	if !encrypted.Valid || encrypted.String == apiKey {
		t.Fatal("provider API key was not encrypted")
	}

	models, err := service.RefreshModels(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, provider.ID)
	if err != nil {
		t.Fatalf("refresh models: %v", err)
	}
	if len(models) != 1 || models[0].ModelID != "mock-model" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func newProviderTestContext(t *testing.T) (config.Config, *database.Store, auth.User, func()) {
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
