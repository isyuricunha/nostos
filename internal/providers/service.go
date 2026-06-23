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
	ErrInvalidInput      = errors.New("invalid provider input")
	ErrSecretMissing     = errors.New("provider secret is missing")
	ErrRefreshInProgress = errors.New("model refresh is already running")
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
	modelIDs, err := s.client.ListModels(ctx, s.providerForModelRefresh(provider), apiKey)
	status := "healthy"
	lastError := ""
	if err != nil {
		status = "unhealthy"
		lastError = err.Error()
		_ = s.repo.UpdateHealth(ctx, principal.WorkspaceID, providerID, status, lastError, time.Now().UTC())
		return nil, err
	}
	_ = s.repo.UpdateHealth(ctx, principal.WorkspaceID, providerID, status, lastError, time.Now().UTC())
	return s.repo.UpsertRefreshedModels(ctx, principal.WorkspaceID, providerID, modelIDs)
}

func (s *Service) ListModels(ctx context.Context, principal PrincipalContext, providerID string) ([]Model, error) {
	if strings.TrimSpace(providerID) != "" {
		provider, _, err := s.repo.Get(ctx, principal.WorkspaceID, providerID)
		if err != nil {
			return nil, err
		}
		providerID = provider.ID
	}
	return s.repo.ListModels(ctx, ModelQuery{WorkspaceID: principal.WorkspaceID, ProviderID: providerID, IncludeUnavailable: true})
}

func (s *Service) ListCatalogModels(ctx context.Context, principal PrincipalContext, query ModelQuery) ([]Model, error) {
	query.WorkspaceID = principal.WorkspaceID
	return s.repo.ListModels(ctx, query)
}

func (s *Service) CreateManualModel(ctx context.Context, principal PrincipalContext, input ModelInput) (Model, error) {
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.ModelID = strings.TrimSpace(input.ModelID)
	if input.ProviderID == "" || input.ModelID == "" {
		return Model{}, fmt.Errorf("%w: provider_id and model_id are required", ErrInvalidInput)
	}
	return s.repo.CreateManualModel(ctx, principal.WorkspaceID, input)
}

func (s *Service) UpdateModel(ctx context.Context, principal PrincipalContext, modelID string, patch ModelPatch) (Model, error) {
	return s.repo.UpdateModel(ctx, principal.WorkspaceID, modelID, patch)
}

func (s *Service) CleanupUnavailableModels(ctx context.Context, principal PrincipalContext, providerID string) (int, error) {
	if _, _, err := s.repo.Get(ctx, principal.WorkspaceID, providerID); err != nil {
		return 0, err
	}
	return s.repo.CleanupUnavailableModels(ctx, principal.WorkspaceID, providerID)
}

func (s *Service) StartModelRefresh(ctx context.Context, principal PrincipalContext, providerID string) (ModelRefreshStatus, error) {
	startedAt := time.Now().UTC()
	started, err := s.repo.TryStartModelRefresh(ctx, principal.WorkspaceID, providerID, startedAt)
	if err != nil {
		return ModelRefreshStatus{}, err
	}
	status, err := s.repo.ModelRefreshStatus(ctx, principal.WorkspaceID, providerID)
	if err != nil {
		return ModelRefreshStatus{}, err
	}
	if !started {
		return status, ErrRefreshInProgress
	}
	go s.runModelRefresh(principal.WorkspaceID, providerID, startedAt)
	return status, nil
}

func (s *Service) ModelRefreshStatus(ctx context.Context, principal PrincipalContext, providerID string) (ModelRefreshStatus, error) {
	return s.repo.ModelRefreshStatus(ctx, principal.WorkspaceID, providerID)
}

func (s *Service) ListModelRoles(ctx context.Context, principal PrincipalContext) ([]ModelRoleBinding, error) {
	return s.repo.ListModelRoles(ctx, principal.WorkspaceID)
}

