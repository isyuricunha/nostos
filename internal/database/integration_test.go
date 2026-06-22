//go:build integration

package database_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/database"
)

func TestSQLiteMigrationsIntegration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := database.Open(ctx, config.DatabaseConfig{Driver: "sqlite", URL: filepath.Join(dir, "integration.db")})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()
	if err := database.RunMigrations(ctx, store, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run sqlite migrations: %v", err)
	}
	assertCoreTables(ctx, t, store)
}

func TestPostgresMigrationsIntegration(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := database.Open(ctx, config.DatabaseConfig{Driver: "postgres", URL: dsn})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer store.Close()
	if err := database.RunMigrations(ctx, store, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("run postgres migrations: %v", err)
	}
	assertCoreTables(ctx, t, store)
}

func assertCoreTables(ctx context.Context, t *testing.T, store *database.Store) {
	t.Helper()
	for _, table := range []string{"users", "providers", "conversations", "messages", "tasks", "mcp_servers", "reply_presets"} {
		query := "SELECT COUNT(*) FROM " + table
		var count int
		if err := store.DB.QueryRowContext(ctx, query).Scan(&count); err != nil {
			t.Fatalf("table %s is not queryable: %v", table, err)
		}
	}
}
