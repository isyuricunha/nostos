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
	ListEnabledServers(ctx context.Context, limit int) ([]Server, []ServerSecret, error)
	GetServer(ctx context.Context, workspaceID string, serverID string) (Server, ServerSecret, error)
	CreateServer(ctx context.Context, server Server, secret ServerSecret) (Server, error)
	UpdateServer(ctx context.Context, server Server, secret ServerSecret) (Server, error)
	DeleteServer(ctx context.Context, workspaceID string, serverID string) error
	ReplaceTools(ctx context.Context, serverID string, tools []DiscoveredTool) ([]Tool, error)
	ListTools(ctx context.Context, workspaceID string, serverID string) ([]Tool, error)
	ListAgentTools(ctx context.Context, workspaceID string, agentID string, conversationID string) ([]AgentTool, error)
	GetToolForExecution(ctx context.Context, workspaceID string, toolID string) (Tool, Server, ServerSecret, error)
	UpdateToolPermission(ctx context.Context, workspaceID string, toolID string, mode string) error
	UpsertAgentToolPermission(ctx context.Context, workspaceID string, agentID string, toolID string, mode string) error
	ReplaceAgentServerAssignments(ctx context.Context, workspaceID string, agentID string, serverIDs []string) error
	ListAgentServerAssignments(ctx context.Context, workspaceID string, agentID string) ([]string, error)
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

