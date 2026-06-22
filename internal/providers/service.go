package providers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/isyuricunha/nostos/internal/auth"
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/crypto"
)

var (
	ErrInvalidInput  = errors.New("invalid provider input")
	ErrSecretMissing = errors.New("provider secret is missing")
)

type Service struct {
	cfg    config.Config
	repo   Repository
	audit  auth.Repository
	client *OpenAIClient
}

func NewService(cfg config.Config, repo Repository, audit auth.Repository, client *OpenAIClient) *Service {
	return &Service{cfg: cfg, repo: repo, audit: audit, client: client}
}

func (s *Service) List(ctx context.Context, principal PrincipalContext) ([]Provider, error) {
	return s.repo.List(ctx, principal.WorkspaceID)
}

func (s *Service) Get(ctx context.Context, principal PrincipalContext, providerID string) (Provider, error) {
	provider, _, err := s.repo.Get(ctx, principal.WorkspaceID, providerID)
	return provider, err
}

func (s *Service) Create(ctx context.Context, principal PrincipalContext, input ProviderInput) (Provider, error) {
	provider, secret, err := s.normalizeInput(principal.WorkspaceID, input, true)
	if err != nil {
		return Provider{}, err
	}
	created, err := s.repo.Create(ctx, provider, secret)
	if err != nil {
		return Provider{}, err
	}
	s.auditProvider(ctx, principal, auth.AuditProviderCreated, created.ID)
	return created, nil
}

func (s *Service) Update(ctx context.Context, principal PrincipalContext, providerID string, input ProviderInput) (Provider, error) {
	provider, secret, err := s.normalizeInput(principal.WorkspaceID, input, false)
	if err != nil {
		return Provider{}, err
	}
	provider.ID = providerID
	var secretPtr *ProviderSecret
	if input.APIKey != nil || input.APIKeyEnvRef != "" {
		secretPtr = &secret
	}
	updated, err := s.repo.Update(ctx, provider, secretPtr)
	if err != nil {
		return Provider{}, err
	}
	s.auditProvider(ctx, principal, auth.AuditProviderUpdated, providerID)
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, principal PrincipalContext, providerID string) error {
	if err := s.repo.Delete(ctx, principal.WorkspaceID, providerID); err != nil {
		return err
	}
	s.auditProvider(ctx, principal, auth.AuditProviderDeleted, providerID)
	return nil
}

func (s *Service) TestConnection(ctx context.Context, principal PrincipalContext, providerID string) error {
	provider, secret, err := s.repo.Get(ctx, principal.WorkspaceID, providerID)
	if err != nil {
		return err
	}
	apiKey, err := s.resolveAPIKey(secret)
	if err != nil {
		return err
	}
	_, err = s.client.ListModels(ctx, provider, apiKey)
	status := "healthy"
	lastError := ""
	if err != nil {
		status = "unhealthy"
		lastError = err.Error()
	}
	_ = s.repo.UpdateHealth(ctx, principal.WorkspaceID, providerID, status, lastError, time.Now().UTC())
	return err
}

func (s *Service) RefreshModels(ctx context.Context, principal PrincipalContext, providerID string) ([]Model, error) {
	provider, secret, err := s.repo.Get(ctx, principal.WorkspaceID, providerID)
	if err != nil {
		return nil, err
	}
	apiKey, err := s.resolveAPIKey(secret)
	if err != nil {
		return nil, err
	}
	modelIDs, err := s.client.ListModels(ctx, provider, apiKey)
	status := "healthy"
	lastError := ""
	if err != nil {
		status = "unhealthy"
		lastError = err.Error()
		_ = s.repo.UpdateHealth(ctx, principal.WorkspaceID, providerID, status, lastError, time.Now().UTC())
		return nil, err
	}
	_ = s.repo.UpdateHealth(ctx, principal.WorkspaceID, providerID, status, lastError, time.Now().UTC())
	return s.repo.ReplaceModels(ctx, providerID, modelIDs, "api")
}

