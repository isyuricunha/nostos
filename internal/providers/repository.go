package providers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	UpsertRefreshedModels(ctx context.Context, workspaceID string, providerID string, modelIDs []string) ([]Model, error)
	ListModels(ctx context.Context, query ModelQuery) ([]Model, error)
	CreateManualModel(ctx context.Context, workspaceID string, input ModelInput) (Model, error)
	UpdateModel(ctx context.Context, workspaceID string, modelID string, patch ModelPatch) (Model, error)
	CleanupUnavailableModels(ctx context.Context, workspaceID string, providerID string) (int, error)
	TryStartModelRefresh(ctx context.Context, workspaceID string, providerID string, startedAt time.Time) (bool, error)
	FinishModelRefresh(ctx context.Context, workspaceID string, providerID string, status ModelRefreshStatus) error
	ModelRefreshStatus(ctx context.Context, workspaceID string, providerID string) (ModelRefreshStatus, error)
	ListModelRoles(ctx context.Context, workspaceID string) ([]ModelRoleBinding, error)
	ReplaceModelRoleBindings(ctx context.Context, workspaceID string, role string, refs []ModelRoleReference) ([]ModelRoleBinding, error)
	UpdateHealth(ctx context.Context, workspaceID string, providerID string, status string, lastError string, checkedAt time.Time) error
	UpdateHealthWithLatency(ctx context.Context, workspaceID string, providerID string, status string, lastError string, checkedAt time.Time, latencyMS int) error
	ListEnabledWithSecrets(ctx context.Context, limit int) ([]Provider, []ProviderSecret, error)
}

