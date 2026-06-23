package replies

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/providers"
)

var ErrInvalidInput = errors.New("invalid reply input")

type ProviderResolver interface {
	ResolveForChat(ctx context.Context, workspaceID string, providerID string) (providers.Provider, string, error)
}

type ModelRoleResolver interface {
	ResolveModelRole(ctx context.Context, workspaceID string, role string) (providers.RoleResolution, error)
}

type Service struct {
	cfg       config.Config
	repo      Repository
	providers ProviderResolver
	client    *providers.OpenAIClient
}

func NewService(cfg config.Config, repo Repository, providerResolver ProviderResolver, client *providers.OpenAIClient) *Service {
	return &Service{cfg: cfg, repo: repo, providers: providerResolver, client: client}
}

func (s *Service) EnsureDefaultPresets(ctx context.Context) error {
	workspaces, err := s.repo.Workspaces(ctx)
	if err != nil {
		return err
	}
	for _, workspaceID := range workspaces {
		if err := s.ensureDefaultPresetsForWorkspace(ctx, workspaceID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListPresets(ctx context.Context, principal PrincipalContext) ([]Preset, error) {
	if err := s.ensureDefaultPresetsForWorkspace(ctx, principal.WorkspaceID); err != nil {
		return nil, err
	}
	return s.repo.ListPresets(ctx, principal.WorkspaceID)
}

func (s *Service) CreatePreset(ctx context.Context, principal PrincipalContext, input PresetInput) (Preset, error) {
	preset, err := normalizePreset(principal.WorkspaceID, input)
	if err != nil {
		return Preset{}, err
	}
	return s.repo.CreatePreset(ctx, preset)
}

func (s *Service) UpdatePreset(ctx context.Context, principal PrincipalContext, presetID string, input PresetInput) (Preset, error) {
	preset, err := normalizePreset(principal.WorkspaceID, input)
	if err != nil {
		return Preset{}, err
	}
	existing, err := s.repo.GetPreset(ctx, principal.WorkspaceID, presetID)
	if err != nil {
		return Preset{}, err
	}
	preset.ID = presetID
	preset.SystemDefault = existing.SystemDefault
	return s.repo.UpdatePreset(ctx, preset)
}

func (s *Service) DeletePreset(ctx context.Context, principal PrincipalContext, presetID string) error {
	return s.repo.DeletePreset(ctx, principal.WorkspaceID, presetID)
}

func (s *Service) ResetDefaults(ctx context.Context, principal PrincipalContext) error {
	for _, preset := range defaultPresets(principal.WorkspaceID) {
		exists, err := s.repo.HasPreset(ctx, principal.WorkspaceID, preset.Name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := s.repo.CreatePreset(ctx, preset); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) GenerateDraft(ctx context.Context, principal PrincipalContext, input DraftInput) (Draft, error) {
	sourceID := strings.TrimSpace(input.SourceMessageID)
	if sourceID == "" {
		return Draft{}, fmt.Errorf("%w: source_message_id is required", ErrInvalidInput)
	}
	presetID := strings.TrimSpace(input.PresetID)
	if presetID == "" {
		return Draft{}, fmt.Errorf("%w: preset_id is required", ErrInvalidInput)
	}
	source, err := s.repo.GetSourceMessage(ctx, principal.WorkspaceID, principal.UserID, sourceID)
	if err != nil {
		return Draft{}, err
	}
	preset, err := s.repo.GetPreset(ctx, principal.WorkspaceID, presetID)
	if err != nil {
		return Draft{}, err
	}
	if !preset.Active {
		return Draft{}, fmt.Errorf("%w: reply preset is disabled", ErrInvalidInput)
	}
	providerID := strings.TrimSpace(input.ProviderID)
	if providerID == "" {
		providerID = source.ProviderID
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = source.Model
	}
	if s.providers == nil || s.client == nil {
		return Draft{}, errors.New("provider execution is not configured")
	}
	var provider providers.Provider
	var apiKey string
	if providerID == "" && model == "" {
		roleResolver, ok := s.providers.(ModelRoleResolver)
		if !ok {
			return Draft{}, fmt.Errorf("%w: provider_id and model are required", ErrInvalidInput)
		}
		resolution, err := roleResolver.ResolveModelRole(ctx, principal.WorkspaceID, providers.ModelRoleUtility)
		if err != nil {
			return Draft{}, err
		}
		provider = resolution.Provider
		apiKey = resolution.APIKey
		model = resolution.ModelID
	} else {
		if providerID == "" || model == "" {
			return Draft{}, fmt.Errorf("%w: provider_id and model are required", ErrInvalidInput)
		}
		var err error
		provider, apiKey, err = s.providers.ResolveForChat(ctx, principal.WorkspaceID, providerID)
		if err != nil {
			return Draft{}, err
		}
	}
	timeout := time.Duration(provider.RequestTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = s.cfg.Chat.DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	events, err := s.client.StreamChat(runCtx, providers.StreamRequest{
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
		Messages: []providers.ChatMessage{
			{Role: "system", Content: "Generate one editable reply draft. Do not send it. Do not include commentary before or after the draft."},
			{Role: "user", Content: draftPrompt(source.Content, preset, input.CustomInstruction)},
		},
	})
	if err != nil {
		return Draft{}, err
	}
	var builder strings.Builder
	for event := range events {
		if event.Error != nil {
			return Draft{}, event.Error
		}
		if event.Content != "" {
			builder.WriteString(event.Content)
		}
	}
	text := strings.TrimSpace(builder.String())
	if text == "" {
		return Draft{}, errors.New("provider returned an empty reply draft")
	}
	return s.repo.CreateDraft(ctx, Draft{
		WorkspaceID:       principal.WorkspaceID,
		SourceMessageID:   source.ID,
		PresetID:          preset.ID,
		PresetName:        preset.Name,
		CustomInstruction: strings.TrimSpace(input.CustomInstruction),
		GeneratedDraft:    text,
		ProviderID:        provider.ID,
		Model:             model,
	})
}

func (s *Service) ListDrafts(ctx context.Context, principal PrincipalContext, sourceMessageID string) ([]Draft, error) {
	return s.repo.ListDrafts(ctx, principal.WorkspaceID, principal.UserID, sourceMessageID)
}

func (s *Service) ensureDefaultPresetsForWorkspace(ctx context.Context, workspaceID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	for _, preset := range defaultPresets(workspaceID) {
		exists, err := s.repo.HasPreset(ctx, workspaceID, preset.Name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := s.repo.CreatePreset(ctx, preset); err != nil {
			return err
		}
	}
	return nil
}

func normalizePreset(workspaceID string, input PresetInput) (Preset, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Preset{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	instruction := strings.TrimSpace(input.PromptInstruction)
	if instruction == "" {
		return Preset{}, fmt.Errorf("%w: prompt_instruction is required", ErrInvalidInput)
	}
	icon := strings.TrimSpace(input.Icon)
	if icon == "" {
		icon = "message-circle"
	}
	return Preset{
		WorkspaceID:       workspaceID,
		Name:              name,
		Description:       strings.TrimSpace(input.Description),
		PromptInstruction: instruction,
		Icon:              icon,
		SortOrder:         input.SortOrder,
		Active:            input.Active,
	}, nil
}

func draftPrompt(source string, preset Preset, customInstruction string) string {
	var builder strings.Builder
	builder.WriteString("Source message:\n")
	builder.WriteString(source)
	builder.WriteString("\n\nReply intent:\n")
	builder.WriteString(preset.PromptInstruction)
	if strings.TrimSpace(customInstruction) != "" {
		builder.WriteString("\n\nAdditional custom instruction:\n")
		builder.WriteString(strings.TrimSpace(customInstruction))
	}
	return builder.String()
}

func defaultPresets(workspaceID string) []Preset {
	defaults := []struct {
		name        string
		description string
		instruction string
		icon        string
	}{
		{"Positive", "Draft a positive and constructive reply.", "Respond positively and constructively while staying natural.", "thumbs-up"},
		{"Negative", "Draft a negative but controlled reply.", "Respond negatively or disagree clearly without becoming hostile.", "thumbs-down"},
		{"Empathetic", "Draft a warm and understanding reply.", "Respond with empathy, care, and emotional awareness.", "heart"},
		{"Direct", "Draft a concise direct reply.", "Respond directly with minimal hedging.", "send"},
		{"Formal", "Draft a polished formal reply.", "Use a formal, professional tone.", "briefcase"},
		{"Informal", "Draft a casual reply.", "Use an informal, conversational tone.", "smile"},
		{"Short", "Draft a short reply.", "Keep the reply very brief.", "minimize"},
		{"Ask for clarification", "Ask for more information.", "Ask a clear follow-up question to clarify what is needed.", "help-circle"},
		{"Politely accept", "Accept politely.", "Politely accept the offer, request, or idea.", "check"},
		{"Politely decline", "Decline politely.", "Politely decline without over-explaining.", "x"},
		{"Custom", "Use the custom instruction field.", "Follow the user's custom instruction exactly while producing a sendable draft.", "wand"},
	}
	presets := make([]Preset, 0, len(defaults))
	for index, item := range defaults {
		presets = append(presets, Preset{
			WorkspaceID:       workspaceID,
			Name:              item.name,
			Description:       item.description,
			PromptInstruction: item.instruction,
			Icon:              item.icon,
			SortOrder:         index + 1,
			Active:            true,
			SystemDefault:     true,
		})
	}
	return presets
}
