package mcp

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

var ErrNotFound = errors.New("mcp record not found")

type Repository interface {
	ListServers(ctx context.Context, workspaceID string) ([]Server, error)
	GetServer(ctx context.Context, workspaceID string, serverID string) (Server, ServerSecret, error)
	CreateServer(ctx context.Context, server Server, secret ServerSecret) (Server, error)
	UpdateServer(ctx context.Context, server Server, secret ServerSecret) (Server, error)
	DeleteServer(ctx context.Context, workspaceID string, serverID string) error
	ReplaceTools(ctx context.Context, serverID string, tools []DiscoveredTool) ([]Tool, error)
	ListTools(ctx context.Context, workspaceID string, serverID string) ([]Tool, error)
	UpdateToolPermission(ctx context.Context, workspaceID string, toolID string, mode string) error
	UpdateServerHealth(ctx context.Context, workspaceID string, serverID string, status string, lastError string, connectedAt *time.Time) error
}

type SQLRepository struct {
	store *database.Store
}

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) ListServers(ctx context.Context, workspaceID string) ([]Server, error) {
	query := `SELECT id, workspace_id, name, description, transport_type, command, arguments, working_directory, encrypted_environment,
http_url, encrypted_http_headers, enabled, startup_timeout_ms, request_timeout_ms, health_status, last_error, last_connected_at, created_at, updated_at
FROM mcp_servers WHERE workspace_id = ` + r.store.Placeholder(1) + ` ORDER BY name`
	rows, err := r.store.DB.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []Server
	for rows.Next() {
		server, _, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

func (r *SQLRepository) GetServer(ctx context.Context, workspaceID string, serverID string) (Server, ServerSecret, error) {
	query := `SELECT id, workspace_id, name, description, transport_type, command, arguments, working_directory, encrypted_environment,
http_url, encrypted_http_headers, enabled, startup_timeout_ms, request_timeout_ms, health_status, last_error, last_connected_at, created_at, updated_at
FROM mcp_servers WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	server, secret, err := scanServer(r.store.DB.QueryRowContext(ctx, query, workspaceID, serverID))
	if errors.Is(err, sql.ErrNoRows) {
		return Server{}, ServerSecret{}, ErrNotFound
	}
	return server, secret, err
}

func (r *SQLRepository) CreateServer(ctx context.Context, server Server, secret ServerSecret) (Server, error) {
	now := time.Now().UTC()
	server.ID = id.New()
	server.HealthStatus = "unknown"
	server.CreatedAt = now
	server.UpdatedAt = now
	query := `INSERT INTO mcp_servers (id, workspace_id, name, description, transport_type, command, arguments, working_directory,
encrypted_environment, http_url, encrypted_http_headers, enabled, startup_timeout_ms, request_timeout_ms, health_status, created_at, updated_at)
VALUES (` + placeholders(r.store, 17) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, server.ID, server.WorkspaceID, server.Name, server.Description, server.TransportType,
		nullableString(server.Command), jsonArray(server.Arguments), nullableString(server.WorkingDirectory), jsonMap(secret.Environment),
		nullableString(server.HTTPURL), jsonMap(secret.HTTPHeaders), server.Enabled, server.StartupTimeoutMS, server.RequestTimeoutMS,
		server.HealthStatus, r.store.NowArg(now), r.store.NowArg(now))
	return server, err
}

func (r *SQLRepository) UpdateServer(ctx context.Context, server Server, secret ServerSecret) (Server, error) {
	now := time.Now().UTC()
	query := `UPDATE mcp_servers SET name = ` + r.store.Placeholder(1) + `, description = ` + r.store.Placeholder(2) +
		`, transport_type = ` + r.store.Placeholder(3) + `, command = ` + r.store.Placeholder(4) + `, arguments = ` + r.store.Placeholder(5) +
		`, working_directory = ` + r.store.Placeholder(6) + `, encrypted_environment = ` + r.store.Placeholder(7) +
		`, http_url = ` + r.store.Placeholder(8) + `, encrypted_http_headers = ` + r.store.Placeholder(9) + `, enabled = ` + r.store.Placeholder(10) +
		`, startup_timeout_ms = ` + r.store.Placeholder(11) + `, request_timeout_ms = ` + r.store.Placeholder(12) +
		`, updated_at = ` + r.store.Placeholder(13) + ` WHERE workspace_id = ` + r.store.Placeholder(14) + ` AND id = ` + r.store.Placeholder(15)
	result, err := r.store.DB.ExecContext(ctx, query, server.Name, server.Description, server.TransportType, nullableString(server.Command),
		jsonArray(server.Arguments), nullableString(server.WorkingDirectory), jsonMap(secret.Environment), nullableString(server.HTTPURL),
		jsonMap(secret.HTTPHeaders), server.Enabled, server.StartupTimeoutMS, server.RequestTimeoutMS, r.store.NowArg(now), server.WorkspaceID, server.ID)
	if err != nil {
		return Server{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Server{}, err
	}
	if affected == 0 {
		return Server{}, ErrNotFound
	}
	server.UpdatedAt = now
	return server, nil
}

func (r *SQLRepository) DeleteServer(ctx context.Context, workspaceID string, serverID string) error {
	query := `DELETE FROM mcp_servers WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	result, err := r.store.DB.ExecContext(ctx, query, workspaceID, serverID)
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

func (r *SQLRepository) ReplaceTools(ctx context.Context, serverID string, discovered []DiscoveredTool) ([]Tool, error) {
	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM mcp_tools WHERE server_id = `+r.store.Placeholder(1), serverID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	tools := make([]Tool, 0, len(discovered))
	for _, item := range discovered {
		schema, _ := json.Marshal(item.InputSchema)
		tool := Tool{ID: id.New(), ServerID: serverID, Name: item.Name, Description: item.Description, InputSchema: string(schema), PermissionMode: "ask", DiscoveredAt: now, UpdatedAt: now}
		query := `INSERT INTO mcp_tools (id, server_id, name, description, input_schema, permission_mode, discovered_at, updated_at)
VALUES (` + placeholders(r.store, 8) + `)`
		if _, err := tx.ExecContext(ctx, query, tool.ID, tool.ServerID, tool.Name, tool.Description, tool.InputSchema, tool.PermissionMode, r.store.NowArg(now), r.store.NowArg(now)); err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, tx.Commit()
}

func (r *SQLRepository) ListTools(ctx context.Context, workspaceID string, serverID string) ([]Tool, error) {
	args := []any{workspaceID}
	query := `SELECT t.id, t.server_id, t.name, t.description, t.input_schema, t.permission_mode, t.discovered_at, t.updated_at
FROM mcp_tools t JOIN mcp_servers s ON s.id = t.server_id WHERE s.workspace_id = ` + r.store.Placeholder(1)
	if strings.TrimSpace(serverID) != "" {
		args = append(args, serverID)
		query += ` AND t.server_id = ` + r.store.Placeholder(2)
	}
	query += ` ORDER BY t.name`
	rows, err := r.store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tools []Tool
	for rows.Next() {
		tool, err := scanTool(rows)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, rows.Err()
}

func (r *SQLRepository) UpdateToolPermission(ctx context.Context, workspaceID string, toolID string, mode string) error {
	query := `UPDATE mcp_tools SET permission_mode = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
		` WHERE id = ` + r.store.Placeholder(3) + ` AND server_id IN (SELECT id FROM mcp_servers WHERE workspace_id = ` + r.store.Placeholder(4) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, mode, r.store.NowArg(time.Now().UTC()), toolID, workspaceID)
	return err
}

func (r *SQLRepository) UpdateServerHealth(ctx context.Context, workspaceID string, serverID string, status string, lastError string, connectedAt *time.Time) error {
	query := `UPDATE mcp_servers SET health_status = ` + r.store.Placeholder(1) + `, last_error = ` + r.store.Placeholder(2) +
		`, last_connected_at = ` + r.store.Placeholder(3) + `, updated_at = ` + r.store.Placeholder(4) +
		` WHERE workspace_id = ` + r.store.Placeholder(5) + ` AND id = ` + r.store.Placeholder(6)
	_, err := r.store.DB.ExecContext(ctx, query, status, nullableString(lastError), timePtrArg(r.store, connectedAt), r.store.NowArg(time.Now().UTC()), workspaceID, serverID)
	return err
}

func scanServer(row rowScanner) (Server, ServerSecret, error) {
	var server Server
	var command sql.NullString
	var argsRaw string
	var workingDir sql.NullString
	var envRaw string
	var httpURL sql.NullString
	var headersRaw string
	var lastError sql.NullString
	var connectedRaw any
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&server.ID, &server.WorkspaceID, &server.Name, &server.Description, &server.TransportType, &command, &argsRaw,
		&workingDir, &envRaw, &httpURL, &headersRaw, &server.Enabled, &server.StartupTimeoutMS, &server.RequestTimeoutMS,
		&server.HealthStatus, &lastError, &connectedRaw, &createdRaw, &updatedRaw); err != nil {
		return Server{}, ServerSecret{}, err
	}
	server.Command = command.String
	server.Arguments = parseArray(argsRaw)
	server.WorkingDirectory = workingDir.String
	secret := ServerSecret{Environment: parseMap(envRaw), HTTPHeaders: parseMap(headersRaw)}
	server.EnvironmentKeys = mapKeys(secret.Environment)
	server.HTTPURL = httpURL.String
	server.HTTPHeaderKeys = mapKeys(secret.HTTPHeaders)
	server.LastError = lastError.String
	var err error
	server.LastConnectedAt, err = nullableTime(connectedRaw)
	if err != nil {
		return Server{}, ServerSecret{}, err
	}
	server.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Server{}, ServerSecret{}, err
	}
	server.UpdatedAt, err = database.ParseTime(updatedRaw)
	if err != nil {
		return Server{}, ServerSecret{}, err
	}
	return server, secret, nil
}

func scanTool(row rowScanner) (Tool, error) {
	var tool Tool
	var discoveredRaw any
	var updatedRaw any
	if err := row.Scan(&tool.ID, &tool.ServerID, &tool.Name, &tool.Description, &tool.InputSchema, &tool.PermissionMode, &discoveredRaw, &updatedRaw); err != nil {
		return Tool{}, err
	}
	var err error
	tool.DiscoveredAt, err = database.ParseTime(discoveredRaw)
	if err != nil {
		return Tool{}, err
	}
	tool.UpdatedAt, err = database.ParseTime(updatedRaw)
	if err != nil {
		return Tool{}, err
	}
	return tool, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func parseArray(raw string) []string {
	var values []string
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

func parseMap(raw string) map[string]string {
	values := map[string]string{}
	_ = json.Unmarshal([]byte(raw), &values)
	return values
}

func mapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func jsonArray(values []string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func jsonMap(values map[string]string) string {
	if values == nil {
		values = map[string]string{}
	}
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func nullableTime(value any) (*time.Time, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
	case []byte:
		if strings.TrimSpace(string(typed)) == "" {
			return nil, nil
		}
	}
	parsed, err := database.ParseTime(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
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

func timePtrArg(store *database.Store, value *time.Time) any {
	if value == nil {
		return nil
	}
	return store.NowArg(*value)
}
