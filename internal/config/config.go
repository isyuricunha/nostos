package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv        string
	Host          string
	Port          int
	BaseURL       string
	Timezone      string
	LogLevel      string
	DataDir       string
	MigrationsDir string
	WebDistDir    string

	Database DatabaseConfig
	Security SecurityConfig
	Worker   WorkerConfig
	Tasks    TaskConfig
	Chat     ChatConfig
	Models   ModelConfig
	Runtime  RuntimeConfig
}

type DatabaseConfig struct {
	Driver string
	URL    string
}

type SecurityConfig struct {
	EncryptionKey     []byte
	SessionSecret     string
	SecureCookies     bool
	TrustedProxies    []string
	AllowedOrigins    []string
	BootstrapEmail    string
	BootstrapPassword string
	SessionTTL        time.Duration
	AuditLogRetention time.Duration
}

type WorkerConfig struct {
	Concurrency  int
	PollInterval time.Duration
}

type TaskConfig struct {
	DefaultTimeout time.Duration
	MaxRetries     int
	RunRetention   time.Duration
}

type ChatConfig struct {
	DefaultTimeout     time.Duration
	MaxToolIterations  int
	ContextThreshold   int
	RecentMessageLimit int
}

type ModelConfig struct {
	RefreshTimeout time.Duration
}

type RuntimeConfig struct {
	Version string
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:        envString("APP_ENV", "production"),
		Host:          envString("APP_HOST", "0.0.0.0"),
		BaseURL:       envString("APP_BASE_URL", "http://localhost:7000"),
		Timezone:      envString("APP_TIMEZONE", "UTC"),
		LogLevel:      envString("APP_LOG_LEVEL", "info"),
		DataDir:       envString("DATA_DIR", "/data"),
		MigrationsDir: envString("MIGRATIONS_DIR", "migrations"),
		WebDistDir:    envString("WEB_DIST_DIR", "web/dist"),
		Database: DatabaseConfig{
			Driver: strings.ToLower(envString("DATABASE_DRIVER", "postgres")),
			URL:    envString("DATABASE_URL", ""),
		},
		Security: SecurityConfig{
			SecureCookies:     envBool("SECURE_COOKIES", false),
			TrustedProxies:    envCSV("TRUSTED_PROXIES"),
			AllowedOrigins:    envCSV("ALLOWED_ORIGINS"),
			BootstrapEmail:    strings.TrimSpace(envString("APP_BOOTSTRAP_EMAIL", "")),
			BootstrapPassword: envString("APP_BOOTSTRAP_PASSWORD", ""),
			SessionTTL:        envDuration("SESSION_TTL", 720*time.Hour),
			AuditLogRetention: envDuration("AUDIT_LOG_RETENTION", 2160*time.Hour),
		},
		Worker: WorkerConfig{
			Concurrency:  envInt("WORKER_CONCURRENCY", 4),
			PollInterval: envDuration("WORKER_POLL_INTERVAL", 2*time.Second),
		},
		Tasks: TaskConfig{
			DefaultTimeout: envDuration("TASK_DEFAULT_TIMEOUT", 10*time.Minute),
			MaxRetries:     envInt("TASK_MAX_RETRIES", 3),
			RunRetention:   envDuration("TASK_RUN_RETENTION", 720*time.Hour),
		},
		Chat: ChatConfig{
			DefaultTimeout:     envDuration("CHAT_DEFAULT_TIMEOUT", 5*time.Minute),
			MaxToolIterations:  envInt("CHAT_MAX_TOOL_ITERATIONS", 8),
			ContextThreshold:   envInt("CHAT_CONTEXT_THRESHOLD", 60000),
			RecentMessageLimit: envInt("CHAT_RECENT_MESSAGE_LIMIT", 30),
		},
		Models: ModelConfig{
			RefreshTimeout: envDuration("MODEL_REFRESH_TIMEOUT", 60*time.Second),
		},
	}

	port, err := envPort("APP_PORT", 7000)
	if err != nil {
		return Config{}, err
	}
	cfg.Port = port

	encryptionKey, err := loadKey("APP_ENCRYPTION_KEY")
	if err != nil {
		return Config{}, err
	}
	cfg.Security.EncryptionKey = encryptionKey
	cfg.Security.SessionSecret = envString("APP_SESSION_SECRET", "")

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) HTTPAddress() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

func (c *Config) Validate() error {
	if c.Host == "" {
		return errors.New("APP_HOST is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return errors.New("APP_PORT must be between 1 and 65535")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("APP_TIMEZONE must be a valid IANA timezone: %w", err)
	}
	switch c.Database.Driver {
	case "postgres", "sqlite":
	default:
		return errors.New("DATABASE_DRIVER must be postgres or sqlite")
	}
	if c.Database.URL == "" {
		if c.IsProduction() {
			return errors.New("DATABASE_URL is required in production")
		}
		if c.Database.Driver == "sqlite" {
			c.Database.URL = "data/nostos.db"
		}
	}
	if c.IsProduction() {
		if len(c.Security.EncryptionKey) != 32 {
			return errors.New("APP_ENCRYPTION_KEY must be a 32-byte base64 value in production")
		}
		if len(c.Security.SessionSecret) < 32 {
			return errors.New("APP_SESSION_SECRET must be at least 32 characters in production")
		}
	}
	if len(c.Security.EncryptionKey) != 0 && len(c.Security.EncryptionKey) != 32 {
		return errors.New("APP_ENCRYPTION_KEY must decode to 32 bytes")
	}
	if c.Worker.Concurrency < 1 {
		return errors.New("WORKER_CONCURRENCY must be at least 1")
	}
	if c.Worker.PollInterval < 250*time.Millisecond {
		return errors.New("WORKER_POLL_INTERVAL must be at least 250ms")
	}
	if c.Tasks.MaxRetries < 0 {
		return errors.New("TASK_MAX_RETRIES cannot be negative")
	}
	if c.Chat.MaxToolIterations < 0 {
		return errors.New("CHAT_MAX_TOOL_ITERATIONS cannot be negative")
	}
	if c.Chat.RecentMessageLimit < 1 {
		return errors.New("CHAT_RECENT_MESSAGE_LIMIT must be at least 1")
	}
	if c.Models.RefreshTimeout == 0 {
		c.Models.RefreshTimeout = 60 * time.Second
	}
	if c.Models.RefreshTimeout < time.Second || c.Models.RefreshTimeout > 5*time.Minute {
		return errors.New("MODEL_REFRESH_TIMEOUT must be between 1s and 300s")
	}
	return nil
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envCSV(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envPort(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number: %w", key, err)
	}
	return parsed, nil
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func loadKey(key string) ([]byte, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be base64 encoded: %w", key, err)
	}
	return decoded, nil
}
