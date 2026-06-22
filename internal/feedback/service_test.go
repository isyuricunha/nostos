package feedback

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/isyuricunha/nostos/internal/auth"
	"github.com/isyuricunha/nostos/internal/chat"
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
)

func TestFeedbackStorageChangeRemovalAndStats(t *testing.T) {
	ctx := context.Background()
	_, store, user, cleanup := newFeedbackTestContext(t)
	defer cleanup()
	chatRepo := chat.NewSQLRepository(store)
	conversation, err := chatRepo.CreateConversation(ctx, chat.Conversation{WorkspaceID: user.WorkspaceID, OwnerUserID: user.ID, Title: "Feedback"})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	message, err := chatRepo.CreateMessage(ctx, chat.Message{ConversationID: conversation.ID, Role: chat.RoleAssistant, Content: "Draft answer", Model: "mock-model"})
	if err != nil {
		t.Fatalf("create assistant message: %v", err)
	}

	service := NewService(NewSQLRepository(store))
	principal := PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID}
	positive, err := service.Upsert(ctx, principal, message.ID, FeedbackInput{Rating: RatingPositive})
	if err != nil {
		t.Fatalf("store positive feedback: %v", err)
	}
	if positive.Reason != "" || positive.Model != "mock-model" {
		t.Fatalf("unexpected positive feedback: %#v", positive)
	}
	negative, err := service.Upsert(ctx, principal, message.ID, FeedbackInput{
		Rating:  RatingNegative,
		Reason:  "Invented information",
		Comment: "The source did not say this.",
	})
	if err != nil {
		t.Fatalf("store negative feedback: %v", err)
	}
	if negative.Rating != RatingNegative || negative.Reason != "Invented information" {
		t.Fatalf("unexpected negative feedback: %#v", negative)
	}
	items, err := service.ListForConversation(ctx, principal, conversation.ID)
	if err != nil {
		t.Fatalf("list feedback: %v", err)
	}
	if len(items) != 1 || items[0].Rating != RatingNegative {
		t.Fatalf("unexpected feedback list: %#v", items)
	}
	stats, err := service.Stats(ctx, principal)
	if err != nil {
		t.Fatalf("feedback stats: %v", err)
	}
	if stats.Positive != 0 || stats.Negative != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if err := service.Delete(ctx, principal, message.ID); err != nil {
		t.Fatalf("delete feedback: %v", err)
	}
	items, err = service.ListForConversation(ctx, principal, conversation.ID)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("feedback was not deleted: %#v", items)
	}
}

func TestFeedbackRejectsInvalidNegativeReason(t *testing.T) {
	_, err := normalizeInput(FeedbackInput{Rating: RatingNegative, Reason: "Bad"})
	if err == nil {
		t.Fatal("expected invalid reason to be rejected")
	}
}

func newFeedbackTestContext(t *testing.T) (config.Config, *database.Store, auth.User, func()) {
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
