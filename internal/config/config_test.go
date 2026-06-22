package config

import "testing"

func TestProductionRequiresSecrets(t *testing.T) {
	cfg := Config{
		AppEnv:   "production",
		Host:     "0.0.0.0",
		Port:     7000,
		BaseURL:  "http://localhost:7000",
		Timezone: "UTC",
		Database: DatabaseConfig{
			Driver: "postgres",
			URL:    "postgresql://example",
		},
		Worker: WorkerConfig{
			Concurrency:  1,
			PollInterval: 1_000_000_000,
		},
		Tasks: TaskConfig{},
		Chat: ChatConfig{
			RecentMessageLimit: 1,
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config without secrets to fail")
	}
}

func TestDevelopmentSQLiteDefaultURL(t *testing.T) {
	cfg := Config{
		AppEnv:   "development",
		Host:     "127.0.0.1",
		Port:     7000,
		BaseURL:  "http://localhost:7000",
		Timezone: "UTC",
		Database: DatabaseConfig{
			Driver: "sqlite",
		},
		Worker: WorkerConfig{
			Concurrency:  1,
			PollInterval: 1_000_000_000,
		},
		Chat: ChatConfig{
			RecentMessageLimit: 1,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate development config: %v", err)
	}
	if cfg.Database.URL == "" {
		t.Fatal("expected development SQLite URL default")
	}
}
