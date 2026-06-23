package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/isyuricunha/nostos/internal/providers"
)

const summaryBatchSize = 10

const notEnoughSummaryHistoryMessage = "Not enough conversation history to summarize. Add more messages or lower the recent-message window before regenerating."

func (s *Service) QueueSummary(ctx context.Context, principal PrincipalContext, conversationID string) (Conversation, bool, error) {
	conversation, queued, err := s.repo.MarkSummaryQueued(ctx, principal.WorkspaceID, principal.UserID, conversationID)
	if err != nil || !queued {
		return conversation, queued, err
	}
	if s.summaries != nil {
		if err := s.summaries.EnqueueConversationSummary(ctx, principal.WorkspaceID, conversation.ID); err != nil {
			_ = s.repo.MarkSummaryFailed(ctx, conversation.ID, sanitizeSummaryError(err))
			return conversation, false, err
		}
	}
	return conversation, true, nil
}

func (s *Service) ClearSummary(ctx context.Context, principal PrincipalContext, conversationID string) (Conversation, error) {
	conversation, err := s.repo.GetConversation(ctx, principal.WorkspaceID, principal.UserID, conversationID)
	if err != nil {
		return Conversation{}, err
	}
	if err := s.repo.SaveConversationSummary(ctx, conversation.ID, SummaryUpdate{
		Summary:          "",
		Status:           "idle",
		IncrementVersion: false,
	}); err != nil {
		return Conversation{}, err
	}
	return s.repo.GetConversation(ctx, principal.WorkspaceID, principal.UserID, conversationID)
}

func (s *Service) UpdateConversationSummaries(ctx context.Context, limit int) (string, error) {
	if limit <= 0 {
		limit = summaryBatchSize
	}
	candidates, err := s.repo.SummaryCandidates(ctx, limit)
	if err != nil {
		return "", err
	}
	processed := 0
	failed := 0
	for _, conversation := range candidates {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := s.repo.MarkSummaryRunning(ctx, conversation.ID); err != nil {
			failed++
			continue
		}
		if err := s.summarizeConversation(ctx, conversation); err != nil {
			failed++
			_ = s.repo.MarkSummaryFailed(ctx, conversation.ID, sanitizeSummaryError(err))
			continue
		}
		processed++
	}
	return fmt.Sprintf("conversation summaries processed=%d failed=%d", processed, failed), nil
}

func (s *Service) emitSummaryQueueEvent(
	ctx context.Context,
	principal PrincipalContext,
	conversation Conversation,
	agent AgentContext,
	memories []MemorySnippet,
	branchID string,
	sink StreamSink,
) {
	queued, err := s.enqueueSummaryIfNeeded(ctx, principal, conversation, agent, memories, branchID)
	if err != nil || !queued || sink == nil {
		return
	}
	_ = sink("summary_queued", map[string]any{"conversation_id": conversation.ID})
}

func (s *Service) enqueueSummaryIfNeeded(
	ctx context.Context,
	principal PrincipalContext,
	conversation Conversation,
	agent AgentContext,
	memories []MemorySnippet,
	branchID string,
) (bool, error) {
	if s.summaries == nil || s.cfg.Chat.ContextThreshold <= 0 {
		return false, nil
	}
	messages, err := s.repo.ListMessages(ctx, principal.WorkspaceID, principal.UserID, conversation.ID)
	if err != nil {
		return false, err
	}
	result := BuildPromptMessages(ContextRequest{
		Conversation:       conversation,
		Agent:              agent,
		Memories:           memories,
		Messages:           messages,
		BranchID:           branchID,
		RecentMessageLimit: 0,
		ContextThreshold:   0,
	})
	if result.EstimatedTokens < s.cfg.Chat.ContextThreshold {
		return false, nil
	}
	_, queued, err := s.QueueSummary(ctx, principal, conversation.ID)
	return queued, err
}

