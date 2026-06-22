package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/yuricunha/nostos/internal/config"
)

type Store struct {
	DB      *sql.DB
	Dialect Dialect
}

type Dialect string

const (
	Postgres Dialect = "postgres"
	SQLite   Dialect = "sqlite"
)

func Open(ctx context.Context, cfg config.DatabaseConfig) (*Store, error) {
	var driverName string
	var dsn string
	var dialect Dialect

	switch cfg.Driver {
	case "postgres":
		driverName = "pgx"
		dsn = cfg.URL
		dialect = Postgres
	case "sqlite":
		driverName = "sqlite"
		dsn = sqliteDSN(cfg.URL)
		dialect = SQLite
		if err := ensureSQLiteDirectory(cfg.URL); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	if dialect == SQLite {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(20)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(30 * time.Minute)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if dialect == SQLite {
		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{DB: db, Dialect: dialect}, nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func RunMigrations(ctx context.Context, store *Store, root string) error {
	if store == nil || store.DB == nil {
		return errors.New("database store is nil")
	}
	dialectDir := string(store.Dialect)
	dir := filepath.Join(root, dialectDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations directory %s: %w", dir, err)
	}
	if err := ensureMigrationsTable(ctx, store); err != nil {
		return err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	for _, name := range files {
		applied, err := migrationApplied(ctx, store, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		path := filepath.Join(dir, name)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", path, err)
		}
		if err := applyMigration(ctx, store, name, string(sqlBytes)); err != nil {
			return err
		}
	}
	return nil
}

func ensureMigrationsTable(ctx context.Context, store *Store) error {
	query := `CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	applied_at TEXT NOT NULL
)`
	if store.Dialect == Postgres {
		query = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`
	}
	_, err := store.DB.ExecContext(ctx, query)
	return err
}

func migrationApplied(ctx context.Context, store *Store, version string) (bool, error) {
	var existing string
	err := store.DB.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE version = ?", version).Scan(&existing)
	if store.Dialect == Postgres {
		err = store.DB.QueryRowContext(ctx, "SELECT version FROM schema_migrations WHERE version = $1", version).Scan(&existing)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func applyMigration(ctx context.Context, store *Store, version string, sqlText string) error {
	tx, err := store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	insert := "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)"
	args := []any{version, time.Now().UTC().Format(time.RFC3339Nano)}
	if store.Dialect == Postgres {
		insert = "INSERT INTO schema_migrations (version) VALUES ($1)"
		args = []any{version}
	}
	if _, err := tx.ExecContext(ctx, insert, args...); err != nil {
		return err
	}
	return tx.Commit()
}

func sqliteDSN(path string) string {
	if strings.HasPrefix(path, "file:") {
		if strings.Contains(path, "?") {
			return path + "&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
		}
		return path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}
	escaped := url.PathEscape(path)
	if filepath.IsAbs(path) {
		escaped = "file://" + escaped
	} else {
		escaped = "file:" + escaped
	}
	return escaped + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
}

func ensureSQLiteDirectory(path string) error {
	if strings.HasPrefix(path, "file:") || path == ":memory:" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o750)
}
