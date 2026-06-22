package replies

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/isyuricunha/nostos/internal/database"
	"github.com/isyuricunha/nostos/internal/id"
)

var ErrNotFound = errors.New("reply record not found")

type Repository interface {
	ListPresets(ctx context.Context, workspaceID string) ([]Preset, error)
	GetPreset(ctx context.Context, workspaceID string, presetID string) (Preset, error)
	CreatePreset(ctx context.Context, preset Preset) (Preset, error)
	UpdatePreset(ctx context.Context, preset Preset) (Preset, error)
	DeletePreset(ctx context.Context, workspaceID string, presetID string) error
	HasPreset(ctx context.Context, workspaceID string, name string) (bool, error)
	Workspaces(ctx context.Context) ([]string, error)
	GetSourceMessage(ctx context.Context, workspaceID string, userID string, messageID string) (SourceMessage, error)
	CreateDraft(ctx context.Context, draft Draft) (Draft, error)
	ListDrafts(ctx context.Context, workspaceID string, userID string, sourceMessageID string) ([]Draft, error)
}

type SQLRepository struct {
	store *database.Store
}

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) ListPresets(ctx context.Context, workspaceID string) ([]Preset, error) {
	rows, err := r.store.DB.QueryContext(ctx, presetSelect(r.store)+` WHERE workspace_id = `+r.store.Placeholder(1)+` ORDER BY sort_order, name`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var presets []Preset
	for rows.Next() {
		preset, err := scanPreset(rows)
		if err != nil {
			return nil, err
		}
		presets = append(presets, preset)
	}
	return presets, rows.Err()
}

func (r *SQLRepository) GetPreset(ctx context.Context, workspaceID string, presetID string) (Preset, error) {
	preset, err := scanPreset(r.store.DB.QueryRowContext(ctx, presetSelect(r.store)+` WHERE workspace_id = `+r.store.Placeholder(1)+` AND id = `+r.store.Placeholder(2), workspaceID, presetID))
	if errors.Is(err, sql.ErrNoRows) {
		return Preset{}, ErrNotFound
	}
	return preset, err
}

func (r *SQLRepository) CreatePreset(ctx context.Context, preset Preset) (Preset, error) {
	now := time.Now().UTC()
	preset.ID = id.New()
	preset.CreatedAt = now
	preset.UpdatedAt = now
	query := `INSERT INTO reply_presets (id, workspace_id, name, description, prompt_instruction, icon, sort_order, active, system_default, created_at, updated_at)
VALUES (` + placeholders(r.store, 11) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, preset.ID, preset.WorkspaceID, preset.Name, preset.Description, preset.PromptInstruction, preset.Icon,
		preset.SortOrder, preset.Active, preset.SystemDefault, r.store.NowArg(now), r.store.NowArg(now))
	return preset, err
}