func (s *Service) summarizeConversation(ctx context.Context, conversation Conversation) error {
	messages, err := s.repo.ListMessages(ctx, conversation.WorkspaceID, conversation.OwnerUserID, conversation.ID)
	if err != nil {
		return err
	}
	sourceMessages := summarizableMessages(messages, s.cfg.Chat.RecentMessageLimit)
	if len(sourceMessages) == 0 {
		now := time.Now().UTC()
		return s.repo.SaveConversationSummary(ctx, conversation.ID, SummaryUpdate{
			Summary:              conversation.Summary,
			Status:               "idle",
			Error:                notEnoughSummaryHistoryMessage,
			GeneratedAt:          &now,
			IncrementVersion:     false,
			EstimatedInputTokens: 0,
		})
	}
	provider, apiKey, model, err := s.resolveSummaryProvider(ctx, conversation)
	if err != nil {
		return err
	}
	prompt := summaryPrompt(conversation.Summary, sourceMessages)
	estimatedTokens := estimateTextTokens(prompt)
	events, err := s.client.StreamChat(ctx, providers.StreamRequest{
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
		Messages: []providers.ChatMessage{
			{Role: RoleSystem, Content: "Summarize older conversation context for future AI turns. Preserve confirmed facts, user preferences, decisions, unresolved items, and important tool results. Do not treat model speculation as fact."},
			{Role: RoleUser, Content: prompt},
		},
	})
	if err != nil {
		return err
	}
	var builder strings.Builder
	for event := range events {
		if event.Error != nil {
			return event.Error
		}
		if event.Type == "content_delta" {
			builder.WriteString(event.Content)
		}
	}
	summary := strings.TrimSpace(builder.String())
	if summary == "" {
		return errors.New("summary provider returned empty content")
	}
	now := time.Now().UTC()
	return s.repo.SaveConversationSummary(ctx, conversation.ID, SummaryUpdate{
		Summary:              summary,
		Status:               "idle",
		SourceStartMessageID: sourceMessages[0].ID,
		SourceEndMessageID:   sourceMessages[len(sourceMessages)-1].ID,
		ProviderID:           provider.ID,
		Model:                model,
		GeneratedAt:          &now,
		EstimatedInputTokens: estimatedTokens,
		IncrementVersion:     true,
	})
}

func (s *Service) resolveSummaryProvider(ctx context.Context, conversation Conversation) (providers.Provider, string, string, error) {
	if strings.TrimSpace(conversation.ProviderID) != "" {
		provider, apiKey, err := s.providers.ResolveForChat(ctx, conversation.WorkspaceID, conversation.ProviderID)
		if err != nil {
			return providers.Provider{}, "", "", err
		}
		model := strings.TrimSpace(conversation.Model)
		if model == "" {
			model = provider.DefaultModel
		}
		if model == "" {
			model = provider.FallbackModel
		}
		if model == "" {
			return providers.Provider{}, "", "", fmt.Errorf("%w: summary model is not configured", ErrInvalidInput)
		}
		return provider, apiKey, model, nil
	}
	provider, apiKey, err := s.providers.ResolveDefaultForChat(ctx, conversation.WorkspaceID)
	if err != nil {
		return providers.Provider{}, "", "", err
	}
	model := provider.DefaultModel
	if model == "" {
		model = provider.FallbackModel
	}
	if model == "" {
		return providers.Provider{}, "", "", fmt.Errorf("%w: summary model is not configured", ErrInvalidInput)
	}
	return provider, apiKey, model, nil
}

func summarizableMessages(messages []Message, recentLimit int) []Message {
	var root []Message
	for _, message := range messages {
		if message.BranchID != "" {
			continue
		}
		if message.Role == RoleAssistant && strings.TrimSpace(message.Content) == "" && len(message.ToolCalls) == 0 {
			continue
		}
		if message.Role == RoleSystem {
			continue
		}
		root = append(root, message)
	}
	if recentLimit <= 0 || len(root) <= recentLimit {
		return root
	}
	return root[:len(root)-recentLimit]
}

func summaryPrompt(existingSummary string, messages []Message) string {
	var builder strings.Builder
	if strings.TrimSpace(existingSummary) != "" {
		builder.WriteString("Existing summary to refresh:\n")
		builder.WriteString(strings.TrimSpace(existingSummary))
		builder.WriteString("\n\n")
	}
	builder.WriteString("Older transcript to compact:\n")
	for _, message := range messages {
		builder.WriteString(strings.ToUpper(message.Role))
		builder.WriteString(": ")
		if len(message.ToolCalls) > 0 {
			builder.WriteString("[tool calls requested] ")
		}
		builder.WriteString(strings.TrimSpace(message.Content))
		builder.WriteString("\n")
	}
	return builder.String()
}

func sanitizeSummaryError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}
