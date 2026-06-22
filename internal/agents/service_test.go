package agents

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yuricunha/nostos/internal/auth"
	"github.com/yuricunha/nostos/internal/config"
	"github.com/yuricunha/nostos/internal/database"
)

func TestEnsureDefaultAgentsCreatesGeneralAssistant(t *testing.T) {
	ctx := context.Background()
	cfg, store, user, cleanup := newAgentTestContext(t)
	defer cleanup()

	service := NewService(NewSQLRepository(store))
	if err := service.EnsureDefaultAgents(ctx); err != nil {
		t.Fatalf("ensure default agents: %v", err)
	}
	agents, err := service.List(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID})
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) != 1 || agents[0].Name != DefaultAgentName {
		t.Fatalf("unexpected agents: %#v", agents)
	}
	if err := service.EnsureDefaultAgents(ctx); err != nil {
		t.Fatalf("second ensure default agents: %v", err)
	}
	agents, err = service.List(ctx, PrincipalContext{WorkspaceID: user.WorkspaceID, UserID: user.ID})
	if err != nil {
		t.Fatalf("list agents after second ensure: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("default agent should be idempotent, got %d", len(agents))
	}
	_ = cfg
}

func newAgentTestContext(t *testing.T) (config.Config, *database.Store, auth.User, func()) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	cfg := config.Config{
		AppEnv:        "development",
		BaseURL:       "http://localhost:7000",
		Timezone:      "UTC",
		MigrationsDir: filepath.Join("..", "..", "migrations"),
		Database:      config.DatabaseConfig{Driver: "sqlite", URL: filepath.Join(dir, "test.db")},
		Security:      config.SecurityConfig{SessionSecret: "test-session-secret-with-enough-length", SessionTTL: time.Hour},
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
