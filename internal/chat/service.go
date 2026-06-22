package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/providers"
)

var ErrInvalidInput = errors.New("invalid chat input")

type ProviderResolver interface {
	ResolveForChat(ctx context.Context, workspaceID string, providerID string) (providers.Provider, string, error)
	ResolveDefaultForChat(ctx context.Context, workspaceID string) (providers.Provider, string, error)
}

type AgentResolver interface {
	GetChatAgent(ctx context.Context, workspaceID string, agentID string) (AgentContext, error)
}

type MemoryProvider interface {
	SelectForRun(ctx context.Context, request MemoryRequest) ([]MemorySnippet, error)
	RecordRunMemories(ctx context.Context, runID string, memories []MemorySnippet) error
}

type ToolProvider interface {
	AllowedChatTools(ctx context.Context, workspaceID string) ([]providers.ChatTool, error)
	ExecuteAllowedTool(ctx context.Context, workspaceID string, name string, arguments string) (string, error)
}

type SummaryEnqueuer interface {
	EnqueueConversationSummary(ctx context.Context, workspaceID string, conversationID string) error
}

type Service struct {
	cfg       config.Config
	repo      Repository
	providers ProviderResolver
	client    *providers.OpenAIClient
	agents    AgentResolver
	memories  MemoryProvider
	tools     ToolProvider
	summaries SummaryEnqueuer
}

type StreamSink func(event string, payload any) error

func NewService(
	cfg config.Config,
	repo Repository,
	providerResolver ProviderResolver,
	client *providers.OpenAIClient,
	agentResolver AgentResolver,
	memoryProvider MemoryProvider,
) *Service {
	return &Service{cfg: cfg, repo: repo, providers: providerResolver, client: client, agents: agentResolver, memories: memoryProvider}
}

func (s *Service) WithToolProvider(toolProvider ToolProvider) *Service {
	s.tools = toolProvider
	return s
}

func (s *Service) WithSummaryEnqueuer(enqueuer SummaryEnqueuer) *Service {
	s.summaries = enqueuer
	return s
}

func (s *Service) ListConversations(ctx context.Context, principal PrincipalContext, search string) ([]Conversation, error) {
	return s.repo.ListConversations(ctx, principal.WorkspaceID, principal.UserID, search)
}

func (s *Service) CreateConversation(ctx context.Context, principal PrincipalContext, input Conversation) (Conversation, error) {
	input.WorkspaceID = principal.WorkspaceID
	input.OwnerUserID = principal.UserID
	return s.repo.CreateConversation(ctx, input)
}

func (s *Service) GetConversation(ctx context.Context, principal PrincipalContext, conversationID string) (Conversation, error) {
	return s.repo.GetConversation(ctx, principal.WorkspaceID, principal.UserID, conversationID)
}

func (s *Service) UpdateConversation(ctx context.Context, principal PrincipalContext, conversationID string, input UpdateConversationInput) (Conversation, error) {
	conversation, err := s.repo.GetConversation(ctx, principal.WorkspaceID, principal.UserID, conversationID)
	if err != nil {
		return Conversation{}, err
	}
	if strings.TrimSpace(input.Title) != "" {
		conversation.Title = strings.TrimSpace(input.Title)
	}
	if input.Archive != nil {
		if *input.Archive {
			now := time.Now().UTC()
			conversation.ArchivedAt = &now
		} else {
			conversation.ArchivedAt = nil
		}
	}
	if input.Summary != nil {
		conversation.Summary = strings.TrimSpace(*input.Summary)
		now := time.Now().UTC()
		if conversation.Summary == "" {
			conversation.SummaryUpdatedAt = nil
		} else {
			conversation.SummaryUpdatedAt = &now
		}
	}
	return s.repo.UpdateConversation(ctx, conversation)
}

func (s *Service) DeleteConversation(ctx context.Context, principal PrincipalContext, conversationID string) error {
	return s.repo.DeleteConversation(ctx, principal.WorkspaceID, principal.UserID, conversationID, time.Now().UTC())
}

func (s *Service) ListMessages(ctx context.Context, principal PrincipalContext, conversationID string) ([]Message, error) {
	return s.repo.ListMessages(ctx, principal.WorkspaceID, principal.UserID, conversationID)
}

