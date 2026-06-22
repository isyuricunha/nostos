package logging

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/isyuricunha/nostos/internal/config"
)

func New(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	options := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if isSensitiveKey(attr.Key) {
				return slog.String(attr.Key, "[redacted]")
			}
			if attr.Value.Kind() == slog.KindString && len(attr.Value.String()) > 1024 {
				return slog.String(attr.Key, attr.Value.String()[:1024]+"...[truncated]")
			}
			return attr
		},
	}

	var handler slog.Handler
	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, options)
	} else {
		handler = slog.NewTextHandler(os.Stdout, options)
	}
	return slog.New(handler)
}

func WriteJSON(w io.Writer, payload any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(key)
	return strings.Contains(key, "password") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "api_key") ||
		strings.Contains(key, "authorization")
}
