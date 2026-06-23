package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/isyuricunha/nostos/internal/agents"
	"github.com/isyuricunha/nostos/internal/chat"
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/feedback"
	"github.com/isyuricunha/nostos/internal/health"
	"github.com/isyuricunha/nostos/internal/mcp"
	"github.com/isyuricunha/nostos/internal/memory"
	"github.com/isyuricunha/nostos/internal/providers"
	"github.com/isyuricunha/nostos/internal/replies"
	"github.com/isyuricunha/nostos/internal/tasks"
)

type RouterDeps struct {
	Config    config.Config
	Logger    *slog.Logger
	Health    *health.Service
	Auth      AuthDeps
	Providers *providers.Service
	Chat      *chat.Service
	Agents    *agents.Service
	Memories  *memory.Service
	MCP       *mcp.Service
	Tasks     *tasks.Service
	Feedback  *feedback.Service
	Replies   *replies.Service
}

type APIError struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()
	r.Use(requestID)
	r.Use(middleware.RealIP)
	r.Use(securityHeaders)
	r.Use(sameOrigin(deps.Config))
	r.Use(requestLogger(deps.Logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Minute))

	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, deps.Health.Live())
	})
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		status := deps.Health.Ready(r.Context())
		code := http.StatusOK
		if !status.Ready {
			code = http.StatusServiceUnavailable
		}
		writeJSON(w, code, status)
	})

	r.Route("/api/v1", func(r chi.Router) {
		if deps.Auth.Auth != nil {
			newAuthHandler(deps.Auth).Routes(r)
		}
		if deps.Auth.Auth != nil && (deps.Providers != nil || deps.Chat != nil || deps.Agents != nil || deps.Memories != nil || deps.MCP != nil || deps.Tasks != nil || deps.Feedback != nil || deps.Replies != nil) {
			r.Group(func(r chi.Router) {
				authHandler := newAuthHandler(deps.Auth)
				r.Use(authHandler.requireAuth)
				r.Use(authHandler.requireCSRF)
				if deps.Providers != nil {
					newProvidersHandler(deps.Providers).Routes(r)
				}
				if deps.Chat != nil {
					newChatHandler(deps.Chat).Routes(r)
				}
				if deps.Agents != nil {
					newAgentsHandler(deps.Agents).Routes(r)
				}
				if deps.Memories != nil {
					newMemoriesHandler(deps.Memories).Routes(r)
				}
				if deps.MCP != nil {
					newMCPHandler(deps.MCP).Routes(r)
				}
				if deps.Tasks != nil {
					newTasksHandler(deps.Tasks).Routes(r)
				}
				if deps.Feedback != nil {
					newFeedbackHandler(deps.Feedback).Routes(r)
				}
				if deps.Replies != nil {
					newRepliesHandler(deps.Replies).Routes(r)
				}
			})
		}
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, deps.Health.Ready(r.Context()))
		})
		r.Get("/diagnostics", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, deps.Health.Ready(r.Context()))
		})
	})

	r.NotFound(spaHandler(deps.Config.WebDistDir))
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "The requested method is not allowed.", nil)
	})
	return r
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(recorder, r)
			if logger != nil {
				logger.Info("request completed",
					"request_id", RequestID(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"status", recorder.Status(),
					"bytes", recorder.BytesWritten(),
					"duration_ms", time.Since(started).Milliseconds(),
				)
			}
		})
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

type requestIDKey struct{}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 128 {
			id = randomID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "request"
	}
	return hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func jsonSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code string, message string, details map[string]any) {
	writeJSON(w, status, APIError{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: RequestID(r.Context()),
			Details:   details,
		},
	})
}

func spaHandler(webDistDir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(webDistDir))
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/health/") {
			writeError(w, r, http.StatusNotFound, "not_found", "The requested endpoint does not exist.", nil)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "The requested method is not allowed.", nil)
			return
		}
		requestedPath := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if requestedPath == "." {
			requestedPath = "index.html"
		}
		if isExistingFile(filepath.Join(webDistDir, requestedPath)) {
			fileServer.ServeHTTP(w, r)
			return
		}
		indexPath := filepath.Join(webDistDir, "index.html")
		if !isExistingFile(indexPath) {
			writeError(w, r, http.StatusServiceUnavailable, "frontend_unavailable", "The frontend build is not available.", nil)
			return
		}
		http.ServeFile(w, r, indexPath)
	}
}

func isExistingFile(path string) bool {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return err == nil && !info.IsDir()
}
