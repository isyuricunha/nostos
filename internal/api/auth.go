package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yuricunha/nostos/internal/auth"
	"github.com/yuricunha/nostos/internal/config"
)

type AuthDeps struct {
	Config config.Config
	Auth   *auth.Service
}

type authHandler struct {
	cfg  config.Config
	auth *auth.Service
}

type principalKey struct{}

func newAuthHandler(deps AuthDeps) *authHandler {
	return &authHandler{cfg: deps.Config, auth: deps.Auth}
}

func (h *authHandler) Routes(r chi.Router) {
	r.Get("/setup/status", h.setupStatus)
	r.Post("/setup", h.setupOwner)
	r.Post("/auth/login", h.login)

	r.Group(func(r chi.Router) {
		r.Use(h.requireAuth)
		r.Get("/auth/me", h.me)
		r.Get("/sessions", h.sessions)
		r.Group(func(r chi.Router) {
			r.Use(h.requireCSRF)
			r.Post("/auth/logout", h.logout)
			r.Delete("/sessions/{sessionID}", h.revokeSession)
		})
	})
}

func (h *authHandler) setupStatus(w http.ResponseWriter, r *http.Request) {
	available, err := h.auth.SetupAvailable(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "setup_status_failed", "Unable to read setup status.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"available": available})
}

func (h *authHandler) setupOwner(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email           string `json:"email"`
		DisplayName     string `json:"display_name"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	user, err := h.auth.CreateOwner(r.Context(), auth.SetupInput{
		Email:           input.Email,
		DisplayName:     input.DisplayName,
		Password:        input.Password,
		ConfirmPassword: input.ConfirmPassword,
		IPAddress:       clientIP(r),
		UserAgent:       r.UserAgent(),
	})
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	tokens, err := h.auth.CreateSessionForUser(r.Context(), user, clientIP(r), r.UserAgent())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "session_create_failed", "Owner was created, but the login session could not be created.", nil)
		return
	}
	h.setAuthCookies(w, tokens)
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := h.auth.Login(r.Context(), auth.LoginInput{
		Email:     input.Email,
		Password:  input.Password,
		IPAddress: clientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		h.writeAuthError(w, r, err)
		return
	}
	h.setAuthCookies(w, result.Tokens)
	writeJSON(w, http.StatusOK, map[string]any{"user": result.User})
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	principal := Principal(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"user": principal.User})
}

func (h *authHandler) sessions(w http.ResponseWriter, r *http.Request) {
	principal := Principal(r.Context())
	sessions, err := h.auth.ListSessions(r.Context(), principal)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "sessions_failed", "Unable to list sessions.", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	principal := Principal(r.Context())
	if err := h.auth.Logout(r.Context(), principal, clientIP(r), r.UserAgent()); err != nil {
		writeError(w, r, http.StatusInternalServerError, "logout_failed", "Unable to log out.", nil)
		return
	}
	h.clearAuthCookies(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *authHandler) revokeSession(w http.ResponseWriter, r *http.Request) {
	principal := Principal(r.Context())
	sessionID := chi.URLParam(r, "sessionID")
	if strings.TrimSpace(sessionID) == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_session", "Session ID is required.", nil)
		return
	}
	if err := h.auth.RevokeSession(r.Context(), principal, sessionID, clientIP(r), r.UserAgent()); err != nil {
		if auth.IsNotFound(err) {
			writeError(w, r, http.StatusNotFound, "session_not_found", "The session was not found.", nil)
			return
		}
		writeError(w, r, http.StatusInternalServerError, "session_revoke_failed", "Unable to revoke session.", nil)
		return
	}
	if sessionID == principal.Session.ID {
		h.clearAuthCookies(w)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *authHandler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(auth.SessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeError(w, r, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		principal, err := h.auth.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			h.clearAuthCookies(w)
			writeError(w, r, http.StatusUnauthorized, "authentication_required", "Authentication is required.", nil)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *authHandler) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal := Principal(r.Context())
		token := r.Header.Get(auth.CSRFHeaderName)
		if token == "" {
			if cookie, err := r.Cookie(auth.CSRFCookieName); err == nil {
				token = cookie.Value
			}
		}
		if err := h.auth.VerifyCSRF(principal, token); err != nil {
			writeError(w, r, http.StatusForbidden, "csrf_failed", "The request could not be verified.", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *authHandler) setAuthCookies(w http.ResponseWriter, tokens auth.AuthTokens) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    tokens.SessionToken,
		Path:     "/",
		Expires:  tokens.ExpiresAt,
		HttpOnly: true,
		Secure:   h.cfg.Security.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CSRFCookieName,
		Value:    tokens.CSRFToken,
		Path:     "/",
		Expires:  tokens.ExpiresAt,
		HttpOnly: false,
		Secure:   h.cfg.Security.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *authHandler) clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{auth.SessionCookieName, auth.CSRFCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: name == auth.SessionCookieName,
			Secure:   h.cfg.Security.SecureCookies,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func (h *authHandler) writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrSetupClosed):
		writeError(w, r, http.StatusConflict, "setup_closed", "Owner setup is already complete.", nil)
	case errors.Is(err, auth.ErrInvalidInput):
		writeError(w, r, http.StatusBadRequest, "invalid_input", err.Error(), nil)
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "The email or password is incorrect.", nil)
	case errors.Is(err, auth.ErrLoginRateLimited):
		writeError(w, r, http.StatusTooManyRequests, "login_rate_limited", "Too many login attempts. Try again later.", nil)
	case errors.Is(err, auth.ErrDisabledUser):
		writeError(w, r, http.StatusForbidden, "user_disabled", "This user is disabled.", nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "auth_failed", "Authentication request failed.", nil)
	}
}

func Principal(ctx context.Context) auth.Principal {
	principal, _ := ctx.Value(principalKey{}).(auth.Principal)
	return principal
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.", nil)
		return false
	}
	return true
}

func sameOrigin(cfg config.Config) func(http.Handler) http.Handler {
	allowed := map[string]struct{}{}
	if baseOrigin := originFromURL(cfg.BaseURL); baseOrigin != "" {
		allowed[baseOrigin] = struct{}{}
	}
	for _, origin := range cfg.Security.AllowedOrigins {
		if parsed := originFromURL(origin); parsed != "" {
			allowed[parsed] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := allowed[origin]; !ok {
				writeError(w, r, http.StatusForbidden, "origin_forbidden", "The request origin is not allowed.", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func originFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(r.RemoteAddr)
}
