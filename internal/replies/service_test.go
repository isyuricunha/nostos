package replies

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
	"github.com/yuricunha/nostos/internal/chat"
	"github.com/yuricunha/nostos/internal/config"
	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/providers"
)

func TestDefaultPresetsAndDraftGeneration(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":"Not really."}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: {"choices":[{"delta":{"content":" I need more time."}}]}`)
			fmt.Fprintln(w)
			fmt.Fprintln(w, `data: [DONE]`)
		case "/v1/models":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "mock-model"}}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg, store, user, cleanup := newReplyTestContext(t)
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
	chatRepo := chat.NewSQLRepository(store)
	conversation, err := chatRepo.CreateConversation(ctx, chat.Conversation{WorkspaceID: user.WorkspaceID, OwnerUserID: user.ID, Title: "Replies"})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	source, err := chatRepo.CreateMessage(ctx, chat.Message{
		ConversationID: conversation.ID,
		Role:           chat.RoleUser,
		Content:        "Are you okay?",
		ProviderID:     provider.ID,
		Model:          "mock-model",
	})
	if err != nil {
		t.Fatalf("create source message: %v", err)
	}

	service := NewService(cfg, NewSQLRepository(store), providerService, client)
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	if err := service.EnsureDefaultPresets(ctx); err != nil {
		t.Fatalf("ensure presets: %v", err)
	}
	presets, err := service.ListPresets(ctx, principal)
	if err != nil {
		t.Fatalf("list presets: %v", err)
	}
	if len(presets) < 11 {
		t.Fatalf("expected default presets, got %d", len(presets))
	}
	var negative Preset
	for _, preset := range presets {
		if preset.Name == "Negative" {
			negative = preset
		}
	}
	if negative.ID == "" {
		t.Fatal("negative preset was not created")
	}
	draft, err := service.GenerateDraft(ctx, principal, DraftInput{
		SourceMessageID:   source.ID,
		PresetID:          negative.ID,
		CustomInstruction: "Keep it honest.",
		ProviderID:        provider.ID,
		Model:             "mock-model",
	})
	if err != nil {
		t.Fatalf("generate draft: %v", err)
	}
	if draft.GeneratedDraft != "Not really. I need more time." || draft.PresetName != "Negative" {
		t.Fatalf("unexpected draft: %#v", draft)
	}
	drafts, err := service.ListDrafts(ctx, principal, source.ID)
	if err != nil {
		t.Fatalf("list drafts: %v", err)
	}
	if len(drafts) != 1 || drafts[0].GeneratedDraft != draft.GeneratedDraft {
		t.Fatalf("draft was not persisted: %#v", drafts)
	}
}

func TestNormalizePresetRequiresInstruction(t *testing.T) {
	_, err := normalizePreset("workspace", PresetInput{Name: "Custom"})
	if err == nil {
		t.Fatal("expected missing instruction to be rejected")
	}
}

func newReplyTestContext(t *testing.T) (config.Config, *database.Store, auth.User, func()) {
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
