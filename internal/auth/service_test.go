package auth

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuricunha/nostos/internal/config"
	"github.com/yuricunha/nostos/internal/crypto"
	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/id"
)

func TestOwnerSetupLoginAndSessionList(t *testing.T) {
	ctx := context.Background()
	service, _, cleanup := newTestAuthService(t)
	defer cleanup()

	available, err := service.SetupAvailable(ctx)
	if err != nil {
		t.Fatalf("setup status: %v", err)
	}
	if !available {
		t.Fatal("setup should be available before owner creation")
	}

	user, err := service.CreateOwner(ctx, SetupInput{
		Email:           "owner@example.com",
		DisplayName:     "Owner",
		Password:        "very-secure-password",
		ConfirmPassword: "very-secure-password",
		IPAddress:       "127.0.0.1",
		UserAgent:       "test",
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if user.Role != "owner" || user.WorkspaceID == "" {
		t.Fatalf("unexpected owner: %#v", user)
	}

	available, err = service.SetupAvailable(ctx)
	if err != nil {
		t.Fatalf("setup status after owner: %v", err)
	}
	if available {
		t.Fatal("setup should close after owner creation")
	}

	result, err := service.Login(ctx, LoginInput{
		Email:     "owner@example.com",
		Password:  "very-secure-password",
		IPAddress: "127.0.0.1",
		UserAgent: "test",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if result.Tokens.SessionToken == "" || result.Tokens.CSRFToken == "" {
		t.Fatal("login should return session and csrf tokens")
	}

	principal, err := service.Authenticate(ctx, result.Tokens.SessionToken)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	sessions, err := service.ListSessions(ctx, principal)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one active session, got %d", len(sessions))
	}
}

func TestExpiredSessionFailsAuthentication(t *testing.T) {
	ctx := context.Background()
	service, repo, cleanup := newTestAuthService(t)
	defer cleanup()

	user, err := service.CreateOwner(ctx, SetupInput{
		Email:           "owner@example.com",
		Password:        "very-secure-password",
		ConfirmPassword: "very-secure-password",
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	token := "expired-token"
	now := time.Now().UTC()
	err = repo.CreateSession(ctx, Session{
		ID:            id.New(),
		UserID:        user.ID,
		TokenHash:     crypto.HashToken("test-session-secret-with-enough-length", token),
		CSRFTokenHash: crypto.HashToken("test-session-secret-with-enough-length", "csrf"),
		ExpiresAt:     now.Add(-time.Minute),
		CreatedAt:     now.Add(-time.Hour),
		UpdatedAt:     now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	_, err = service.Authenticate(ctx, token)
	if !errors.Is(err, ErrExpiredSession) {
		t.Fatalf("expected expired session error, got %v", err)
	}
}

func TestDisabledUserLosesAccess(t *testing.T) {
	ctx := context.Background()
	service, repo, cleanup := newTestAuthService(t)
	defer cleanup()

	user, err := service.CreateOwner(ctx, SetupInput{
		Email:           "owner@example.com",
		Password:        "very-secure-password",
		ConfirmPassword: "very-secure-password",
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	tokens, err := service.CreateSessionForUser(ctx, user, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	now := time.Now().UTC()
	sqlRepo := repo.(*SQLRepository)
	query := "UPDATE users SET disabled_at = " + sqlRepo.store.Placeholder(1) + " WHERE id = " + sqlRepo.store.Placeholder(2)
	if _, err := sqlRepo.store.DB.ExecContext(ctx, query, sqlRepo.store.NowArg(now), user.ID); err != nil {
		t.Fatalf("disable user: %v", err)
	}
	_, err = service.Authenticate(ctx, tokens.SessionToken)
	if !errors.Is(err, ErrDisabledUser) {
		t.Fatalf("expected disabled user error, got %v", err)
	}
}

func TestLoginRateLimit(t *testing.T) {
	ctx := context.Background()
	service, _, cleanup := newTestAuthService(t)
	defer cleanup()

	for i := 0; i < maxFailedLoginAttempts; i++ {
		_, _ = service.Login(ctx, LoginInput{
			Email:     "missing@example.com",
			Password:  "wrong-password",
			IPAddress: "127.0.0.1",
		})
	}
	_, err := service.Login(ctx, LoginInput{
		Email:     "missing@example.com",
		Password:  "wrong-password",
		IPAddress: "127.0.0.1",
	})
	if !errors.Is(err, ErrLoginRateLimited) {
		t.Fatalf("expected rate limited error, got %v", err)
	}
}

func newTestAuthService(t *testing.T) (*Service, Repository, func()) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
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
	repo := NewSQLRepository(store)
	service := NewService(repo, cfg)
	return service, repo, func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	}
}
