package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yuricunha/nostos/internal/chat"
)

var ErrInvalidInput = errors.New("invalid memory input")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, principal PrincipalContext, query string) ([]Memory, error) {
	return s.repo.List(ctx, principal.WorkspaceID, query)
}

func (s *Service) Create(ctx context.Context, principal PrincipalContext, input MemoryInput) (Memory, error) {
	memory, err := normalizeInput(principal, input)
	if err != nil {
		return Memory{}, err
	}
	return s.repo.Create(ctx, memory)
}

func (s *Service) Update(ctx context.Context, principal PrincipalContext, memoryID string, input MemoryInput) (Memory, error) {
	memory, err := normalizeInput(principal, input)
	if err != nil {
		return Memory{}, err
	}
	memory.ID = memoryID
	return s.repo.Update(ctx, memory)
}

func (s *Service) Delete(ctx context.Context, principal PrincipalContext, memoryID string) error {
	return s.repo.Delete(ctx, principal.WorkspaceID, memoryID)
}

func (s *Service) SelectForRun(ctx context.Context, request chat.MemoryRequest) ([]chat.MemorySnippet, error) {
	candidates, err := s.repo.Candidates(ctx, request)
	if err != nil {
		return nil, err
	}
	return RankMemories(candidates, request), nil
}

func (s *Service) RecordRunMemories(ctx context.Context, runID string, memories []chat.MemorySnippet) error {
	return s.repo.RecordRun(ctx, runID, memories)
}

func (s *Service) UsedByRun(ctx context.Context, runID string) ([]chat.MemorySnippet, error) {
	return s.repo.UsedByRun(ctx, runID)
}

func (s *Service) RemoveFromRun(ctx context.Context, runID string, memoryID string) error {
	return s.repo.RemoveFromRun(ctx, runID, memoryID, time.Now().UTC())
}

func normalizeInput(principal PrincipalContext, input MemoryInput) (Memory, error) {
	title := strings.TrimSpace(input.Title)
	content := strings.TrimSpace(input.Content)
	if title == "" {
		return Memory{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if content == "" {
		return Memory{}, fmt.Errorf("%w: content is required", ErrInvalidInput)
	}
	scope := strings.TrimSpace(input.Scope)
	if scope == "" {
		scope = "global"
	}
	if scope != "global" && scope != "agent" && scope != "conversation" && scope != "workspace" {
		return Memory{}, fmt.Errorf("%w: scope is invalid", ErrInvalidInput)
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "manual"
	}
	if source != "manual" && source != "message" && source != "task" && source != "import" {
		return Memory{}, fmt.Errorf("%w: source is invalid", ErrInvalidInput)
	}
	importance := input.Importance
	if importance <= 0 {
		importance = 50
	}
	if importance > 100 {
		importance = 100
	}
	return Memory{
		WorkspaceID:     principal.WorkspaceID,
		OwnerUserID:     principal.UserID,
		AgentID:         strings.TrimSpace(input.AgentID),
		ConversationID:  strings.TrimSpace(input.ConversationID),
		Title:           title,
		Content:         content,
		Tags:            input.Tags,
		Scope:           scope,
		Importance:      importance,
		Pinned:          input.Pinned,
		Active:          input.Active,
		Source:          source,
		SourceMessageID: strings.TrimSpace(input.SourceMessageID),
	}, nil
}