func (s *Service) Run(ctx context.Context, principal PrincipalContext, conversationID string, input RunInput, sink StreamSink) error {
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return fmt.Errorf("%w: message content is required", ErrInvalidInput)
	}
	conversation, err := s.repo.GetConversation(ctx, principal.WorkspaceID, principal.UserID, conversationID)
	if err != nil {
		return err
	}
	providerID := strings.TrimSpace(input.ProviderID)
	if providerID == "" {
		providerID = conversation.ProviderID
	}
	agent, err := s.resolveAgent(ctx, conversation)
	if err != nil {
		return err
	}
	if providerID == "" {
		providerID = agent.DefaultProviderID
	}
	if providerID == "" {
		return fmt.Errorf("%w: provider is required", ErrInvalidInput)
	}
	provider, apiKey, err := s.providers.ResolveForChat(ctx, principal.WorkspaceID, providerID)
	if err != nil {
		return err
	}
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = conversation.Model
	}
	if model == "" {
		model = agent.DefaultModel
	}
	if model == "" {
		model = provider.DefaultModel
	}
	if model == "" {
		return fmt.Errorf("%w: model is required", ErrInvalidInput)
	}
	parentMessageID, err := s.lastContextMessageID(ctx, principal, conversation.ID, "")
	if err != nil {
		return err
	}

	userMessage, err := s.repo.CreateMessage(ctx, Message{
		ConversationID:  conversation.ID,
		ParentMessageID: parentMessageID,
		Role:            RoleUser,
		Content:         content,
		ProviderID:      provider.ID,
		Model:           model,
	})
	if err != nil {
		return err
	}
	assistantMessage, err := s.repo.CreateMessage(ctx, Message{
		ConversationID:  conversation.ID,
		ParentMessageID: userMessage.ID,
		Role:            RoleAssistant,
		Content:         "",
		ProviderID:      provider.ID,
		Model:           model,
	})
	if err != nil {
		return err
	}
	run, err := s.repo.CreateRun(ctx, ChatRun{
		ConversationID:     conversation.ID,
		UserMessageID:      userMessage.ID,
		AssistantMessageID: assistantMessage.ID,
		ProviderID:         provider.ID,
		Model:              model,
	})
	if err != nil {
		return err
	}
	memories, err := s.selectMemories(ctx, principal, conversation, agent, content)
	if err != nil {
		return err
	}
	if len(memories) > 0 {
		_ = s.repo.UpdateRunState(ctx, run.ID, RunStreaming, "", "", false)
		if err := s.memories.RecordRunMemories(ctx, run.ID, memories); err != nil {
			return err
		}
	}
	promptMessages, err := s.contextMessages(ctx, principal, conversation, agent, memories, userMessage.ID, assistantMessage.ID, "")
	if err != nil {
		return err
	}
	if err := sink("run_started", map[string]any{"run": run, "user_message": userMessage, "assistant_message": assistantMessage}); err != nil {
		return err
	}
	if len(memories) > 0 {
		if err := sink("memories_used", map[string]any{"memories": memories}); err != nil {
			return err
		}
	}

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events, err := s.client.StreamChat(streamCtx, providers.StreamRequest{
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
		Messages: promptMessages,
		Tools:    s.allowedTools(ctx, principal.WorkspaceID),
	})
	if err != nil {
		_ = s.repo.UpdateRunState(ctx, run.ID, RunFailed, "provider_unavailable", err.Error(), true)
		_ = sink("run_failed", map[string]any{"run_id": run.ID, "message": err.Error()})
		return err
	}

	var contentBuilder strings.Builder
	var usage UsageValues
	var toolCalls []providers.ToolCall
	toolReady := false
	for event := range events {
		cancelled, err := s.repo.CancellationRequested(ctx, run.ID)
		if err != nil {
			return err
		}
		if cancelled {
			cancel()
			_ = s.repo.UpdateRunState(ctx, run.ID, RunCancelled, "", "", true)
			_ = sink("run_cancelled", map[string]any{"run_id": run.ID})
			return nil
		}
		if event.Error != nil {
			_ = s.repo.UpdateRunState(ctx, run.ID, RunFailed, "provider_stream_failed", event.Error.Error(), true)
			_ = sink("run_failed", map[string]any{"run_id": run.ID, "message": event.Error.Error()})
			return event.Error
		}
		switch event.Type {
		case "content_delta":
			contentBuilder.WriteString(event.Content)
			_ = s.repo.UpdateMessageContent(ctx, assistantMessage.ID, contentBuilder.String(), usage)
			if err := sink("content_delta", map[string]string{"delta": event.Content}); err != nil {
				return err
			}
		case "reasoning_delta":
			if err := sink("reasoning_delta", map[string]string{"delta": event.Reasoning}); err != nil {
				return err
			}
		case "tool_call_delta":
			if event.ToolCall != nil {
				toolCalls = mergeToolCall(toolCalls, *event.ToolCall)
			}
			if err := sink(event.Type, event.ToolCall); err != nil {
				return err
			}
		case "tool_call_ready":
			toolReady = true
			_ = s.repo.UpdateRunState(ctx, run.ID, RunWaitingForApproval, "", "", false)
			if err := sink(event.Type, map[string]any{"run_id": run.ID, "tool_calls": toolCalls}); err != nil {
				return err
			}
		case "usage":
			if event.Usage != nil {
				usage = UsageValues{PromptTokens: event.Usage.PromptTokens, CompletionTokens: event.Usage.CompletionTokens, TotalTokens: event.Usage.TotalTokens}
				_ = s.repo.UpdateRunUsage(ctx, run.ID, usage)
				_ = s.repo.UpdateMessageContent(ctx, assistantMessage.ID, contentBuilder.String(), usage)
				if err := sink("usage", usage); err != nil {
					return err
				}
			}
		case "run_completed":
			if toolReady {
				break
			}
			_ = s.repo.UpdateMessageContent(ctx, assistantMessage.ID, contentBuilder.String(), usage)
			_ = s.repo.UpdateRunState(ctx, run.ID, RunCompleted, "", "", true)
			s.emitSummaryQueueEvent(ctx, principal, conversation, agent, memories, "", sink)
			return sink("run_completed", map[string]any{"run_id": run.ID, "assistant_message_id": assistantMessage.ID})
		}
	}
	if toolReady {
		return s.executeToolFollowup(ctx, principal, run.ID, assistantMessage.ID, provider, apiKey, model, conversation, agent, memories, "", promptMessages, toolCalls, sink)
	}
	_ = s.repo.UpdateRunState(ctx, run.ID, RunCompleted, "", "", true)
	s.emitSummaryQueueEvent(ctx, principal, conversation, agent, memories, "", sink)
	return sink("run_completed", map[string]any{"run_id": run.ID, "assistant_message_id": assistantMessage.ID})
}

