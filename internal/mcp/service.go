package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/yuricunha/nostos/internal/auth"
	"github.com/yuricunha/nostos/internal/config"
	"github.com/yuricunha/nostos/internal/crypto"
	"github.com/yuricunha/nostos/internal/providers"
)

var (
	ErrInvalidInput     = errors.New("invalid MCP input")
	ErrSecretKey        = errors.New("APP_ENCRYPTION_KEY is required for MCP secrets")
	ErrApprovalRequired = errors.New("MCP tool approval is required")
	ErrToolDenied       = errors.New("MCP tool is denied")
)

type Service struct {
	cfg    config.Config
	repo   Repository
	audit  auth.Repository
	client *Client
}

func NewService(cfg config.Config, repo Repository, audit auth.Repository, client *Client) *Service {
	return &Service{cfg: cfg, repo: repo, audit: audit, client: client}
}

func (s *Service) ListServers(ctx context.Context, principal PrincipalContext) ([]Server, error) {
	return s.repo.ListServers(ctx, principal.WorkspaceID)
}

func (s *Service) CreateServer(ctx context.Context, principal PrincipalContext, input ServerInput) (Server, error) {
	server, secret, err := s.normalizeInput(principal.WorkspaceID, input)
	if err != nil {
		return Server{}, err
	}
	created, err := s.repo.CreateServer(ctx, server, secret)
	if err != nil {
		return Server{}, err
	}
	s.auditEvent(ctx, principal, "mcp_server_created", created.ID)
	return created, nil
}

func (s *Service) UpdateServer(ctx context.Context, principal PrincipalContext, serverID string, input ServerInput) (Server, error) {
	server, secret, err := s.normalizeInput(principal.WorkspaceID, input)
	if err != nil {
		return Server{}, err
	}
	server.ID = serverID
	updated, err := s.repo.UpdateServer(ctx, server, secret)
	if err != nil {
		return Server{}, err
	}
	s.auditEvent(ctx, principal, "mcp_server_updated", serverID)
	return updated, nil
}

func (s *Service) DeleteServer(ctx context.Context, principal PrincipalContext, serverID string) error {
	if err := s.repo.DeleteServer(ctx, principal.WorkspaceID, serverID); err != nil {
		return err
	}
	s.auditEvent(ctx, principal, "mcp_server_deleted", serverID)
	return nil
}

func (s *Service) DiscoverTools(ctx context.Context, principal PrincipalContext, serverID string) ([]Tool, error) {
	server, secret, err := s.repo.GetServer(ctx, principal.WorkspaceID, serverID)
	if err != nil {
		return nil, err
	}
	secret, err = s.decryptSecret(secret)
	if err != nil {
		return nil, err
	}
	discovered, err := s.client.Discover(ctx, server, secret)
	now := time.Now().UTC()
	if err != nil {
		_ = s.repo.UpdateServerHealth(ctx, principal.WorkspaceID, serverID, "unhealthy", err.Error(), nil)
		return nil, err
	}
	_ = s.repo.UpdateServerHealth(ctx, principal.WorkspaceID, serverID, "healthy", "", &now)
	return s.repo.ReplaceTools(ctx, serverID, discovered)
}

func (s *Service) ListTools(ctx context.Context, principal PrincipalContext, serverID string) ([]Tool, error) {
	return s.repo.ListTools(ctx, principal.WorkspaceID, serverID)
}

func (s *Service) UpdateToolPermission(ctx context.Context, principal PrincipalContext, toolID string, mode string) error {
	if mode != "deny" && mode != "ask" && mode != "allow" {
		return fmt.Errorf("%w: permission mode is invalid", ErrInvalidInput)
	}
	return s.repo.UpdateToolPermission(ctx, principal.WorkspaceID, toolID, mode)
}

func (s *Service) AllowedChatTools(ctx context.Context, workspaceID string) ([]providers.ChatTool, error) {
	tools, err := s.repo.ListTools(ctx, workspaceID, "")
	if err != nil {
		return nil, err
	}
	out := make([]providers.ChatTool, 0, len(tools))
	for _, tool := range tools {
		if tool.PermissionMode != "allow" {
			continue
		}
		parameters := json.RawMessage(`{}`)
		if strings.TrimSpace(tool.InputSchema) != "" {
			parameters = json.RawMessage(tool.InputSchema)
		}
		out = append(out, providers.ChatTool{
			Type: "function",
			Function: providers.ChatToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  parameters,
			},
		})
	}
	return out, nil
}

