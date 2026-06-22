package providers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/isyuricunha/nostos/internal/database"
	"github.com/isyuricunha/nostos/internal/id"
)

var ErrNotFound = errors.New("provider not found")

type Repository interface {
	List(ctx context.Context, workspaceID string) ([]Provider, error)
	Get(ctx context.Context, workspaceID string, providerID string) (Provider, ProviderSecret, error)
	Create(ctx context.Context, provider Provider, secret ProviderSecret) (Provider, error)
	Update(ctx context.Context, provider Provider, secret *ProviderSecret) (Provider, error)
	Delete(ctx context.Context, workspaceID string, providerID string) error
	ReplaceModels(ctx context.Context, providerID string, modelIDs []string, source string) ([]Model, error)
	ListModels(ctx context.Context, providerID string) ([]Model, error)
	UpdateHealth(ctx context.Context, workspaceID string, providerID string, status string, lastError string, checkedAt time.Time) error
}

type SQLRepository struct {
	store *database.Store
}

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) List(ctx context.Context, workspaceID string) ([]Provider, error) {
	query := `SELECT id, workspace_id, name, base_url, api_key_env_ref, organization_header, project_header, custom_headers,
enabled, request_timeout_ms, default_model, fallback_model, health_status, last_health_check_at, last_error, created_at, updated_at
FROM providers WHERE workspace_id = ` + r.store.Placeholder(1) + ` ORDER BY name`
	rows, err := r.store.DB.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var providers []Provider
	for rows.Next() {
		provider, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func (r *SQLRepository) Get(ctx context.Context, workspaceID string, providerID string) (Provider, ProviderSecret, error) {
	query := `SELECT id, workspace_id, name, base_url, encrypted_api_key, api_key_env_ref, organization_header, project_header, custom_headers,
enabled, request_timeout_ms, default_model, fallback_model, health_status, last_health_check_at, last_error, created_at, updated_at
FROM providers WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	row := r.store.DB.QueryRowContext(ctx, query, workspaceID, providerID)
	provider, secret, err := scanProviderWithSecret(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Provider{}, ProviderSecret{}, ErrNotFound
	}
	return provider, secret, err
}

func (r *SQLRepository) Create(ctx context.Context, provider Provider, secret ProviderSecret) (Provider, error) {
	now := time.Now().UTC()
	provider.ID = id.New()
	provider.CreatedAt = now
	provider.UpdatedAt = now
	provider.HealthStatus = "unknown"
	query := `INSERT INTO providers (id, workspace_id, name, base_url, encrypted_api_key, api_key_env_ref, organization_header, project_header,
custom_headers, enabled, request_timeout_ms, default_model, fallback_model, health_status, created_at, updated_at)
VALUES (` + placeholders(r.store, 16) + `)`
	_, err := r.store.DB.ExecContext(ctx, query,
		provider.ID,
		provider.WorkspaceID,
		provider.Name,
		provider.BaseURL,
		nullableString(secret.EncryptedAPIKey),
		nullableString(secret.APIKeyEnvRef),
		nullableString(provider.Organization),
		nullableString(provider.Project),
		jsonString(provider.CustomHeaders),
		provider.Enabled,
		provider.RequestTimeoutMS,
		nullableString(provider.DefaultModel),
		nullableString(provider.FallbackModel),
		provider.HealthStatus,
		r.store.NowArg(now),
		r.store.NowArg(now),
	)
	return provider, err
}

func (r *SQLRepository) Update(ctx context.Context, provider Provider, secret *ProviderSecret) (Provider, error) {
	now := time.Now().UTC()
	provider.UpdatedAt = now
	args := []any{
		provider.Name,
		provider.BaseURL,
		nullableString(provider.Organization),
		nullableString(provider.Project),
		jsonString(provider.CustomHeaders),
		provider.Enabled,
		provider.RequestTimeoutMS,
		nullableString(provider.DefaultModel),
		nullableString(provider.FallbackModel),
		r.store.NowArg(now),
	}
	setSecret := ""
	whereWorkspacePlaceholder := 11
	whereIDPlaceholder := 12
	if secret != nil {
		setSecret = `, encrypted_api_key = ` + r.store.Placeholder(11) + `, api_key_env_ref = ` + r.store.Placeholder(12)
		args = append(args, nullableString(secret.EncryptedAPIKey), nullableString(secret.APIKeyEnvRef))
		whereWorkspacePlaceholder = 13
		whereIDPlaceholder = 14
	}
	args = append(args, provider.WorkspaceID, provider.ID)
	query := `UPDATE providers SET name = ` + r.store.Placeholder(1) + `, base_url = ` + r.store.Placeholder(2) +
		`, organization_header = ` + r.store.Placeholder(3) + `, project_header = ` + r.store.Placeholder(4) +
		`, custom_headers = ` + r.store.Placeholder(5) + `, enabled = ` + r.store.Placeholder(6) +
		`, request_timeout_ms = ` + r.store.Placeholder(7) + `, default_model = ` + r.store.Placeholder(8) +
		`, fallback_model = ` + r.store.Placeholder(9) + `, updated_at = ` + r.store.Placeholder(10) + setSecret +
		` WHERE workspace_id = ` + r.store.Placeholder(whereWorkspacePlaceholder) + ` AND id = ` + r.store.Placeholder(whereIDPlaceholder)
	result, err := r.store.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return Provider{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Provider{}, err
	}
	if affected == 0 {
		return Provider{}, ErrNotFound
	}
	return provider, nil
}

func (r *SQLRepository) Delete(ctx context.Context, workspaceID string, providerID string) error {
	query := `DELETE FROM providers WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	result, err := r.store.DB.ExecContext(ctx, query, workspaceID, providerID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLRepository) ReplaceModels(ctx context.Context, providerID string, modelIDs []string, source string) ([]Model, error) {
	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM provider_models WHERE provider_id = `+r.store.Placeholder(1)+` AND source = `+r.store.Placeholder(2), providerID, source); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	models := make([]Model, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		model := Model{
			ID:          id.New(),
			ProviderID:  providerID,
			ModelID:     modelID,
			DisplayName: modelID,
			Source:      source,
			Active:      true,
			RefreshedAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		query := `INSERT INTO provider_models (id, provider_id, model_id, display_name, source, active, refreshed_at, created_at, updated_at)
VALUES (` + placeholders(r.store, 9) + `)`
		if _, err := tx.ExecContext(ctx, query,
			model.ID, model.ProviderID, model.ModelID, model.DisplayName, model.Source, model.Active,
			r.store.NowArg(now), r.store.NowArg(now), r.store.NowArg(now),
		); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, tx.Commit()
}

func (r *SQLRepository) ListModels(ctx context.Context, providerID string) ([]Model, error) {
	query := `SELECT id, provider_id, model_id, display_name, source, active, refreshed_at, created_at, updated_at
FROM provider_models WHERE provider_id = ` + r.store.Placeholder(1) + ` ORDER BY model_id`
	rows, err := r.store.DB.QueryContext(ctx, query, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var models []Model
	for rows.Next() {
		model, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func (r *SQLRepository) UpdateHealth(ctx context.Context, workspaceID string, providerID string, status string, lastError string, checkedAt time.Time) error {
	query := `UPDATE providers SET health_status = ` + r.store.Placeholder(1) + `, last_error = ` + r.store.Placeholder(2) +
		`, last_health_check_at = ` + r.store.Placeholder(3) + `, updated_at = ` + r.store.Placeholder(4) +
		` WHERE workspace_id = ` + r.store.Placeholder(5) + ` AND id = ` + r.store.Placeholder(6)
	_, err := r.store.DB.ExecContext(ctx, query, status, nullableString(lastError), r.store.NowArg(checkedAt), r.store.NowArg(checkedAt), workspaceID, providerID)
	return err
}

func scanProvider(row rowScanner) (Provider, error) {
	provider, _, err := scanProviderFields(row, false)
	return provider, err
}

func scanProviderWithSecret(row rowScanner) (Provider, ProviderSecret, error) {
	return scanProviderFields(row, true)
}

func scanProviderFields(row rowScanner, withSecret bool) (Provider, ProviderSecret, error) {
	var provider Provider
	var encrypted sql.NullString
	var envRef sql.NullString
	var organization sql.NullString
	var project sql.NullString
	var headers string
	var defaultModel sql.NullString
	var fallbackModel sql.NullString
	var lastHealthRaw any
	var createdRaw any
	var updatedRaw any
	var lastError sql.NullString
	var err error
	if withSecret {
		err = row.Scan(&provider.ID, &provider.WorkspaceID, &provider.Name, &provider.BaseURL, &encrypted, &envRef,
			&organization, &project, &headers, &provider.Enabled, &provider.RequestTimeoutMS, &defaultModel,
			&fallbackModel, &provider.HealthStatus, &lastHealthRaw, &lastError, &createdRaw, &updatedRaw)
	} else {
		err = row.Scan(&provider.ID, &provider.WorkspaceID, &provider.Name, &provider.BaseURL, &envRef,
			&organization, &project, &headers, &provider.Enabled, &provider.RequestTimeoutMS, &defaultModel,
			&fallbackModel, &provider.HealthStatus, &lastHealthRaw, &lastError, &createdRaw, &updatedRaw)
	}
	if err != nil {
		return Provider{}, ProviderSecret{}, err
	}
	provider.APIKeyEnvRef = envRef.String
	provider.Organization = organization.String
	provider.Project = project.String
	provider.DefaultModel = defaultModel.String
	provider.FallbackModel = fallbackModel.String
	provider.LastError = lastError.String
	provider.CustomHeaders = parseStringMap(headers)
	lastHealthAt, err := nullableTime(lastHealthRaw)
	if err != nil {
		return Provider{}, ProviderSecret{}, err
	}
	createdAt, err := database.ParseTime(createdRaw)
	if err != nil {
		return Provider{}, ProviderSecret{}, err
	}
	updatedAt, err := database.ParseTime(updatedRaw)
	if err != nil {
		return Provider{}, ProviderSecret{}, err
	}
	provider.LastHealthCheckAt = lastHealthAt
	provider.CreatedAt = createdAt
	provider.UpdatedAt = updatedAt
	return provider, ProviderSecret{EncryptedAPIKey: encrypted.String, APIKeyEnvRef: envRef.String}, nil
}

func scanModel(row rowScanner) (Model, error) {
	var model Model
	var displayName sql.NullString
	var refreshedRaw any
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&model.ID, &model.ProviderID, &model.ModelID, &displayName, &model.Source, &model.Active, &refreshedRaw, &createdRaw, &updatedRaw); err != nil {
		return Model{}, err
	}
	model.DisplayName = displayName.String
	refreshedAt, err := database.ParseTime(refreshedRaw)
	if err != nil {
		return Model{}, err
	}
	createdAt, err := database.ParseTime(createdRaw)
	if err != nil {
		return Model{}, err
	}
	updatedAt, err := database.ParseTime(updatedRaw)
	if err != nil {
		return Model{}, err
	}
	model.RefreshedAt = refreshedAt
	model.CreatedAt = createdAt
	model.UpdatedAt = updatedAt
	return model, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func nullableTime(value any) (*time.Time, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		if strings.TrimSpace(string(typed)) == "" {
			return nil, nil
		}
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
	}
	parsed, err := database.ParseTime(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseStringMap(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return map[string]string{}
	}
	return values
}

func jsonString(values map[string]string) string {
	if values == nil {
		values = map[string]string{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func placeholders(store *database.Store, count int) string {
	values := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		values = append(values, store.Placeholder(i))
	}
	return strings.Join(values, ", ")
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