type SQLRepository struct {
	store *database.Store
}

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) List(ctx context.Context, workspaceID string) ([]Provider, error) {
	query := providerSelect(false) + ` FROM providers WHERE workspace_id = ` + r.store.Placeholder(1) + ` ORDER BY name`
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
	query := providerSelect(true) + ` FROM providers WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	row := r.store.DB.QueryRowContext(ctx, query, workspaceID, providerID)
	provider, secret, err := scanProviderWithSecret(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Provider{}, ProviderSecret{}, ErrNotFound
	}
	if err != nil {
		return Provider{}, ProviderSecret{}, err
	}
	if err := r.attachModelCounts(ctx, &provider); err != nil {
		return Provider{}, ProviderSecret{}, err
	}
	return provider, secret, nil
}

func (r *SQLRepository) Create(ctx context.Context, provider Provider, secret ProviderSecret) (Provider, error) {
	now := time.Now().UTC()
	provider.ID = id.New()
	provider.CreatedAt = now
	provider.UpdatedAt = now
	provider.HealthStatus = "unknown"
	provider.ModelRefreshState = "idle"
	query := `INSERT INTO providers (id, workspace_id, name, base_url, encrypted_api_key, api_key_env_ref, organization_header, project_header,
custom_headers, enabled, request_timeout_ms, default_model, fallback_model, health_status, model_refresh_state, created_at, updated_at)
VALUES (` + placeholders(r.store, 17) + `)`
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
		provider.ModelRefreshState,
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
	var workspaceID string
	query := `SELECT workspace_id FROM providers WHERE id = ` + r.store.Placeholder(1)
	if err := r.store.DB.QueryRowContext(ctx, query, providerID).Scan(&workspaceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if source == "api" {
		return r.UpsertRefreshedModels(ctx, workspaceID, providerID, modelIDs)
	}
	models := make([]Model, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		model, err := r.CreateManualModel(ctx, workspaceID, ModelInput{
			ProviderID:       providerID,
			ModelID:          modelID,
			DisplayName:      modelID,
			Enabled:          true,
			Available:        true,
			Capabilities:     []string{"chat"},
			CapabilitySource: "manual",
		})
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, nil
}

func (r *SQLRepository) UpsertRefreshedModels(ctx context.Context, workspaceID string, providerID string, modelIDs []string) ([]Model, error) {
	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	markMissing := `UPDATE provider_models SET active = ` + r.store.Placeholder(1) + `, available = ` + r.store.Placeholder(2) +
		`, updated_at = ` + r.store.Placeholder(3) + ` WHERE workspace_id = ` + r.store.Placeholder(4) +
		` AND provider_id = ` + r.store.Placeholder(5) + ` AND source = ` + r.store.Placeholder(6)
	if _, err := tx.ExecContext(ctx, markMissing, false, false, r.store.NowArg(now), workspaceID, providerID, "api"); err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	for _, raw := range modelIDs {
		modelID := strings.TrimSpace(raw)
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		displayName := friendlyModelName(modelID)
		capabilities := inferCapabilities(modelID)
		searchText := normalizedModelSearch("", displayName, modelID)
		if r.store.Dialect == database.Postgres {
			query := `INSERT INTO provider_models (id, workspace_id, provider_id, model_id, display_name, source, active, metadata, refreshed_at, enabled, manually_added, available, first_seen_at, last_seen_at, capabilities, capability_source, search_text, created_at, updated_at)
VALUES (` + placeholders(r.store, 19) + `)
ON CONFLICT (provider_id, model_id) DO UPDATE SET
display_name = EXCLUDED.display_name,
active = true,
available = true,
refreshed_at = EXCLUDED.refreshed_at,
last_seen_at = EXCLUDED.last_seen_at,
capabilities = EXCLUDED.capabilities,
capability_source = EXCLUDED.capability_source,
search_text = EXCLUDED.search_text,
last_error_category = NULL,
last_safe_error_message = NULL,
updated_at = EXCLUDED.updated_at`
			if _, err := tx.ExecContext(ctx, query,
				id.New(), workspaceID, providerID, modelID, displayName, "api", true, "{}", r.store.NowArg(now),
				true, false, true, r.store.NowArg(now), r.store.NowArg(now), jsonArray(capabilities), "heuristic", searchText,
				r.store.NowArg(now), r.store.NowArg(now),
			); err != nil {
				return nil, err
			}
			continue
		}
		query := `INSERT INTO provider_models (id, workspace_id, provider_id, model_id, display_name, source, active, metadata, refreshed_at, enabled, manually_added, available, first_seen_at, last_seen_at, capabilities, capability_source, search_text, created_at, updated_at)
VALUES (` + placeholders(r.store, 19) + `)
ON CONFLICT(provider_id, model_id) DO UPDATE SET
display_name = excluded.display_name,
active = 1,
available = 1,
refreshed_at = excluded.refreshed_at,
last_seen_at = excluded.last_seen_at,
capabilities = excluded.capabilities,
capability_source = excluded.capability_source,
search_text = excluded.search_text,
last_error_category = NULL,
last_safe_error_message = NULL,
updated_at = excluded.updated_at`
		if _, err := tx.ExecContext(ctx, query,
			id.New(), workspaceID, providerID, modelID, displayName, "api", true, "{}", r.store.NowArg(now),
			true, false, true, r.store.NowArg(now), r.store.NowArg(now), jsonArray(capabilities), "heuristic", searchText,
			r.store.NowArg(now), r.store.NowArg(now),
		); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ListModels(ctx, ModelQuery{WorkspaceID: workspaceID, ProviderID: providerID, IncludeUnavailable: true, Limit: len(seen) + 25})
}

func (r *SQLRepository) ListModels(ctx context.Context, query ModelQuery) ([]Model, error) {
	if query.Limit <= 0 {
		query.Limit = 500
	} else if query.Limit > 2000 {
		query.Limit = 2000
	}
	args := []any{query.WorkspaceID}
	where := []string{"pm.workspace_id = " + r.store.Placeholder(1)}
	if strings.TrimSpace(query.ProviderID) != "" {
		args = append(args, query.ProviderID)
		where = append(where, "pm.provider_id = "+r.store.Placeholder(len(args)))
	}
	if !query.IncludeUnavailable {
		args = append(args, true)
		where = append(where, "pm.enabled = "+r.store.Placeholder(len(args)))
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	if search != "" {
		args = append(args, "%"+search+"%")
		searchTextPlaceholder := r.store.Placeholder(len(args))
		args = append(args, "%"+search+"%")
		modelPlaceholder := r.store.Placeholder(len(args))
		args = append(args, "%"+search+"%")
		providerPlaceholder := r.store.Placeholder(len(args))
		where = append(where, "(pm.search_text LIKE "+searchTextPlaceholder+" OR lower(pm.model_id) LIKE "+modelPlaceholder+" OR lower(p.name) LIKE "+providerPlaceholder+")")
	}
	args = append(args, query.Limit, maxInt(query.Offset, 0))
	limitPlaceholder := r.store.Placeholder(len(args) - 1)
	offsetPlaceholder := r.store.Placeholder(len(args))
	sqlText := modelSelect() + ` FROM provider_models pm JOIN providers p ON p.id = pm.provider_id WHERE ` + strings.Join(where, " AND ") + ` ORDER BY p.name, pm.model_id LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder
	rows, err := r.store.DB.QueryContext(ctx, sqlText, args...)
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

func (r *SQLRepository) CreateManualModel(ctx context.Context, workspaceID string, input ModelInput) (Model, error) {
	provider, _, err := r.Get(ctx, workspaceID, input.ProviderID)
	if err != nil {
		return Model{}, err
	}
	modelID := strings.TrimSpace(input.ModelID)
	if modelID == "" {
		return Model{}, fmt.Errorf("%w: model_id is required", ErrInvalidInput)
	}
	now := time.Now().UTC()
	model := Model{
		ID:               id.New(),
		WorkspaceID:      workspaceID,
		ProviderID:       provider.ID,
		ProviderName:     provider.Name,
		ModelID:          modelID,
		DisplayName:      coalesceString(input.DisplayName, friendlyModelName(modelID)),
		Source:           "manual",
		Active:           true,
		Enabled:          input.Enabled,
		ManuallyAdded:    true,
		Available:        input.Available,
		Capabilities:     sanitizeCapabilities(input.Capabilities),
		CapabilitySource: coalesceString(input.CapabilitySource, "manual"),
		Metadata:         input.Metadata,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if len(model.Capabilities) == 0 {
		model.Capabilities = []string{"chat"}
	}
	model.FirstSeenAt = &now
	model.LastSeenAt = &now
	insert := `INSERT INTO provider_models (id, workspace_id, provider_id, model_id, display_name, source, active, metadata, refreshed_at, enabled, manually_added, available, first_seen_at, last_seen_at, capabilities, capability_source, search_text, created_at, updated_at)
VALUES (` + placeholders(r.store, 19) + `)
ON CONFLICT(provider_id, model_id) DO NOTHING`
	if r.store.Dialect == database.Postgres {
		insert = strings.ReplaceAll(insert, "ON CONFLICT(provider_id, model_id)", "ON CONFLICT (provider_id, model_id)")
	}
	_, err = r.store.DB.ExecContext(ctx, insert,
		model.ID, model.WorkspaceID, model.ProviderID, model.ModelID, model.DisplayName, model.Source, model.Active,
		jsonAny(model.Metadata), r.store.NowArg(now), model.Enabled, model.ManuallyAdded, model.Available, r.store.NowArg(now),
		r.store.NowArg(now), jsonArray(model.Capabilities), model.CapabilitySource, normalizedModelSearch(provider.Name, model.DisplayName, model.ModelID),
		r.store.NowArg(now), r.store.NowArg(now),
	)
	if err != nil {
		return Model{}, err
	}
	update := `UPDATE provider_models SET display_name = ` + r.store.Placeholder(1) + `, active = ` + r.store.Placeholder(2) +
		`, enabled = ` + r.store.Placeholder(3) + `, manually_added = ` + r.store.Placeholder(4) + `, available = ` + r.store.Placeholder(5) +
		`, capabilities = ` + r.store.Placeholder(6) + `, capability_source = ` + r.store.Placeholder(7) + `, search_text = ` + r.store.Placeholder(8) +
		`, updated_at = ` + r.store.Placeholder(9) + ` WHERE workspace_id = ` + r.store.Placeholder(10) +
		` AND provider_id = ` + r.store.Placeholder(11) + ` AND model_id = ` + r.store.Placeholder(12)
	if _, err := r.store.DB.ExecContext(ctx, update, model.DisplayName, model.Active, model.Enabled, model.ManuallyAdded, model.Available,
		jsonArray(model.Capabilities), model.CapabilitySource, normalizedModelSearch(provider.Name, model.DisplayName, model.ModelID),
		r.store.NowArg(now), workspaceID, model.ProviderID, model.ModelID); err != nil {
		return Model{}, err
	}
	items, err := r.ListModels(ctx, ModelQuery{WorkspaceID: workspaceID, ProviderID: input.ProviderID, Search: modelID, IncludeUnavailable: true, Limit: 1})
	if err != nil {
		return Model{}, err
	}
	if len(items) == 0 {
		return Model{}, ErrNotFound
	}
	return items[0], nil
}

func (r *SQLRepository) UpdateModel(ctx context.Context, workspaceID string, modelID string, patch ModelPatch) (Model, error) {
	existing, err := r.modelByID(ctx, workspaceID, modelID)
	if err != nil {
		return Model{}, err
	}
	displayName := existing.DisplayName
	if patch.DisplayName != nil {
		displayName = strings.TrimSpace(*patch.DisplayName)
	}
	enabled := existing.Enabled
	if patch.Enabled != nil {
		enabled = *patch.Enabled
	}
	available := existing.Available
	if patch.Available != nil {
		available = *patch.Available
	}
	capabilities := existing.Capabilities
	if patch.Capabilities != nil {
		capabilities = sanitizeCapabilities(patch.Capabilities)
	}
	capabilitySource := coalesceString(patch.CapabilitySource, existing.CapabilitySource)
	errorCategory := existing.LastErrorCategory
	if patch.LastErrorCategory != nil {
		errorCategory = strings.TrimSpace(*patch.LastErrorCategory)
	}
	errorMessage := existing.LastSafeErrorMessage
	if patch.LastSafeErrorMessage != nil {
		errorMessage = strings.TrimSpace(*patch.LastSafeErrorMessage)
	}
	now := time.Now().UTC()
	query := `UPDATE provider_models SET display_name = ` + r.store.Placeholder(1) + `, enabled = ` + r.store.Placeholder(2) +
		`, available = ` + r.store.Placeholder(3) + `, capabilities = ` + r.store.Placeholder(4) + `, capability_source = ` + r.store.Placeholder(5) +
		`, last_error_category = ` + r.store.Placeholder(6) + `, last_safe_error_message = ` + r.store.Placeholder(7) + `, search_text = ` + r.store.Placeholder(8) +
		`, updated_at = ` + r.store.Placeholder(9) + ` WHERE workspace_id = ` + r.store.Placeholder(10) + ` AND id = ` + r.store.Placeholder(11)
	result, err := r.store.DB.ExecContext(ctx, query, nullableString(displayName), enabled, available, jsonArray(capabilities), capabilitySource,
		nullableString(errorCategory), nullableString(errorMessage), normalizedModelSearch(existing.ProviderName, displayName, existing.ModelID),
		r.store.NowArg(now), workspaceID, modelID)
	if err != nil {
		return Model{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Model{}, err
	}
	if affected == 0 {
		return Model{}, ErrNotFound
	}
	return r.modelByID(ctx, workspaceID, modelID)
}

func (r *SQLRepository) CleanupUnavailableModels(ctx context.Context, workspaceID string, providerID string) (int, error) {
	query := `DELETE FROM provider_models WHERE workspace_id = ` + r.store.Placeholder(1) +
		` AND provider_id = ` + r.store.Placeholder(2) + ` AND available = ` + r.store.Placeholder(3) + ` AND manually_added = ` + r.store.Placeholder(4)
	result, err := r.store.DB.ExecContext(ctx, query, workspaceID, providerID, false, false)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	return int(count), err
}

func (r *SQLRepository) TryStartModelRefresh(ctx context.Context, workspaceID string, providerID string, startedAt time.Time) (bool, error) {
	query := `UPDATE providers SET model_refresh_state = ` + r.store.Placeholder(1) + `, model_refresh_started_at = ` + r.store.Placeholder(2) +
		`, model_refresh_completed_at = NULL, model_refresh_duration_ms = NULL, model_refresh_error_category = NULL, model_refresh_error_message = NULL, updated_at = ` + r.store.Placeholder(3) +
		` WHERE workspace_id = ` + r.store.Placeholder(4) + ` AND id = ` + r.store.Placeholder(5) + ` AND model_refresh_state <> ` + r.store.Placeholder(6)
	result, err := r.store.DB.ExecContext(ctx, query, "refreshing", r.store.NowArg(startedAt), r.store.NowArg(startedAt), workspaceID, providerID, "refreshing")
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if count == 0 {
		var exists string
		get := `SELECT id FROM providers WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
		if scanErr := r.store.DB.QueryRowContext(ctx, get, workspaceID, providerID).Scan(&exists); errors.Is(scanErr, sql.ErrNoRows) {
			return false, ErrNotFound
		}
	}
	return count > 0, nil
}

func (r *SQLRepository) FinishModelRefresh(ctx context.Context, workspaceID string, providerID string, status ModelRefreshStatus) error {
	completedAt := time.Now().UTC()
	if status.CompletedAt != nil {
		completedAt = *status.CompletedAt
	}
	query := `UPDATE providers SET model_refresh_state = ` + r.store.Placeholder(1) + `, model_refresh_completed_at = ` + r.store.Placeholder(2) +
		`, model_refresh_duration_ms = ` + r.store.Placeholder(3) + `, model_refresh_error_category = ` + r.store.Placeholder(4) +
		`, model_refresh_error_message = ` + r.store.Placeholder(5) + `, updated_at = ` + r.store.Placeholder(6) +
		` WHERE workspace_id = ` + r.store.Placeholder(7) + ` AND id = ` + r.store.Placeholder(8)
	result, err := r.store.DB.ExecContext(ctx, query, status.State, r.store.NowArg(completedAt), nullInt(status.DurationMS),
		nullableString(status.ErrorCategory), nullableString(status.ErrorMessage), r.store.NowArg(completedAt), workspaceID, providerID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *SQLRepository) ModelRefreshStatus(ctx context.Context, workspaceID string, providerID string) (ModelRefreshStatus, error) {
	query := `SELECT id, model_refresh_state, model_refresh_started_at, model_refresh_completed_at, model_refresh_duration_ms,
model_refresh_error_category, model_refresh_error_message
FROM providers WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	var status ModelRefreshStatus
	var startedRaw any
	var completedRaw any
	var duration sql.NullInt64
	var errorCategory sql.NullString
	var errorMessage sql.NullString
	err := r.store.DB.QueryRowContext(ctx, query, workspaceID, providerID).Scan(&status.ProviderID, &status.State, &startedRaw, &completedRaw, &duration, &errorCategory, &errorMessage)
	if errors.Is(err, sql.ErrNoRows) {
		return ModelRefreshStatus{}, ErrNotFound
	}
	if err != nil {
		return ModelRefreshStatus{}, err
	}
	startedAt, err := nullableTime(startedRaw)
	if err != nil {
		return ModelRefreshStatus{}, err
	}
	completedAt, err := nullableTime(completedRaw)
	if err != nil {
		return ModelRefreshStatus{}, err
	}
	status.StartedAt = startedAt
	status.CompletedAt = completedAt
	status.DurationMS = int(duration.Int64)
	status.ErrorCategory = errorCategory.String
	status.ErrorMessage = errorMessage.String
	if err := r.attachRefreshCounts(ctx, workspaceID, providerID, &status); err != nil {
		return ModelRefreshStatus{}, err
	}
	return status, nil
}

func (r *SQLRepository) ListModelRoles(ctx context.Context, workspaceID string) ([]ModelRoleBinding, error) {
	query := `SELECT mrb.id, mrb.workspace_id, mrb.role, mrb.position, mrb.provider_id, p.name, mrb.model_id, mrb.created_at, mrb.updated_at
FROM model_role_bindings mrb JOIN providers p ON p.id = mrb.provider_id
WHERE mrb.workspace_id = ` + r.store.Placeholder(1) + ` ORDER BY mrb.role, mrb.position`
	rows, err := r.store.DB.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []ModelRoleBinding
	for rows.Next() {
		role, err := scanModelRole(rows)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *SQLRepository) ReplaceModelRoleBindings(ctx context.Context, workspaceID string, role string, refs []ModelRoleReference) ([]ModelRoleBinding, error) {
	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	deleteQuery := `DELETE FROM model_role_bindings WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND role = ` + r.store.Placeholder(2)
	if _, err := tx.ExecContext(ctx, deleteQuery, workspaceID, role); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	for position, ref := range refs {
		providerID := strings.TrimSpace(ref.ProviderID)
		modelID := strings.TrimSpace(ref.ModelID)
		if providerID == "" || modelID == "" {
			continue
		}
		insert := `INSERT INTO model_role_bindings (id, workspace_id, role, position, provider_id, model_id, created_at, updated_at)
VALUES (` + placeholders(r.store, 8) + `)`
		if _, err := tx.ExecContext(ctx, insert, id.New(), workspaceID, role, position, providerID, modelID, r.store.NowArg(now), r.store.NowArg(now)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ListModelRoles(ctx, workspaceID)
}

func (r *SQLRepository) UpdateHealth(ctx context.Context, workspaceID string, providerID string, status string, lastError string, checkedAt time.Time) error {
	return r.UpdateHealthWithLatency(ctx, workspaceID, providerID, status, lastError, checkedAt, 0)
}

func (r *SQLRepository) UpdateHealthWithLatency(ctx context.Context, workspaceID string, providerID string, status string, lastError string, checkedAt time.Time, latencyMS int) error {
	query := `UPDATE providers SET health_status = ` + r.store.Placeholder(1) + `, last_error = ` + r.store.Placeholder(2) +
		`, last_health_check_at = ` + r.store.Placeholder(3) + `, health_latency_ms = ` + r.store.Placeholder(4) +
		`, updated_at = ` + r.store.Placeholder(5) + ` WHERE workspace_id = ` + r.store.Placeholder(6) + ` AND id = ` + r.store.Placeholder(7)
	_, err := r.store.DB.ExecContext(ctx, query, status, nullableString(lastError), r.store.NowArg(checkedAt), nullInt(latencyMS), r.store.NowArg(checkedAt), workspaceID, providerID)
	return err
}

func (r *SQLRepository) ListEnabledWithSecrets(ctx context.Context, limit int) ([]Provider, []ProviderSecret, error) {
	if limit <= 0 {
		limit = 25
	}
	query := providerSelect(true) + ` FROM providers WHERE enabled = ` + r.store.Placeholder(1) + ` ORDER BY updated_at ASC LIMIT ` + r.store.Placeholder(2)
	rows, err := r.store.DB.QueryContext(ctx, query, true, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var providers []Provider
	var secrets []ProviderSecret
	for rows.Next() {
		provider, secret, err := scanProviderWithSecret(rows)
		if err != nil {
			return nil, nil, err
		}
		providers = append(providers, provider)
		secrets = append(secrets, secret)
	}
	return providers, secrets, rows.Err()
}

func (r *SQLRepository) attachModelCounts(ctx context.Context, provider *Provider) error {
	if provider == nil || provider.ID == "" {
		return nil
	}
	var total int
	var available int
	query := `SELECT COUNT(*), COALESCE(SUM(CASE WHEN available = ` + r.store.Placeholder(1) + ` THEN 1 ELSE 0 END), 0) FROM provider_models WHERE provider_id = ` + r.store.Placeholder(2)
	if err := r.store.DB.QueryRowContext(ctx, query, true, provider.ID).Scan(&total, &available); err != nil {
		return err
	}
	provider.ModelCount = total
	provider.AvailableModelCount = available
	provider.UnavailableModelCount = total - available
	return nil
}

func (r *SQLRepository) attachRefreshCounts(ctx context.Context, workspaceID string, providerID string, status *ModelRefreshStatus) error {
	var total int
	var available int
	query := `SELECT COUNT(*), COALESCE(SUM(CASE WHEN available = ` + r.store.Placeholder(1) + ` THEN 1 ELSE 0 END), 0) FROM provider_models WHERE workspace_id = ` + r.store.Placeholder(2) + ` AND provider_id = ` + r.store.Placeholder(3)
	if err := r.store.DB.QueryRowContext(ctx, query, true, workspaceID, providerID).Scan(&total, &available); err != nil {
		return err
	}
	status.CachedModelCount = total
	status.AvailableModelCount = available
	status.UnavailableModelCount = total - available
	return nil
}

func (r *SQLRepository) modelByID(ctx context.Context, workspaceID string, modelID string) (Model, error) {
	query := modelSelect() + ` FROM provider_models pm JOIN providers p ON p.id = pm.provider_id WHERE pm.workspace_id = ` + r.store.Placeholder(1) + ` AND pm.id = ` + r.store.Placeholder(2)
	row := r.store.DB.QueryRowContext(ctx, query, workspaceID, modelID)
	model, err := scanModel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, ErrNotFound
	}
	return model, err
}

func providerSelect(withSecret bool) string {
	secret := ""
	if withSecret {
		secret = ", encrypted_api_key"
	}
	return `SELECT id, workspace_id, name, base_url` + secret + `, api_key_env_ref, organization_header, project_header, custom_headers,
enabled, request_timeout_ms, default_model, fallback_model, health_status, last_health_check_at, health_latency_ms, last_error,
model_refresh_state, model_refresh_started_at, model_refresh_completed_at, model_refresh_duration_ms, model_refresh_error_category, model_refresh_error_message,
created_at, updated_at`
}

func modelSelect() string {
	return `SELECT pm.id, pm.workspace_id, pm.provider_id, p.name, pm.model_id, pm.display_name, pm.source, pm.active, pm.enabled,
pm.manually_added, pm.available, pm.metadata, pm.refreshed_at, pm.first_seen_at, pm.last_seen_at, pm.last_successful_probe_at,
pm.last_failed_probe_at, pm.last_error_category, pm.last_safe_error_message, pm.capabilities, pm.capability_source, pm.created_at, pm.updated_at`
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
	var refreshStartedRaw any
	var refreshCompletedRaw any
	var latency sql.NullInt64
	var refreshDuration sql.NullInt64
	var createdRaw any
	var updatedRaw any
	var lastError sql.NullString
	var refreshErrorCategory sql.NullString
	var refreshErrorMessage sql.NullString
	var err error
	if withSecret {
		err = row.Scan(&provider.ID, &provider.WorkspaceID, &provider.Name, &provider.BaseURL, &encrypted, &envRef,
			&organization, &project, &headers, &provider.Enabled, &provider.RequestTimeoutMS, &defaultModel,
			&fallbackModel, &provider.HealthStatus, &lastHealthRaw, &latency, &lastError, &provider.ModelRefreshState,
			&refreshStartedRaw, &refreshCompletedRaw, &refreshDuration, &refreshErrorCategory, &refreshErrorMessage, &createdRaw, &updatedRaw)
	} else {
		err = row.Scan(&provider.ID, &provider.WorkspaceID, &provider.Name, &provider.BaseURL, &envRef,
			&organization, &project, &headers, &provider.Enabled, &provider.RequestTimeoutMS, &defaultModel,
			&fallbackModel, &provider.HealthStatus, &lastHealthRaw, &latency, &lastError, &provider.ModelRefreshState,
			&refreshStartedRaw, &refreshCompletedRaw, &refreshDuration, &refreshErrorCategory, &refreshErrorMessage, &createdRaw, &updatedRaw)
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
	provider.HealthLatencyMS = int(latency.Int64)
	provider.ModelRefreshDurationMS = int(refreshDuration.Int64)
	provider.ModelRefreshErrorCategory = refreshErrorCategory.String
	provider.ModelRefreshErrorMessage = refreshErrorMessage.String
	provider.CustomHeaders = parseStringMap(headers)
	lastHealthAt, err := nullableTime(lastHealthRaw)
	if err != nil {
		return Provider{}, ProviderSecret{}, err
	}
	refreshStartedAt, err := nullableTime(refreshStartedRaw)
	if err != nil {
		return Provider{}, ProviderSecret{}, err
	}
	refreshCompletedAt, err := nullableTime(refreshCompletedRaw)
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
	provider.ModelRefreshStartedAt = refreshStartedAt
	provider.ModelRefreshCompletedAt = refreshCompletedAt
	provider.CreatedAt = createdAt
	provider.UpdatedAt = updatedAt
	if provider.ModelRefreshState == "" {
		provider.ModelRefreshState = "idle"
	}
	return provider, ProviderSecret{EncryptedAPIKey: encrypted.String, APIKeyEnvRef: envRef.String}, nil
}

func scanModel(row rowScanner) (Model, error) {
	var model Model
	var displayName sql.NullString
	var metadata string
	var refreshedRaw any
	var firstSeenRaw any
	var lastSeenRaw any
	var lastSuccessfulProbeRaw any
	var lastFailedProbeRaw any
	var lastErrorCategory sql.NullString
	var lastSafeErrorMessage sql.NullString
	var capabilities string
	var capabilitySource sql.NullString
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&model.ID, &model.WorkspaceID, &model.ProviderID, &model.ProviderName, &model.ModelID, &displayName,
		&model.Source, &model.Active, &model.Enabled, &model.ManuallyAdded, &model.Available, &metadata, &refreshedRaw,
		&firstSeenRaw, &lastSeenRaw, &lastSuccessfulProbeRaw, &lastFailedProbeRaw, &lastErrorCategory, &lastSafeErrorMessage,
		&capabilities, &capabilitySource, &createdRaw, &updatedRaw); err != nil {
		return Model{}, err
	}
	model.DisplayName = displayName.String
	model.Metadata = parseAnyMap(metadata)
	model.Capabilities = parseStringArray(capabilities)
	model.CapabilitySource = capabilitySource.String
	model.LastErrorCategory = lastErrorCategory.String
	model.LastSafeErrorMessage = lastSafeErrorMessage.String
	refreshedAt, err := database.ParseTime(refreshedRaw)
	if err != nil {
		return Model{}, err
	}
	firstSeenAt, err := nullableTime(firstSeenRaw)
	if err != nil {
		return Model{}, err
	}
	lastSeenAt, err := nullableTime(lastSeenRaw)
	if err != nil {
		return Model{}, err
	}
	lastSuccessfulProbeAt, err := nullableTime(lastSuccessfulProbeRaw)
	if err != nil {
		return Model{}, err
	}
	lastFailedProbeAt, err := nullableTime(lastFailedProbeRaw)
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
	model.FirstSeenAt = firstSeenAt
	model.LastSeenAt = lastSeenAt
	model.LastSuccessfulProbeAt = lastSuccessfulProbeAt
	model.LastFailedProbeAt = lastFailedProbeAt
	model.CreatedAt = createdAt
	model.UpdatedAt = updatedAt
	return model, nil
}

func scanModelRole(row rowScanner) (ModelRoleBinding, error) {
	var role ModelRoleBinding
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&role.ID, &role.WorkspaceID, &role.Role, &role.Position, &role.ProviderID, &role.ProviderName, &role.ModelID, &createdRaw, &updatedRaw); err != nil {
		return ModelRoleBinding{}, err
	}
	createdAt, err := database.ParseTime(createdRaw)
	if err != nil {
		return ModelRoleBinding{}, err
	}
	updatedAt, err := database.ParseTime(updatedRaw)
	if err != nil {
		return ModelRoleBinding{}, err
	}
	role.CreatedAt = createdAt
	role.UpdatedAt = updatedAt
	return role, nil
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

func parseAnyMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return map[string]any{}
	}
	return values
}

func parseStringArray(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
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

func jsonAny(values map[string]any) string {
	if values == nil {
		values = map[string]any{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func jsonArray(values []string) string {
	if values == nil {
		values = []string{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "[]"
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

func nullInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