func (s *Service) ExecuteAllowedTool(ctx context.Context, workspaceID string, name string, arguments string) (string, error) {
	tools, err := s.repo.ListTools(ctx, workspaceID, "")
	if err != nil {
		return "", err
	}
	var selected Tool
	for _, tool := range tools {
		if tool.Name == name {
			selected = tool
			break
		}
	}
	if selected.ID == "" {
		return "", ErrNotFound
	}
	switch selected.PermissionMode {
	case "deny":
		return "", ErrToolDenied
	case "ask":
		return "", ErrApprovalRequired
	case "allow":
	default:
		return "", ErrApprovalRequired
	}
	server, secret, err := s.repo.GetServer(ctx, workspaceID, selected.ServerID)
	if err != nil {
		return "", err
	}
	if !server.Enabled {
		return "", ErrToolDenied
	}
	secret, err = s.decryptSecret(secret)
	if err != nil {
		return "", err
	}
	result, err := s.client.CallTool(ctx, server, secret, selected.Name, arguments)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func (s *Service) normalizeInput(workspaceID string, input ServerInput) (Server, ServerSecret, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Server{}, ServerSecret{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	transport := strings.TrimSpace(input.TransportType)
	if transport != "stdio" && transport != "http" {
		return Server{}, ServerSecret{}, fmt.Errorf("%w: transport must be stdio or http", ErrInvalidInput)
	}
	if transport == "stdio" && strings.TrimSpace(input.Command) == "" {
		return Server{}, ServerSecret{}, fmt.Errorf("%w: command is required for stdio", ErrInvalidInput)
	}
	if transport == "http" {
		parsed, err := url.Parse(strings.TrimSpace(input.HTTPURL))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return Server{}, ServerSecret{}, fmt.Errorf("%w: http_url must be an HTTP URL", ErrInvalidInput)
		}
	}
	startupTimeout := input.StartupTimeoutMS
	if startupTimeout <= 0 {
		startupTimeout = 10000
	}
	requestTimeout := input.RequestTimeoutMS
	if requestTimeout <= 0 {
		requestTimeout = 30000
	}
	secret, err := s.encryptSecret(ServerSecret{Environment: input.Environment, HTTPHeaders: input.HTTPHeaders})
	if err != nil {
		return Server{}, ServerSecret{}, err
	}
	return Server{
		WorkspaceID:      workspaceID,
		Name:             name,
		Description:      strings.TrimSpace(input.Description),
		TransportType:    transport,
		Command:          strings.TrimSpace(input.Command),
		Arguments:        input.Arguments,
		WorkingDirectory: strings.TrimSpace(input.WorkingDirectory),
		HTTPURL:          strings.TrimSpace(input.HTTPURL),
		Enabled:          input.Enabled,
		StartupTimeoutMS: startupTimeout,
		RequestTimeoutMS: requestTimeout,
	}, secret, nil
}

func (s *Service) encryptSecret(secret ServerSecret) (ServerSecret, error) {
	if len(secret.Environment) == 0 && len(secret.HTTPHeaders) == 0 {
		return secret, nil
	}
	if len(s.cfg.Security.EncryptionKey) != 32 {
		return ServerSecret{}, ErrSecretKey
	}
	return ServerSecret{
		Environment: encryptMap(s.cfg.Security.EncryptionKey, secret.Environment),
		HTTPHeaders: encryptMap(s.cfg.Security.EncryptionKey, secret.HTTPHeaders),
	}, nil
}

func (s *Service) decryptSecret(secret ServerSecret) (ServerSecret, error) {
	return ServerSecret{
		Environment: decryptMap(s.cfg.Security.EncryptionKey, secret.Environment),
		HTTPHeaders: decryptMap(s.cfg.Security.EncryptionKey, secret.HTTPHeaders),
	}, nil
}

func encryptMap(key []byte, values map[string]string) map[string]string {
	out := map[string]string{}
	for name, value := range values {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		encrypted, err := crypto.Encrypt(key, value)
		if err == nil {
			out[name] = encrypted
		}
	}
	return out
}

func decryptMap(key []byte, values map[string]string) map[string]string {
	out := map[string]string{}
	for name, value := range values {
		decrypted, err := crypto.Decrypt(key, value)
		if err == nil {
			out[name] = decrypted
		}
	}
	return out
}

func (s *Service) auditEvent(ctx context.Context, principal PrincipalContext, eventType string, serverID string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.InsertAuditEvent(ctx, auth.AuditEvent{
		WorkspaceID: principal.WorkspaceID,
		ActorUserID: principal.UserID,
		EventType:   eventType,
		IPAddress:   principal.IPAddress,
		UserAgent:   principal.UserAgent,
		Metadata:    map[string]any{"mcp_server_id": serverID},
	})
}