func (r *SQLRepository) ListEnabledServers(ctx context.Context, limit int) ([]Server, []ServerSecret, error) {
	if limit <= 0 {
		limit = 25
	}
	query := `SELECT id, workspace_id, name, description, transport_type, command, arguments, working_directory, encrypted_environment,
http_url, encrypted_http_headers, enabled, startup_timeout_ms, request_timeout_ms, health_status, last_error, last_connected_at, created_at, updated_at
FROM mcp_servers WHERE enabled = ` + r.store.Placeholder(1) + ` ORDER BY updated_at ASC LIMIT ` + r.store.Placeholder(2)
	rows, err := r.store.DB.QueryContext(ctx, query, true, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var servers []Server
	var secrets []ServerSecret
	for rows.Next() {
		server, secret, err := scanServer(rows)
		if err != nil {
			return nil, nil, err
		}
		servers = append(servers, server)
		secrets = append(secrets, secret)
	}
	return servers, secrets, rows.Err()
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

func (r *SQLRepository) ListAgentTools(ctx context.Context, workspaceID string, agentID string, conversationID string) ([]AgentTool, error) {
	if strings.TrimSpace(agentID) == "" {
		return nil, nil
	}
	query := `SELECT t.id, t.server_id, t.name, t.description, t.input_schema, t.permission_mode, t.discovered_at, t.updated_at,
s.id, s.workspace_id, s.name, s.description, s.transport_type, s.command, s.arguments, s.working_directory, s.encrypted_environment,
s.http_url, s.encrypted_http_headers, s.enabled, s.startup_timeout_ms, s.request_timeout_ms, s.health_status, s.last_error, s.last_connected_at, s.created_at, s.updated_at,
COALESCE(atp.permission_mode, '') AS agent_permission,
EXISTS (SELECT 1 FROM tool_approvals ta WHERE ta.workspace_id = ` + r.store.Placeholder(1) + ` AND ta.tool_id = t.id AND ta.conversation_id = ` + r.store.Placeholder(2) + ` AND ta.decision = 'approve_conversation') AS conversation_approved,
EXISTS (SELECT 1 FROM tool_approvals ta WHERE ta.workspace_id = ` + r.store.Placeholder(3) + ` AND ta.tool_id = t.id AND ta.agent_id = ` + r.store.Placeholder(4) + ` AND ta.decision = 'allow_agent') AS agent_approval
FROM agent_mcp_servers ams
JOIN mcp_servers s ON s.id = ams.server_id
JOIN mcp_tools t ON t.server_id = s.id
LEFT JOIN agent_tool_permissions atp ON atp.agent_id = ams.agent_id AND atp.tool_id = t.id
WHERE ams.agent_id = ` + r.store.Placeholder(5) + ` AND s.workspace_id = ` + r.store.Placeholder(6) + ` AND s.enabled = ` + r.store.Placeholder(7) + `
ORDER BY s.name, t.name`
	rows, err := r.store.DB.QueryContext(ctx, query, workspaceID, conversationID, workspaceID, agentID, agentID, workspaceID, true)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AgentTool
	for rows.Next() {
		var item AgentTool
		var agentPermission sql.NullString
		tool, server, err := scanAgentToolRow(rows, &agentPermission, &item.ConversationApproved, &item.AgentApprovalPersisted)
		if err != nil {
			return nil, err
		}
		item.Tool = tool
		item.Server = server
		item.AgentPermission = agentPermission.String
		item.GlobalPermission = tool.PermissionMode
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) GetToolForExecution(ctx context.Context, workspaceID string, toolID string) (Tool, Server, ServerSecret, error) {
	query := `SELECT t.id, t.server_id, t.name, t.description, t.input_schema, t.permission_mode, t.discovered_at, t.updated_at,
s.id, s.workspace_id, s.name, s.description, s.transport_type, s.command, s.arguments, s.working_directory, s.encrypted_environment,
s.http_url, s.encrypted_http_headers, s.enabled, s.startup_timeout_ms, s.request_timeout_ms, s.health_status, s.last_error, s.last_connected_at, s.created_at, s.updated_at
FROM mcp_tools t JOIN mcp_servers s ON s.id = t.server_id
WHERE s.workspace_id = ` + r.store.Placeholder(1) + ` AND t.id = ` + r.store.Placeholder(2)
	var tool Tool
	var server Server
	var secret ServerSecret
	err := scanToolServerSecretRow(r.store.DB.QueryRowContext(ctx, query, workspaceID, toolID), &tool, &server, &secret)
	if errors.Is(err, sql.ErrNoRows) {
		return Tool{}, Server{}, ServerSecret{}, ErrNotFound
	}
	return tool, server, secret, err
}

func (r *SQLRepository) UpdateToolPermission(ctx context.Context, workspaceID string, toolID string, mode string) error {
	query := `UPDATE mcp_tools SET permission_mode = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
		` WHERE id = ` + r.store.Placeholder(3) + ` AND server_id IN (SELECT id FROM mcp_servers WHERE workspace_id = ` + r.store.Placeholder(4) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, mode, r.store.NowArg(time.Now().UTC()), toolID, workspaceID)
	return err
}

func (r *SQLRepository) UpsertAgentToolPermission(ctx context.Context, workspaceID string, agentID string, toolID string, mode string) error {
	now := time.Now().UTC()
	query := `INSERT INTO agent_tool_permissions (id, agent_id, tool_id, permission_mode, created_at, updated_at)
SELECT ` + r.store.Placeholder(1) + `, ` + r.store.Placeholder(2) + `, ` + r.store.Placeholder(3) + `, ` + r.store.Placeholder(4) + `, ` + r.store.Placeholder(5) + `, ` + r.store.Placeholder(6) + `
WHERE EXISTS (SELECT 1 FROM agents WHERE id = ` + r.store.Placeholder(7) + ` AND workspace_id = ` + r.store.Placeholder(8) + `)
AND EXISTS (SELECT 1 FROM mcp_tools t JOIN mcp_servers s ON s.id = t.server_id WHERE t.id = ` + r.store.Placeholder(9) + ` AND s.workspace_id = ` + r.store.Placeholder(10) + `)
ON CONFLICT(agent_id, tool_id) DO UPDATE SET permission_mode = excluded.permission_mode, updated_at = excluded.updated_at`
	_, err := r.store.DB.ExecContext(ctx, query, id.New(), agentID, toolID, mode, r.store.NowArg(now), r.store.NowArg(now), agentID, workspaceID, toolID, workspaceID)
	return err
}

func (r *SQLRepository) ReplaceAgentServerAssignments(ctx context.Context, workspaceID string, agentID string, serverIDs []string) error {
	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	deleteQuery := `DELETE FROM agent_mcp_servers WHERE agent_id = ` + r.store.Placeholder(1) +
		` AND agent_id IN (SELECT id FROM agents WHERE workspace_id = ` + r.store.Placeholder(2) + `)`
	if _, err := tx.ExecContext(ctx, deleteQuery, agentID, workspaceID); err != nil {
		return err
	}
	for _, serverID := range serverIDs {
		if strings.TrimSpace(serverID) == "" {
			continue
		}
		query := `INSERT INTO agent_mcp_servers (agent_id, server_id, created_at)
SELECT ` + r.store.Placeholder(1) + `, ` + r.store.Placeholder(2) + `, ` + r.store.Placeholder(3) + `
WHERE EXISTS (SELECT 1 FROM agents WHERE id = ` + r.store.Placeholder(4) + ` AND workspace_id = ` + r.store.Placeholder(5) + `)
AND EXISTS (SELECT 1 FROM mcp_servers WHERE id = ` + r.store.Placeholder(6) + ` AND workspace_id = ` + r.store.Placeholder(7) + `)
ON CONFLICT(agent_id, server_id) DO NOTHING`
		if _, err := tx.ExecContext(ctx, query, agentID, serverID, r.store.NowArg(time.Now().UTC()), agentID, workspaceID, serverID, workspaceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLRepository) ListAgentServerAssignments(ctx context.Context, workspaceID string, agentID string) ([]string, error) {
	query := `SELECT ams.server_id FROM agent_mcp_servers ams
JOIN agents a ON a.id = ams.agent_id
JOIN mcp_servers s ON s.id = ams.server_id
WHERE a.workspace_id = ` + r.store.Placeholder(1) + ` AND s.workspace_id = ` + r.store.Placeholder(2) + ` AND ams.agent_id = ` + r.store.Placeholder(3) + `
ORDER BY s.name`
	rows, err := r.store.DB.QueryContext(ctx, query, workspaceID, workspaceID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var serverID string
		if err := rows.Scan(&serverID); err != nil {
			return nil, err
		}
		ids = append(ids, serverID)
	}
	return ids, rows.Err()
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

func scanAgentToolRow(row rowScanner, agentPermission *sql.NullString, conversationApproved *bool, agentApprovalPersisted *bool) (Tool, Server, error) {
	var tool Tool
	var server Server
	var command sql.NullString
	var argsRaw string
	var workingDir sql.NullString
	var envRaw string
	var httpURL sql.NullString
	var headersRaw string
	var lastError sql.NullString
	var toolDiscoveredRaw any
	var toolUpdatedRaw any
	var serverConnectedRaw any
	var serverCreatedRaw any
	var serverUpdatedRaw any
	if err := row.Scan(&tool.ID, &tool.ServerID, &tool.Name, &tool.Description, &tool.InputSchema, &tool.PermissionMode, &toolDiscoveredRaw, &toolUpdatedRaw,
		&server.ID, &server.WorkspaceID, &server.Name, &server.Description, &server.TransportType, &command, &argsRaw, &workingDir, &envRaw,
		&httpURL, &headersRaw, &server.Enabled, &server.StartupTimeoutMS, &server.RequestTimeoutMS, &server.HealthStatus, &lastError, &serverConnectedRaw,
		&serverCreatedRaw, &serverUpdatedRaw, agentPermission, conversationApproved, agentApprovalPersisted); err != nil {
		return Tool{}, Server{}, err
	}
	var err error
	tool.DiscoveredAt, err = database.ParseTime(toolDiscoveredRaw)
	if err != nil {
		return Tool{}, Server{}, err
	}
	tool.UpdatedAt, err = database.ParseTime(toolUpdatedRaw)
	if err != nil {
		return Tool{}, Server{}, err
	}
	hydrateServerSecrets(&server, ServerSecret{Environment: parseMap(envRaw), HTTPHeaders: parseMap(headersRaw)}, command, argsRaw, workingDir, httpURL, lastError)
	server.LastConnectedAt, err = nullableTime(serverConnectedRaw)
	if err != nil {
		return Tool{}, Server{}, err
	}
	server.CreatedAt, err = database.ParseTime(serverCreatedRaw)
	if err != nil {
		return Tool{}, Server{}, err
	}
	server.UpdatedAt, err = database.ParseTime(serverUpdatedRaw)
	return tool, server, err
}

func scanToolServerSecretRow(row rowScanner, tool *Tool, server *Server, secret *ServerSecret) error {
	var command sql.NullString
	var argsRaw string
	var workingDir sql.NullString
	var envRaw string
	var httpURL sql.NullString
	var headersRaw string
	var lastError sql.NullString
	var toolDiscoveredRaw any
	var toolUpdatedRaw any
	var serverConnectedRaw any
	var serverCreatedRaw any
	var serverUpdatedRaw any
	if err := row.Scan(&tool.ID, &tool.ServerID, &tool.Name, &tool.Description, &tool.InputSchema, &tool.PermissionMode, &toolDiscoveredRaw, &toolUpdatedRaw,
		&server.ID, &server.WorkspaceID, &server.Name, &server.Description, &server.TransportType, &command, &argsRaw, &workingDir, &envRaw,
		&httpURL, &headersRaw, &server.Enabled, &server.StartupTimeoutMS, &server.RequestTimeoutMS, &server.HealthStatus, &lastError, &serverConnectedRaw,
		&serverCreatedRaw, &serverUpdatedRaw); err != nil {
		return err
	}
	var err error
	tool.DiscoveredAt, err = database.ParseTime(toolDiscoveredRaw)
	if err != nil {
		return err
	}
	tool.UpdatedAt, err = database.ParseTime(toolUpdatedRaw)
	if err != nil {
		return err
	}
	*secret = ServerSecret{Environment: parseMap(envRaw), HTTPHeaders: parseMap(headersRaw)}
	hydrateServerSecrets(server, *secret, command, argsRaw, workingDir, httpURL, lastError)
	server.LastConnectedAt, err = nullableTime(serverConnectedRaw)
	if err != nil {
		return err
	}
	server.CreatedAt, err = database.ParseTime(serverCreatedRaw)
	if err != nil {
		return err
	}
	server.UpdatedAt, err = database.ParseTime(serverUpdatedRaw)
	return err
}

func hydrateServerSecrets(server *Server, secret ServerSecret, command sql.NullString, argsRaw string, workingDir sql.NullString, httpURL sql.NullString, lastError sql.NullString) {
	server.Command = command.String
	server.Arguments = parseArray(argsRaw)
	server.WorkingDirectory = workingDir.String
	server.EnvironmentKeys = mapKeys(secret.Environment)
	server.HTTPURL = httpURL.String
	server.HTTPHeaderKeys = mapKeys(secret.HTTPHeaders)
	server.LastError = lastError.String
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
