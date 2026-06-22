package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/crypto"
	"github.com/isyuricunha/nostos/internal/id"
)

var (
	ErrSetupClosed         = errors.New("setup is closed")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrLoginRateLimited    = errors.New("too many login attempts")
	ErrDisabledUser        = errors.New("user is disabled")
	ErrExpiredSession      = errors.New("session is expired")
	ErrRevokedSession      = errors.New("session is revoked")
	ErrInvalidInput        = errors.New("invalid input")
	ErrCSRFTokenMismatch   = errors.New("csrf token mismatch")
	ErrSessionTokenMissing = errors.New("session token is missing")
)

type Service struct {
	repo    Repository
	cfg     config.Config
	limiter *loginLimiter
}

func NewService(repo Repository, cfg config.Config) *Service {
	return &Service{
		repo:    repo,
		cfg:     cfg,
		limiter: newLoginLimiter(),
	}
}

func (s *Service) SetupAvailable(ctx context.Context) (bool, error) {
	exists, err := s.repo.OwnerExists(ctx)
	return !exists, err
}

func (s *Service) BootstrapOwner(ctx context.Context) (bool, error) {
	email := strings.TrimSpace(s.cfg.Security.BootstrapEmail)
	password := s.cfg.Security.BootstrapPassword
	if email == "" || password == "" {
		return false, nil
	}
	available, err := s.SetupAvailable(ctx)
	if err != nil {
		return false, err
	}
	if !available {
		return false, nil
	}
	_, err = s.CreateOwner(ctx, SetupInput{
		Email:           email,
		DisplayName:     displayNameFromEmail(email),
		Password:        password,
		ConfirmPassword: password,
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) CreateOwner(ctx context.Context, input SetupInput) (User, error) {
	available, err := s.SetupAvailable(ctx)
	if err != nil {
		return User{}, err
	}
	if !available {
		return User{}, ErrSetupClosed
	}
	email, displayName, err := validateUserInput(input.Email, input.DisplayName)
	if err != nil {
		return User{}, err
	}
	if err := validatePassword(input.Password, input.ConfirmPassword); err != nil {
		return User{}, err
	}
	passwordHash, err := crypto.HashPassword(input.Password)
	if err != nil {
		return User{}, err
	}
	user, err := s.repo.CreateOwner(ctx, User{
		Email:       email,
		DisplayName: displayName,
	}, passwordHash)
	if err != nil {
		return User{}, err
	}
	_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
		WorkspaceID: user.WorkspaceID,
		ActorUserID: user.ID,
		EventType:   AuditSettingsChanged,
		IPAddress:   input.IPAddress,
		UserAgent:   input.UserAgent,
		Metadata: map[string]any{
			"action": "owner_setup",
		},
	})
	return user, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	email := normalizeEmail(input.Email)
	key := loginKey(input.IPAddress, email)
	if s.limiter.Blocked(key, time.Now()) {
		_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
			EventType: AuditLoginFailure,
			IPAddress: input.IPAddress,
			UserAgent: input.UserAgent,
			Metadata:  map[string]any{"email": email, "reason": "rate_limited"},
		})
		return AuthResult{}, ErrLoginRateLimited
	}

	record, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil || !crypto.VerifyPassword(record.PasswordHash, input.Password) {
		s.limiter.RecordFailure(key, time.Now())
		_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
			EventType: AuditLoginFailure,
			IPAddress: input.IPAddress,
			UserAgent: input.UserAgent,
			Metadata:  map[string]any{"email": email, "reason": "invalid_credentials"},
		})
		return AuthResult{}, ErrInvalidCredentials
	}
	if record.DisabledAt != nil {
		s.limiter.RecordFailure(key, time.Now())
		_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
			WorkspaceID: record.WorkspaceID,
			ActorUserID: record.ID,
			EventType:   AuditLoginFailure,
			IPAddress:   input.IPAddress,
			UserAgent:   input.UserAgent,
			Metadata:    map[string]any{"email": email, "reason": "disabled_user"},
		})
		return AuthResult{}, ErrDisabledUser
	}

	tokens, session, err := s.createSession(ctx, record.User, input.IPAddress, input.UserAgent)
	if err != nil {
		return AuthResult{}, err
	}
	s.limiter.RecordSuccess(key)
	_ = s.repo.InsertAuditEvent(ctx, AuditEvent{
		WorkspaceID: record.WorkspaceID,
		ActorUserID: record.ID,
		EventType:   AuditLoginSuccess,
		IPAddress:   input.IPAddress,
		UserAgent:   input.UserAgent,
		Metadata:    map[string]any{"session_id": session.ID},
	})
	return AuthResult{User: record.User, Tokens: tokens}, nil
}

func (s *Service) CreateSessionForUser(ctx context.Context, user User, ipAddress string, userAgent string) (AuthTokens, error) {
	tokens, _, err := s.createSession(ctx, user, ipAddress, userAgent)
	return tokens, err
}