func (s *Service) ListModels(ctx context.Context, principal PrincipalContext, providerID string) ([]Model, error) {
	provider, _, err := s.repo.Get(ctx, principal.WorkspaceID, providerID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListModels(ctx, provider.ID)
}

func (s *Service) ResolveForChat(ctx context.Context, workspaceID string, providerID string) (Provider, string, error) {
	provider, secret, err := s.repo.Get(ctx, workspaceID, providerID)
	if err != nil {
		return Provider{}, "", err
	}
	apiKey, err := s.resolveAPIKey(secret)
	if err != nil {
		return Provider{}, "", err
	}
	return provider, apiKey, nil
}

func (s *Service) normalizeInput(workspaceID string, input ProviderInput, requireSecret bool) (Provider, ProviderSecret, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 120 {
		return Provider{}, ProviderSecret{}, fmt.Errorf("%w: provider name is required", ErrInvalidInput)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return Provider{}, ProviderSecret{}, fmt.Errorf("%w: base URL must be an HTTP URL", ErrInvalidInput)
	}
	timeoutMS := input.RequestTimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = 60000
	}
	if timeoutMS < 1000 || timeoutMS > 600000 {
		return Provider{}, ProviderSecret{}, fmt.Errorf("%w: request timeout must be between 1000 and 600000 milliseconds", ErrInvalidInput)
	}
	secret := ProviderSecret{APIKeyEnvRef: strings.TrimSpace(input.APIKeyEnvRef)}
	if input.APIKey != nil && strings.TrimSpace(*input.APIKey) != "" {
		if len(s.cfg.Security.EncryptionKey) != 32 {
			return Provider{}, ProviderSecret{}, fmt.Errorf("%w: APP_ENCRYPTION_KEY is required to store provider secrets", ErrSecretMissing)
		}
		encrypted, err := crypto.Encrypt(s.cfg.Security.EncryptionKey, strings.TrimSpace(*input.APIKey))
		if err != nil {
			return Provider{}, ProviderSecret{}, err
		}
		secret.EncryptedAPIKey = encrypted
		secret.APIKeyEnvRef = ""
	}
	if requireSecret && secret.EncryptedAPIKey == "" && secret.APIKeyEnvRef == "" {
		return Provider{}, ProviderSecret{}, fmt.Errorf("%w: api_key or api_key_env_ref is required", ErrSecretMissing)
	}
	return Provider{
		WorkspaceID:      workspaceID,
		Name:             name,
		BaseURL:          baseURL,
		APIKeyEnvRef:     secret.APIKeyEnvRef,
		Organization:     strings.TrimSpace(input.Organization),
		Project:          strings.TrimSpace(input.Project),
		CustomHeaders:    sanitizeHeaders(input.CustomHeaders),
		Enabled:          input.Enabled,
		RequestTimeoutMS: timeoutMS,
		DefaultModel:     strings.TrimSpace(input.DefaultModel),
		FallbackModel:    strings.TrimSpace(input.FallbackModel),
	}, secret, nil
}

func (s *Service) resolveAPIKey(secret ProviderSecret) (string, error) {
	if secret.APIKeyEnvRef != "" {
		name := strings.TrimPrefix(secret.APIKeyEnvRef, "env:")
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			return "", fmt.Errorf("%w: %s is empty", ErrSecretMissing, secret.APIKeyEnvRef)
		}
		return value, nil
	}
	if secret.EncryptedAPIKey == "" {
		return "", ErrSecretMissing
	}
	return crypto.Decrypt(s.cfg.Security.EncryptionKey, secret.EncryptedAPIKey)
}

func (s *Service) auditProvider(ctx context.Context, principal PrincipalContext, eventType string, providerID string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.InsertAuditEvent(ctx, auth.AuditEvent{
		WorkspaceID: principal.WorkspaceID,
		ActorUserID: principal.UserID,
		EventType:   eventType,
		IPAddress:   principal.IPAddress,
		UserAgent:   principal.UserAgent,
		Metadata: map[string]any{
			"provider_id": providerID,
		},
	})
}

func sanitizeHeaders(headers map[string]string) map[string]string {
	clean := map[string]string{}
	for key, value := range headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if strings.EqualFold(key, "authorization") {
			continue
		}
		clean[key] = value
	}
	return clean
}
