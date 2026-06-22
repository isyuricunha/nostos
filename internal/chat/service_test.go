package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuricunha/nostos/internal/auth"
	"github.com/yuricunha/nostos/internal/config"
	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/providers"
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
	service := NewService(cfg, repo, providerService, providerClient)
	conversation, err := service.CreateConversation(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, Conversation{
		Title:      "Mock chat",
		ProviderID: provider.ID,
		Model:      "mock-model",
	})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	var events []string
	err = service.Run(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}, conversation.ID, RunInput{Content: "Say hello"}, func(event string, payload any) error {
		events = append(events, event)
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