func (s *Service) CancelRun(ctx context.Context, principal PrincipalContext, runID string) error {
	return s.repo.RequestCancellation(ctx, principal.WorkspaceID, principal.UserID, runID, time.Now().UTC())
}

func (s *Service) Regenerate(ctx context.Context, principal PrincipalContext, assistantMessageID string, input RunInput, sink StreamSink) error {
	run, err := s.repo.FindRunByAssistantMessage(ctx, principal.WorkspaceID, principal.UserID, assistantMessageID)
	if err != nil {
		return err
	}
	userMessage, err := s.repo.GetMessage(ctx, principal.WorkspaceID, principal.UserID, run.UserMessageID)
	if err != nil {
		return err
	}
	branch, err := s.repo.CreateBranch(ctx, Branch{
		ConversationID:  run.ConversationID,
		ParentMessageID: userMessage.ParentMessageID,
		SourceMessageID: assistantMessageID,
		Name:            "Regenerated response",
	})
	if err != nil {
		return err
	}
	input.Content = userMessage.Content
	if instruction := strings.TrimSpace(input.RegenerationInstruction); instruction != "" {
		input.Content += "\n\nRegeneration instruction:\n" + instruction
	}
	return s.runOnBranch(ctx, principal, run.ConversationID, branch, input, sink)
}