func (r *SQLRepository) UpdatePreset(ctx context.Context, preset Preset) (Preset, error) {
	now := time.Now().UTC()
	preset.UpdatedAt = now
	query := `UPDATE reply_presets SET name = ` + r.store.Placeholder(1) + `, description = ` + r.store.Placeholder(2) +
		`, prompt_instruction = ` + r.store.Placeholder(3) + `, icon = ` + r.store.Placeholder(4) + `, sort_order = ` + r.store.Placeholder(5) +
		`, active = ` + r.store.Placeholder(6) + `, updated_at = ` + r.store.Placeholder(7) + ` WHERE workspace_id = ` +
		r.store.Placeholder(8) + ` AND id = ` + r.store.Placeholder(9)
	result, err := r.store.DB.ExecContext(ctx, query, preset.Name, preset.Description, preset.PromptInstruction, preset.Icon, preset.SortOrder,
		preset.Active, r.store.NowArg(now), preset.WorkspaceID, preset.ID)
	if err != nil {
		return Preset{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Preset{}, err
	}
	if affected == 0 {
		return Preset{}, ErrNotFound
	}
	return preset, nil
}

func (r *SQLRepository) DeletePreset(ctx context.Context, workspaceID string, presetID string) error {
	result, err := r.store.DB.ExecContext(ctx, `DELETE FROM reply_presets WHERE workspace_id = `+r.store.Placeholder(1)+` AND id = `+r.store.Placeholder(2)+` AND system_default = `+r.store.Placeholder(3), workspaceID, presetID, false)
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

func (r *SQLRepository) HasPreset(ctx context.Context, workspaceID string, name string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM reply_presets WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND name = ` + r.store.Placeholder(2)
	err := r.store.DB.QueryRowContext(ctx, query, workspaceID, name).Scan(&count)
	return count > 0, err
}

func (r *SQLRepository) Workspaces(ctx context.Context) ([]string, error) {
	rows, err := r.store.DB.QueryContext(ctx, "SELECT id FROM workspaces")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var workspaceID string
		if err := rows.Scan(&workspaceID); err != nil {
			return nil, err
		}
		ids = append(ids, workspaceID)
	}
	return ids, rows.Err()
}

func (r *SQLRepository) GetSourceMessage(ctx context.Context, workspaceID string, userID string, messageID string) (SourceMessage, error) {
	var message SourceMessage
	var providerID, model sql.NullString
	query := `SELECT m.id, m.content, m.provider_id, m.model FROM messages m JOIN conversations c ON c.id = m.conversation_id
WHERE c.workspace_id = ` + r.store.Placeholder(1) + ` AND c.owner_user_id = ` + r.store.Placeholder(2) + ` AND m.id = ` + r.store.Placeholder(3)
	err := r.store.DB.QueryRowContext(ctx, query, workspaceID, userID, messageID).Scan(&message.ID, &message.Content, &providerID, &model)
	if errors.Is(err, sql.ErrNoRows) {
		return SourceMessage{}, ErrNotFound
	}
	message.ProviderID = providerID.String
	message.Model = model.String
	return message, err
}

func (r *SQLRepository) CreateDraft(ctx context.Context, draft Draft) (Draft, error) {
	now := time.Now().UTC()
	draft.ID = id.New()
	draft.CreatedAt = now
	draft.UpdatedAt = now
	query := `INSERT INTO reply_drafts (id, workspace_id, source_message_id, preset_id, preset_name, custom_instruction, generated_draft, provider_id, model, created_at, updated_at)
VALUES (` + placeholders(r.store, 11) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, draft.ID, draft.WorkspaceID, draft.SourceMessageID, nullableString(draft.PresetID), draft.PresetName,
		draft.CustomInstruction, draft.GeneratedDraft, nullableString(draft.ProviderID), nullableString(draft.Model), r.store.NowArg(now), r.store.NowArg(now))
	return draft, err
}

func (r *SQLRepository) ListDrafts(ctx context.Context, workspaceID string, userID string, sourceMessageID string) ([]Draft, error) {
	args := []any{workspaceID, userID}
	query := draftSelect(r.store) + ` JOIN messages m ON m.id = rd.source_message_id JOIN conversations c ON c.id = m.conversation_id
WHERE rd.workspace_id = ` + r.store.Placeholder(1) + ` AND c.owner_user_id = ` + r.store.Placeholder(2)
	if strings.TrimSpace(sourceMessageID) != "" {
		args = append(args, sourceMessageID)
		query += ` AND rd.source_message_id = ` + r.store.Placeholder(3)
	}
	query += ` ORDER BY rd.created_at DESC LIMIT 50`
	rows, err := r.store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var drafts []Draft
	for rows.Next() {
		draft, err := scanDraft(rows)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, draft)
	}
	return drafts, rows.Err()
}

func presetSelect(store *database.Store) string {
	return `SELECT id, workspace_id, name, description, prompt_instruction, icon, sort_order, active, system_default, created_at, updated_at FROM reply_presets`
}

func draftSelect(store *database.Store) string {
	return `SELECT rd.id, rd.workspace_id, rd.source_message_id, rd.preset_id, rd.preset_name, rd.custom_instruction, rd.generated_draft, rd.provider_id, rd.model, rd.created_at, rd.updated_at FROM reply_drafts rd`
}

func scanPreset(row rowScanner) (Preset, error) {
	var preset Preset
	var createdRaw, updatedRaw any
	if err := row.Scan(&preset.ID, &preset.WorkspaceID, &preset.Name, &preset.Description, &preset.PromptInstruction, &preset.Icon,
		&preset.SortOrder, &preset.Active, &preset.SystemDefault, &createdRaw, &updatedRaw); err != nil {
		return Preset{}, err
	}
	var err error
	preset.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Preset{}, err
	}
	preset.UpdatedAt, err = database.ParseTime(updatedRaw)
	return preset, err
}

func scanDraft(row rowScanner) (Draft, error) {
	var draft Draft
	var presetID, providerID, model sql.NullString
	var createdRaw, updatedRaw any
	if err := row.Scan(&draft.ID, &draft.WorkspaceID, &draft.SourceMessageID, &presetID, &draft.PresetName, &draft.CustomInstruction,
		&draft.GeneratedDraft, &providerID, &model, &createdRaw, &updatedRaw); err != nil {
		return Draft{}, err
	}
	draft.PresetID = presetID.String
	draft.ProviderID = providerID.String
	draft.Model = model.String
	var err error
	draft.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Draft{}, err
	}
	draft.UpdatedAt, err = database.ParseTime(updatedRaw)
	return draft, err
}

type rowScanner interface{ Scan(dest ...any) error }

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