func (s *Service) SetModelRole(ctx context.Context, principal PrincipalContext, role string, input ModelRoleInput) ([]ModelRoleBinding, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if !validModelRole(role) {
		return nil, fmt.Errorf("%w: model role is invalid", ErrInvalidInput)
	}
	refs := make([]ModelRoleReference, 0, len(input.Models))
	for _, ref := range input.Models {
		ref.ProviderID = strings.TrimSpace(ref.ProviderID)
		ref.ModelID = strings.TrimSpace(ref.ModelID)
		if ref.ProviderID == "" || ref.ModelID == "" {
			continue
		}
		if _, _, err := s.repo.Get(ctx, principal.WorkspaceID, ref.ProviderID); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return s.repo.ReplaceModelRoleBindings(ctx, principal.WorkspaceID, role, refs)
}

func (s *Service) ResolveModelRole(ctx context.Context, workspaceID string, role string) (RoleResolution, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if !validModelRole(role) {
		return RoleResolution{}, fmt.Errorf("%w: model role is invalid", ErrInvalidInput)
	}
	bindings, err := s.repo.ListModelRoles(ctx, workspaceID)
	if err != nil {
		return RoleResolution{}, err
	}
	for _, binding := range bindings {
		if binding.Role != role {
			continue
		}
		provider, secret, err := s.repo.Get(ctx, workspaceID, binding.ProviderID)
		if err != nil || !provider.Enabled || strings.TrimSpace(binding.ModelID) == "" {
			continue
		}
		apiKey, err := s.resolveAPIKey(secret)
		if err != nil {
			continue
		}
		return RoleResolution{Provider: provider, APIKey: apiKey, ModelID: binding.ModelID, Role: role, Reason: "model_role"}, nil
	}
	provider, apiKey, err := s.ResolveDefaultForChat(ctx, workspaceID)
	if err != nil {
		return RoleResolution{}, err
	}
	model := provider.DefaultModel
	if model == "" {
		model = provider.FallbackModel
	}
	if model == "" {
		return RoleResolution{}, fmt.Errorf("%w: %s model is not configured", ErrInvalidInput, role)
	}
	return RoleResolution{Provider: provider, APIKey: apiKey, ModelID: model, Role: role, Reason: "legacy_provider_default"}, nil
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

func (s *Service) ResolveDefaultForChat(ctx context.Context, workspaceID string) (Provider, string, error) {
	items, err := s.repo.List(ctx, workspaceID)
	if err != nil {
		return Provider{}, "", err
	}
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		provider, secret, err := s.repo.Get(ctx, workspaceID, item.ID)
		if err != nil {
			return Provider{}, "", err
		}
		apiKey, err := s.resolveAPIKey(secret)
		if err != nil {
			continue
		}
		return provider, apiKey, nil
	}
	return Provider{}, "", ErrNotFound
}

func (s *Service) CheckProviderHealth(ctx context.Context, limit int) (string, error) {
	items, secrets, err := s.repo.ListEnabledWithSecrets(ctx, limit)
	if err != nil {
		return "", err
	}
	healthy := 0
	unhealthy := 0
	for index, provider := range items {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		apiKey, err := s.resolveAPIKey(secrets[index])
		started := time.Now()
		status := "healthy"
		lastError := ""
		if err == nil {
			_, err = s.client.ListModels(ctx, provider, apiKey)
		}
		if err != nil {
			status = "unhealthy"
			lastError = sanitizeProviderError(err)
			unhealthy++
		} else {
			healthy++
		}
		latencyMS := int(time.Since(started).Milliseconds())
		if latencyMS < 1 {
			latencyMS = 1
		}
		if updateErr := s.repo.UpdateHealthWithLatency(ctx, provider.WorkspaceID, provider.ID, status, lastError, time.Now().UTC(), latencyMS); updateErr != nil {
			return "", updateErr
		}
	}
	return fmt.Sprintf("provider health checked=%d healthy=%d unhealthy=%d", len(items), healthy, unhealthy), nil
}

func (s *Service) runModelRefresh(workspaceID string, providerID string, startedAt time.Time) {
	timeout := s.cfg.Models.RefreshTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	principal := PrincipalContext{WorkspaceID: workspaceID}
	models, err := s.RefreshModels(ctx, principal, providerID)
	completedAt := time.Now().UTC()
	status := ModelRefreshStatus{
		ProviderID:  providerID,
		State:       "succeeded",
		CompletedAt: &completedAt,
		DurationMS:  int(completedAt.Sub(startedAt).Milliseconds()),
	}
	if err != nil {
		status.State = "failed"
		status.ErrorCategory = "model_refresh_failed"
		status.ErrorMessage = sanitizeProviderError(err)
	} else {
		status.CachedModelCount = len(models)
	}
	_ = s.repo.FinishModelRefresh(context.Background(), workspaceID, providerID, status)
}

func (s *Service) providerForModelRefresh(provider Provider) Provider {
	timeoutMS := int(s.cfg.Models.RefreshTimeout / time.Millisecond)
	if timeoutMS > provider.RequestTimeoutMS {
		provider.RequestTimeoutMS = timeoutMS
	}
	return provider
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

func sanitizeProviderError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}