func (s *Service) EditAndBranch(ctx context.Context, principal PrincipalContext, messageID string, input RunInput, sink StreamSink) error {
	message, err := s.repo.GetMessage(ctx, principal.WorkspaceID, principal.UserID, messageID)
	if err != nil {
		return err
	}
	if message.Role != RoleUser {
		return fmt.Errorf("%w: only user messages can be edited", ErrInvalidInput)
	}
	branch, err := s.repo.CreateBranch(ctx, Branch{
		ConversationID:  message.ConversationID,
		ParentMessageID: message.ParentMessageID,
		SourceMessageID: message.ID,
		Name:            "Edited message branch",
	})
	if err != nil {
		return err
	}
	return s.runOnBranch(ctx, principal, message.ConversationID, branch, input, sink)
}

func (s *Service) runOnBranch(ctx context.Context, principal PrincipalContext, conversationID string, branch Branch, input RunInput, sink StreamSink) error {
	if strings.TrimSpace(input.Content) == "" {
		return fmt.Errorf("%w: message content is required", ErrInvalidInput)
	}
	conversation, err := s.repo.GetConversation(ctx, principal.WorkspaceID, principal.UserID, conversationID)
	if err != nil {
		return err
	}
	providerID := input.ProviderID
	if providerID == "" {
		providerID = conversation.ProviderID
	}
	agent, err := s.resolveAgent(ctx, conversation)
	if err != nil {
		return err
	}
	if providerID == "" {
		providerID = agent.DefaultProviderID
	}
	provider, apiKey, err := s.providers.ResolveForChat(ctx, principal.WorkspaceID, providerID)
	if err != nil {
		return err
	}
	model := input.Model
	if model == "" {
		model = conversation.Model
	}
	if model == "" {
		model = agent.DefaultModel
	}
	if model == "" {
		model = provider.DefaultModel
	}
	userMessage, err := s.repo.CreateMessage(ctx, Message{
		ConversationID:  conversationID,
		BranchID:        branch.ID,
		ParentMessageID: branch.ParentMessageID,
		Role:            RoleUser,
		Content:         strings.TrimSpace(input.Content),
		ProviderID:      provider.ID,
		Model:           model,
	})
	if err != nil {
		return err
	}
	assistantMessage, err := s.repo.CreateMessage(ctx, Message{
		ConversationID:  conversationID,
		BranchID:        branch.ID,
		ParentMessageID: userMessage.ID,
		Role:            RoleAssistant,
		ProviderID:      provider.ID,
		Model:           model,
	})
	if err != nil {
		return err
	}
	run, err := s.repo.CreateRun(ctx, ChatRun{
		ConversationID:     conversationID,
		UserMessageID:      userMessage.ID,
		AssistantMessageID: assistantMessage.ID,
		BranchID:           branch.ID,
		ProviderID:         provider.ID,
		Model:              model,
	})
	if err != nil {
		return err
	}
	if err := sink("run_started", map[string]any{"run": run, "user_message": userMessage, "assistant_message": assistantMessage}); err != nil {
		return err
	}
	memories, err := s.selectMemories(ctx, principal, conversation, agent, input.Content)
	if err != nil {
		return err
	}
	if len(memories) > 0 {
		if err := s.memories.RecordRunMemories(ctx, run.ID, memories); err != nil {
			return err
		}
		if err := sink("memories_used", map[string]any{"memories": memories}); err != nil {
			return err
		}
	}
	promptMessages, err := s.contextMessages(ctx, principal, conversation, agent, memories, userMessage.ID, assistantMessage.ID, branch.ID)
	if err != nil {
		return err
	}
	events, err := s.client.StreamChat(ctx, providers.StreamRequest{
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
		Messages: promptMessages,
	})
	if err != nil {
		_ = s.repo.UpdateRunState(ctx, run.ID, RunFailed, "provider_unavailable", err.Error(), true)
		return err
	}
	var contentBuilder strings.Builder
	for event := range events {
		if event.Type == "content_delta" {
			contentBuilder.WriteString(event.Content)
			_ = s.repo.UpdateMessageContent(ctx, assistantMessage.ID, contentBuilder.String(), UsageValues{})
			if err := sink("content_delta", map[string]string{"delta": event.Content}); err != nil {
				return err
			}
		}
		if event.Type == "run_completed" {
			_ = s.repo.UpdateRunState(ctx, run.ID, RunCompleted, "", "", true)
			s.emitSummaryQueueEvent(ctx, principal, conversation, agent, memories, branch.ID, sink)
			return sink("run_completed", map[string]any{"run_id": run.ID, "assistant_message_id": assistantMessage.ID})
		}
	}
	_ = s.repo.UpdateRunState(ctx, run.ID, RunCompleted, "", "", true)
	s.emitSummaryQueueEvent(ctx, principal, conversation, agent, memories, branch.ID, sink)
	return sink("run_completed", map[string]any{"run_id": run.ID, "assistant_message_id": assistantMessage.ID})
}

