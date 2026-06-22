package agents

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/id"
)

var ErrNotFound = errors.New("agent not found")

type Repository interface {
	List(ctx context.Context, workspaceID string) ([]Agent, error)
	Get(ctx context.Context, workspaceID string, agentID string) (Agent, error)
	Create(ctx context.Context, agent Agent) (Agent, error)
	Update(ctx context.Context, agent Agent) (Agent, error)
	Delete(ctx context.Context, workspaceID string, agentID string) error
	Workspaces(ctx context.Context) ([]string, error)
	HasDefault(ctx context.Context, workspaceID string) (bool, error)
}

type SQLRepository struct {
	store *database.Store
}

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) List(ctx context.Context, workspaceID string) ([]Agent, error) {
	query := agentSelect(r.store) + ` WHERE workspace_id = ` + r.store.Placeholder(1) + ` ORDER BY active DESC, name`
	rows, err := r.store.DB.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []Agent
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (r *SQLRepository) Get(ctx context.Context, workspaceID string, agentID string) (Agent, error) {
	query := agentSelect(r.store) + ` WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	agent, err := scanAgent(r.store.DB.QueryRowContext(ctx, query, workspaceID, agentID))
	if errors.Is(err, sql.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	return agent, err
}

func (r *SQLRepository) Create(ctx context.Context, agent Agent) (Agent, error) {
	now := time.Now().UTC()
	agent.ID = id.New()
	agent.CreatedAt = now
	agent.UpdatedAt = now
	query := `INSERT INTO agents (id, workspace_id, name, description, avatar, system_prompt, default_provider_id, default_model,
fallback_model, temperature, max_tool_iterations, memory_access_mode, tool_permission_default, active, created_at, updated_at)
VALUES (` + placeholders(r.store, 16) + `)`
	_, err := r.store.DB.ExecContext(ctx, query,
		agent.ID, agent.WorkspaceID, agent.Name, agent.Description, agent.Avatar, agent.SystemPrompt,
		nullableString(agent.DefaultProviderID), nullableString(agent.DefaultModel), nullableString(agent.FallbackModel),
		agent.Temperature, agent.MaxToolIterations, agent.MemoryAccessMode, agent.ToolPermissionDefault, agent.Active,
		r.store.NowArg(now), r.store.NowArg(now),
	)
	return agent, err
}

func (r *SQLRepository) Update(ctx context.Context, agent Agent) (Agent, error) {
	now := time.Now().UTC()
	agent.UpdatedAt = now
	query := `UPDATE agents SET name = ` + r.store.Placeholder(1) + `, description = ` + r.store.Placeholder(2) +
		`, avatar = ` + r.store.Placeholder(3) + `, system_prompt = ` + r.store.Placeholder(4) +
		`, default_provider_id = ` + r.store.Placeholder(5) + `, default_model = ` + r.store.Placeholder(6) +
		`, fallback_model = ` + r.store.Placeholder(7) + `, temperature = ` + r.store.Placeholder(8) +
		`, max_tool_iterations = ` + r.store.Placeholder(9) + `, memory_access_mode = ` + r.store.Placeholder(10) +
		`, tool_permission_default = ` + r.store.Placeholder(11) + `, active = ` + r.store.Placeholder(12) +
		`, updated_at = ` + r.store.Placeholder(13) + ` WHERE workspace_id = ` + r.store.Placeholder(14) + ` AND id = ` + r.store.Placeholder(15)
	result, err := r.store.DB.ExecContext(ctx, query, agent.Name, agent.Description, agent.Avatar, agent.SystemPrompt,
		nullableString(agent.DefaultProviderID), nullableString(agent.DefaultModel), nullableString(agent.FallbackModel),
		agent.Temperature, agent.MaxToolIterations, agent.MemoryAccessMode, agent.ToolPermissionDefault, agent.Active,
		r.store.NowArg(now), agent.WorkspaceID, agent.ID)
	if err != nil {
		return Agent{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Agent{}, err
	}
	if affected == 0 {
		return Agent{}, ErrNotFound
	}
	return agent, nil
}

func (r *SQLRepository) Delete(ctx context.Context, workspaceID string, agentID string) error {
	query := `DELETE FROM agents WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	result, err := r.store.DB.ExecContext(ctx, query, workspaceID, agentID)
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

func (r *SQLRepository) HasDefault(ctx context.Context, workspaceID string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM agents WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND name = ` + r.store.Placeholder(2)
	err := r.store.DB.QueryRowContext(ctx, query, workspaceID, DefaultAgentName).Scan(&count)
	return count > 0, err
}

func agentSelect(store *database.Store) string {
	return `SELECT id, workspace_id, name, description, avatar, system_prompt, default_provider_id, default_model, fallback_model,
temperature, max_tool_iterations, memory_access_mode, tool_permission_default, active, created_at, updated_at FROM agents`
}

func scanAgent(row rowScanner) (Agent, error) {
	var agent Agent
	var defaultProvider sql.NullString
	var defaultModel sql.NullString
	var fallbackModel sql.NullString
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&agent.ID, &agent.WorkspaceID, &agent.Name, &agent.Description, &agent.Avatar, &agent.SystemPrompt,
		&defaultProvider, &defaultModel, &fallbackModel, &agent.Temperature, &agent.MaxToolIterations, &agent.MemoryAccessMode,
		&agent.ToolPermissionDefault, &agent.Active, &createdRaw, &updatedRaw); err != nil {
		return Agent{}, err
	}
	agent.DefaultProviderID = defaultProvider.String
	agent.DefaultModel = defaultModel.String
	agent.FallbackModel = fallbackModel.String
	var err error
	agent.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Agent{}, err
	}
	agent.UpdatedAt, err = database.ParseTime(updatedRaw)
	if err != nil {
		return Agent{}, err
	}
	return agent, nil
}

type rowScanner interface {
	Scan(dest ...any) error
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
