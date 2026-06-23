package providers

import (
	"context"
	"database/sql"
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

func TestModelCatalogRefreshPreservesFullIDsAndUnavailableModels(t *testing.T) {
	ctx := context.Background()
	modelIDs := make([]string, 0, 800)
	modelIDs = append(modelIDs,
		"NVIDIA NIM/openai/gpt-oss-120b",
		"NVIDIA NIM/moonshotai/kimi-k2.6",
		"Bifrost/opencode-proxy/deepseek-v4-flash-free",
	)
	for index := 3; index < 800; index++ {
		modelIDs = append(modelIDs, "Bifrost/catalog/model-"+time.Now().UTC().Format("20060102")+"-"+string(rune('a'+index%26))+json.Number(fmt.Sprintf("%03d", index)).String())
	}
	activeIDs := modelIDs
	failModels := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if failModels {
			http.Error(w, "catalog unavailable", http.StatusBadGateway)
			return
		}
		data := make([]map[string]string, 0, len(activeIDs))
		for _, modelID := range activeIDs {
			data = append(data, map[string]string{"id": modelID})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()

	cfg, store, user, cleanup := newProviderTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store), NewOpenAIClient())
	apiKey := "test-api-key"
	provider, err := service.Create(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ProviderInput{
		Name:             "Large Bifrost",
		BaseURL:          server.URL,
		APIKey:           &apiKey,
		Enabled:          true,
		RequestTimeoutMS: 5000,
		DefaultModel:     "NVIDIA NIM/openai/gpt-oss-120b",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	models, err := service.RefreshModels(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, provider.ID)
	if err != nil {
		t.Fatalf("refresh large catalog: %v", err)
	}
	if len(models) != 800 {
		t.Fatalf("expected 800 cached models, got %d", len(models))
	}
	foundFullID := false
	for _, model := range models {
		if model.ModelID == "NVIDIA NIM/moonshotai/kimi-k2.6" && model.Available && model.ProviderID == provider.ID {
			foundFullID = true
		}
	}
	if !foundFullID {
		t.Fatal("full provider model ID was not preserved")
	}

	activeIDs = modelIDs[:10]
	if _, err := service.RefreshModels(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, provider.ID); err != nil {
		t.Fatalf("refresh reduced catalog: %v", err)
	}
	allModels, err := service.ListCatalogModels(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ModelQuery{Limit: 1000, IncludeUnavailable: true})
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	missingMarkedUnavailable := false
	for _, model := range allModels {
		if model.ModelID == modelIDs[50] && !model.Available {
			missingMarkedUnavailable = true
		}
	}
	if !missingMarkedUnavailable {
		t.Fatal("missing API model was not marked unavailable")
	}

	failModels = true
	if _, err := service.RefreshModels(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, provider.ID); err == nil {
		t.Fatal("expected refresh failure")
	}
	afterFailure, err := service.ListCatalogModels(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ModelQuery{Limit: 1000, IncludeUnavailable: true})
	if err != nil {
		t.Fatalf("list after failure: %v", err)
	}
	if len(afterFailure) != len(allModels) {
		t.Fatalf("failed refresh should preserve cache; before=%d after=%d", len(allModels), len(afterFailure))
	}
}

func TestModelRolesResolveProviderScopedFallbacks(t *testing.T) {
	ctx := context.Background()
	cfg, store, user, cleanup := newProviderTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store), NewOpenAIClient())
	apiKey := "test-api-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "Bifrost/opencode-proxy/deepseek-v4-flash-free"}}})
	}))
	defer server.Close()
	provider, err := service.Create(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ProviderInput{
		Name:             "Bifrost",
		BaseURL:          server.URL,
		APIKey:           &apiKey,
		Enabled:          true,
		RequestTimeoutMS: 5000,
		DefaultModel:     "legacy-default",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if _, err := service.CreateManualModel(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ModelInput{
		ProviderID:       provider.ID,
		ModelID:          "Bifrost/opencode-proxy/deepseek-v4-flash-free",
		Enabled:          true,
		Available:        true,
		Capabilities:     []string{"chat"},
		CapabilitySource: "manual",
	}); err != nil {
		t.Fatalf("create manual model: %v", err)
	}
	if _, err := service.SetModelRole(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ModelRoleUtility, ModelRoleInput{
		Models: []ModelRoleReference{{ProviderID: provider.ID, ModelID: "Bifrost/opencode-proxy/deepseek-v4-flash-free"}},
	}); err != nil {
		t.Fatalf("set utility role: %v", err)
	}
	resolution, err := service.ResolveModelRole(ctx, user.WorkspaceID, ModelRoleUtility)
	if err != nil {
		t.Fatalf("resolve utility role: %v", err)
	}
	if resolution.Provider.ID != provider.ID || resolution.ModelID != "Bifrost/opencode-proxy/deepseek-v4-flash-free" {
		t.Fatalf("unexpected resolution: %#v", resolution)
	}
}

func TestModelRolesResolveOrderedFallbackChain(t *testing.T) {
	ctx := context.Background()
	cfg, store, user, cleanup := newProviderTestContext(t)
	defer cleanup()
	service := NewService(cfg, NewSQLRepository(store), auth.NewSQLRepository(store), NewOpenAIClient())
	apiKey := "test-api-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "utility-fast"}}})
	}))
	defer server.Close()
	disabled, err := service.Create(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ProviderInput{
		Name:             "Disabled provider",
		BaseURL:          server.URL,
		APIKey:           &apiKey,
		Enabled:          false,
		RequestTimeoutMS: 5000,
		DefaultModel:     "disabled-model",
	})
	if err != nil {
		t.Fatalf("create disabled provider: %v", err)
	}
	enabled, err := service.Create(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ProviderInput{
		Name:             "Enabled provider",
		BaseURL:          server.URL,
		APIKey:           &apiKey,
		Enabled:          true,
		RequestTimeoutMS: 5000,
		DefaultModel:     "utility-fast",
	})
	if err != nil {
		t.Fatalf("create enabled provider: %v", err)
	}
	roles, err := service.SetModelRole(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, ModelRoleUtility, ModelRoleInput{
		Models: []ModelRoleReference{
			{ProviderID: disabled.ID, ModelID: "disabled-model"},
			{ProviderID: enabled.ID, ModelID: "utility-fast"},
		},
	})
	if err != nil {
		t.Fatalf("set utility chain: %v", err)
	}
	if len(roles) != 2 || roles[0].Position != 0 || roles[1].Position != 1 {
		t.Fatalf("roles were not stored in order: %#v", roles)
	}
	resolution, err := service.ResolveModelRole(ctx, user.WorkspaceID, ModelRoleUtility)
	if err != nil {
		t.Fatalf("resolve utility role: %v", err)
	}
	if resolution.Provider.ID != enabled.ID || resolution.ModelID != "utility-fast" {
		t.Fatalf("expected enabled fallback provider, got %#v", resolution)
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
