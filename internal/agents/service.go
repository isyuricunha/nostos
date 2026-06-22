package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yuricunha/nostos/internal/chat"
)

var ErrInvalidInput = errors.New("invalid agent input")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) EnsureDefaultAgents(ctx context.Context) error {
	workspaces, err := s.repo.Workspaces(ctx)
	if err != nil {
		return err
	}
	for _, workspaceID := range workspaces {
		if err := s.ensureDefaultAgent(ctx, workspaceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) List(ctx context.Context, principal PrincipalContext) ([]Agent, error) {
	if err := s.ensureDefaultAgent(ctx, principal.WorkspaceID); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, principal.WorkspaceID)
}

func (s *Service) Create(ctx context.Context, principal PrincipalContext, input AgentInput) (Agent, error) {
	agent, err := normalizeInput(principal.WorkspaceID, input)
	if err != nil {
		return Agent{}, err
	}
	return s.repo.Create(ctx, agent)
}

func (s *Service) Update(ctx context.Context, principal PrincipalContext, agentID string, input AgentInput) (Agent, error) {
	agent, err := normalizeInput(principal.WorkspaceID, input)
	if err != nil {
		return Agent{}, err
	}
	agent.ID = agentID
	return s.repo.Update(ctx, agent)
}

func (s *Service) Duplicate(ctx context.Context, principal PrincipalContext, agentID string) (Agent, error) {
	agent, err := s.repo.Get(ctx, principal.WorkspaceID, agentID)
	if err != nil {
		return Agent{}, err
	}
	agent.ID = ""
	agent.Name = agent.Name + " Copy"
	return s.repo.Create(ctx, agent)
}

func (s *Service) Delete(ctx context.Context, principal PrincipalContext, agentID string) error {
	return s.repo.Delete(ctx, principal.WorkspaceID, agentID)
}

func (s *Service) GetChatAgent(ctx context.Context, workspaceID string, agentID string) (chat.AgentContext, error) {
	if strings.TrimSpace(agentID) == "" {
		return chat.AgentContext{}, nil
	}
	agent, err := s.repo.Get(ctx, workspaceID, agentID)
	if err != nil {
		return chat.AgentContext{}, err
	}
	if !agent.Active {
		return chat.AgentContext{}, nil
	}
	return chat.AgentContext{
		ID:                agent.ID,
		Name:              agent.Name,
		SystemPrompt:      agent.SystemPrompt,
		DefaultProviderID: agent.DefaultProviderID,
		DefaultModel:      agent.DefaultModel,
		FallbackModel:     agent.FallbackModel,
		MemoryAccessMode:  agent.MemoryAccessMode,
	}, nil
}

func (s *Service) ensureDefaultAgent(ctx context.Context, workspaceID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	hasDefault, err := s.repo.HasDefault(ctx, workspaceID)
	if err != nil {
		return err
	}
	if hasDefault {
		return nil
	}
	_, err = s.repo.Create(ctx, Agent{
		WorkspaceID:           workspaceID,
		Name:                  DefaultAgentName,
		Description:           "Balanced assistant for general workspace tasks.",
		Avatar:                "sparkles",
		SystemPrompt:          "You are a concise, careful assistant in a private self-hosted AI workspace. Follow the user's instructions, use provided memories transparently, and avoid inventing facts.",
		Temperature:           0.7,
		MaxToolIterations:     8,
		MemoryAccessMode:      "pinned_only",
		ToolPermissionDefault: "ask",
		Active:                true,
	})
	return err
}

func normalizeInput(workspaceID string, input AgentInput) (Agent, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Agent{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	systemPrompt := strings.TrimSpace(input.SystemPrompt)
	if systemPrompt == "" {
		return Agent{}, fmt.Errorf("%w: system prompt is required", ErrInvalidInput)
	}
	memoryMode := input.MemoryAccessMode
	if memoryMode == "" {
		memoryMode = "pinned_only"
	}
	if memoryMode != "none" && memoryMode != "pinned_only" && memoryMode != "relevant" && memoryMode != "all" {
		return Agent{}, fmt.Errorf("%w: memory access mode is invalid", ErrInvalidInput)
	}
	toolDefault := input.ToolPermissionDefault
	if toolDefault == "" {
		toolDefault = "ask"
	}
	if toolDefault != "deny" && toolDefault != "ask" && toolDefault != "allow" {
		return Agent{}, fmt.Errorf("%w: tool permission default is invalid", ErrInvalidInput)
	}
	iterations := input.MaxToolIterations
	if iterations <= 0 {
		iterations = 8
	}
	temperature := input.Temperature
	if temperature == 0 {
		temperature = 0.7
	}
	avatar := strings.TrimSpace(input.Avatar)
	if avatar == "" {
		avatar = "sparkles"
	}
	return Agent{
		WorkspaceID:           workspaceID,
		Name:                  name,
		Description:           strings.TrimSpace(input.Description),
		Avatar:                avatar,
		SystemPrompt:          systemPrompt,
		DefaultProviderID:     strings.TrimSpace(input.DefaultProviderID),
		DefaultModel:          strings.TrimSpace(input.DefaultModel),
		FallbackModel:         strings.TrimSpace(input.FallbackModel),
		Temperature:           temperature,
		MaxToolIterations:     iterations,
		MemoryAccessMode:      memoryMode,
		ToolPermissionDefault: toolDefault,
		Active:                input.Active,
	}, nil
}