func (s *Service) Authenticate(ctx context.Context, sessionToken string) (Principal, error) {
	if sessionToken == "" {
		return Principal{}, ErrSessionTokenMissing
	}
	tokenHash := crypto.HashToken(s.cfg.Security.SessionSecret, sessionToken)
	session, err := s.repo.FindSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return Principal{}, ErrInvalidCredentials
	}
	now := time.Now().UTC()
	if session.RevokedAt != nil {
		return Principal{}, ErrRevokedSession
	}
	if !session.ExpiresAt.After(now) {
		return Principal{}, ErrExpiredSession
	}
	record, err := s.repo.FindUserByID(ctx, session.UserID)
	if err != nil {
		return Principal{}, err
	}
	if record.DisabledAt != nil {
		return Principal{}, ErrDisabledUser
	}
	return Principal{User: record.User, Session: session}, nil
}

func (s *Service) VerifyCSRF(principal Principal, token string) error {
	if token == "" {
		return ErrCSRFTokenMismatch
	}
	if crypto.HashToken(s.cfg.Security.SessionSecret, token) != principal.Session.CSRFTokenHash {
		return ErrCSRFTokenMismatch
	}
	return nil
}

func (s *Service) Logout(ctx context.Context, principal Principal, ipAddress string, userAgent string) error {
	now := time.Now().UTC()
	if err := s.repo.RevokeSession(ctx, principal.Session.ID, principal.User.ID, now); err != nil {
		return err
	}
	return s.repo.InsertAuditEvent(ctx, AuditEvent{
		WorkspaceID: principal.User.WorkspaceID,
		ActorUserID: principal.User.ID,
		EventType:   AuditLogout,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Metadata:    map[string]any{"session_id": principal.Session.ID},
	})
}

func (s *Service) ListSessions(ctx context.Context, principal Principal) ([]Session, error) {
	return s.repo.ListSessions(ctx, principal.User.ID)
}

func (s *Service) RevokeSession(ctx context.Context, principal Principal, sessionID string, ipAddress string, userAgent string) error {
	now := time.Now().UTC()
	if err := s.repo.RevokeSession(ctx, sessionID, principal.User.ID, now); err != nil {
		return err
	}
	return s.repo.InsertAuditEvent(ctx, AuditEvent{
		WorkspaceID: principal.User.WorkspaceID,
		ActorUserID: principal.User.ID,
		EventType:   AuditSessionRevoked,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
		Metadata:    map[string]any{"session_id": sessionID},
	})
}

func (s *Service) createSession(ctx context.Context, user User, ipAddress string, userAgent string) (AuthTokens, Session, error) {
	sessionToken, err := crypto.RandomToken(defaultSessionTokenSize)
	if err != nil {
		return AuthTokens{}, Session{}, err
	}
	csrfToken, err := crypto.RandomToken(defaultSessionTokenSize)
	if err != nil {
		return AuthTokens{}, Session{}, err
	}
	now := time.Now().UTC()
	session := Session{
		ID:            id.New(),
		UserID:        user.ID,
		TokenHash:     crypto.HashToken(s.cfg.Security.SessionSecret, sessionToken),
		CSRFTokenHash: crypto.HashToken(s.cfg.Security.SessionSecret, csrfToken),
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		ExpiresAt:     now.Add(s.cfg.Security.SessionTTL),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return AuthTokens{}, Session{}, err
	}
	return AuthTokens{
		SessionToken: sessionToken,
		CSRFToken:    csrfToken,
		ExpiresAt:    session.ExpiresAt,
	}, session, nil
}

func validateUserInput(email string, displayName string) (string, string, error) {
	email = normalizeEmail(email)
	if _, err := mail.ParseAddress(email); err != nil {
		return "", "", fmt.Errorf("%w: email is invalid", ErrInvalidInput)
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = displayNameFromEmail(email)
	}
	if len(displayName) > 120 {
		return "", "", fmt.Errorf("%w: display name is too long", ErrInvalidInput)
	}
	return email, displayName, nil
}

func validatePassword(password string, confirmPassword string) error {
	if password != confirmPassword {
		return fmt.Errorf("%w: password confirmation does not match", ErrInvalidInput)
	}
	if len(password) < MinimumPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, MinimumPasswordLength)
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func displayNameFromEmail(email string) string {
	local := strings.Split(normalizeEmail(email), "@")[0]
	local = strings.ReplaceAll(local, ".", " ")
	local = strings.ReplaceAll(local, "_", " ")
	local = strings.TrimSpace(local)
	if local == "" {
		return "Owner"
	}
	return strings.Title(local)
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

type loginAttempt struct {
	count     int
	firstSeen time.Time
	blockedAt time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: make(map[string]loginAttempt)}
}

func (l *loginLimiter) Blocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	attempt := l.attempts[key]
	if attempt.blockedAt.IsZero() {
		return false
	}
	if now.Sub(attempt.blockedAt) > loginThrottleLockout {
		delete(l.attempts, key)
		return false
	}
	return true
}

func (l *loginLimiter) RecordFailure(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	attempt := l.attempts[key]
	if attempt.firstSeen.IsZero() || now.Sub(attempt.firstSeen) > loginThrottleWindow {
		attempt = loginAttempt{firstSeen: now}
	}
	attempt.count++
	if attempt.count >= maxFailedLoginAttempts {
		attempt.blockedAt = now
	}
	l.attempts[key] = attempt
}

func (l *loginLimiter) RecordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func loginKey(ipAddress string, email string) string {
	return strings.TrimSpace(ipAddress) + "|" + normalizeEmail(email)
}