func (s *Service) CleanupInterruptedRuns(ctx context.Context) error {
	_, err := s.repo.CleanupInterruptedRuns(ctx, time.Now().UTC())
	return err
}

func (s *Service) allowedTools(ctx context.Context, workspaceID string) []providers.ChatTool {
	if s.tools == nil {
		return nil
	}
	tools, err := s.tools.AllowedChatTools(ctx, workspaceID)
	if err != nil {
		return nil
	}
	return tools
}

func mergeToolCall(calls []providers.ToolCall, next providers.ToolCall) []providers.ToolCall {
	if next.ID == "" {
		next.ID = next.Function.Name
	}
	if next.Type == "" {
		next.Type = "function"
	}
	for index := range calls {
		if calls[index].ID == next.ID {
			if next.Function.Name != "" {
				calls[index].Function.Name = next.Function.Name
			}
			calls[index].Function.Arguments += next.Function.Arguments
			return calls
		}
	}
	return append(calls, next)
}

func (s *Service) executeToolFollowup(
	ctx context.Context,
	principal PrincipalContext,
	runID string,
	assistantMessageID string,
	provider providers.Provider,
	apiKey string,
	model string,
	conversation Conversation,
	agent AgentContext,
	memories []MemorySnippet,
	branchID string,
	baseMessages []providers.ChatMessage,
	toolCalls []providers.ToolCall,
	sink StreamSink,
) error {
	if s.tools == nil || len(toolCalls) == 0 {
		message := "The model requested a tool, but no executable tool was available."
		_ = s.repo.UpdateRunState(ctx, runID, RunFailed, "tool_unavailable", message, true)
		_ = sink("tool_approval_required", map[string]any{"run_id": runID, "message": message})
		return errors.New(message)
	}
	messages := append([]providers.ChatMessage{}, baseMessages...)
	messages = append(messages, providers.ChatMessage{Role: "assistant", ToolCalls: toolCalls})
	for _, call := range toolCalls {
		result, err := s.tools.ExecuteAllowedTool(ctx, principal.WorkspaceID, call.Function.Name, call.Function.Arguments)
		if err != nil {
			_ = s.repo.UpdateRunState(ctx, runID, RunFailed, "tool_execution_failed", err.Error(), true)
			_ = sink("run_failed", map[string]any{"run_id": runID, "message": err.Error()})
			return err
		}
		if err := sink("tool_result", map[string]any{"tool_call_id": call.ID, "name": call.Function.Name, "result": result}); err != nil {
			return err
		}
		messages = append(messages, providers.ChatMessage{
			Role:       "tool",
			ToolCallID: call.ID,
			Content:    result,
		})
	}
	_ = s.repo.UpdateRunState(ctx, runID, RunStreaming, "", "", false)
	events, err := s.client.StreamChat(ctx, providers.StreamRequest{
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		_ = s.repo.UpdateRunState(ctx, runID, RunFailed, "provider_unavailable", err.Error(), true)
		return err
	}
	var contentBuilder strings.Builder
	var usage UsageValues
	for event := range events {
		if event.Error != nil {
			_ = s.repo.UpdateRunState(ctx, runID, RunFailed, "provider_stream_failed", event.Error.Error(), true)
			_ = sink("run_failed", map[string]any{"run_id": runID, "message": event.Error.Error()})
			return event.Error
		}
		switch event.Type {
		case "content_delta":
			contentBuilder.WriteString(event.Content)
			_ = s.repo.UpdateMessageContent(ctx, assistantMessageID, contentBuilder.String(), usage)
			if err := sink("content_delta", map[string]string{"delta": event.Content}); err != nil {
				return err
			}
		case "usage":
			if event.Usage != nil {
				usage = UsageValues{PromptTokens: event.Usage.PromptTokens, CompletionTokens: event.Usage.CompletionTokens, TotalTokens: event.Usage.TotalTokens}
				_ = s.repo.UpdateRunUsage(ctx, runID, usage)
				_ = s.repo.UpdateMessageContent(ctx, assistantMessageID, contentBuilder.String(), usage)
				if err := sink("usage", usage); err != nil {
					return err
				}
			}
		case "run_completed":
			_ = s.repo.UpdateMessageContent(ctx, assistantMessageID, contentBuilder.String(), usage)
			_ = s.repo.UpdateRunState(ctx, runID, RunCompleted, "", "", true)
			s.emitSummaryQueueEvent(ctx, principal, conversation, agent, memories, branchID, sink)
			return sink("run_completed", map[string]any{"run_id": runID, "assistant_message_id": assistantMessageID})
		}
	}
	_ = s.repo.UpdateRunState(ctx, runID, RunCompleted, "", "", true)
	s.emitSummaryQueueEvent(ctx, principal, conversation, agent, memories, branchID, sink)
	return sink("run_completed", map[string]any{"run_id": runID, "assistant_message_id": assistantMessageID})
}

func (s *Service) resolveAgent(ctx context.Context, conversation Conversation) (AgentContext, error) {
	if s.agents == nil || conversation.AgentID == "" {
		return AgentContext{MemoryAccessMode: "pinned_only"}, nil
	}
	agent, err := s.agents.GetChatAgent(ctx, conversation.WorkspaceID, conversation.AgentID)
	if err != nil {
		return AgentContext{}, err
	}
	if agent.MemoryAccessMode == "" {
		agent.MemoryAccessMode = "pinned_only"
	}
	return agent, nil
}

func (s *Service) selectMemories(ctx context.Context, principal PrincipalContext, conversation Conversation, agent AgentContext, query string) ([]MemorySnippet, error) {
	if s.memories == nil {
		return nil, nil
	}
	return s.memories.SelectForRun(ctx, MemoryRequest{
		WorkspaceID:    principal.WorkspaceID,
		UserID:         principal.UserID,
		AgentID:        agent.ID,
		ConversationID: conversation.ID,
		AccessMode:     agent.MemoryAccessMode,
		Query:          query,
	})
}

func (s *Service) contextMessages(
	ctx context.Context,
	principal PrincipalContext,
	conversation Conversation,
	agent AgentContext,
	memories []MemorySnippet,
	currentUserMessageID string,
	assistantPlaceholderID string,
	branchID string,
) ([]providers.ChatMessage, error) {
	messages, err := s.repo.ListMessages(ctx, principal.WorkspaceID, principal.UserID, conversation.ID)
	if err != nil {
		return nil, err
	}
	result := BuildPromptMessages(ContextRequest{
		Conversation:           conversation,
		Agent:                  agent,
		Memories:               memories,
		Messages:               messages,
		CurrentUserMessageID:   currentUserMessageID,
		AssistantPlaceholderID: assistantPlaceholderID,
		BranchID:               branchID,
		RecentMessageLimit:     s.cfg.Chat.RecentMessageLimit,
		ContextThreshold:       s.cfg.Chat.ContextThreshold,
	})
	return result.Messages, nil
}

func (s *Service) lastContextMessageID(ctx context.Context, principal PrincipalContext, conversationID string, branchID string) (string, error) {
	messages, err := s.repo.ListMessages(ctx, principal.WorkspaceID, principal.UserID, conversationID)
	if err != nil {
		return "", err
	}
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message.BranchID != branchID {
			continue
		}
		if message.Role == RoleAssistant && strings.TrimSpace(message.Content) == "" && len(message.ToolCalls) == 0 {
			continue
		}
		return message.ID, nil
	}
	return "", nil
}
