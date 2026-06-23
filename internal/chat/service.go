package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/providers"
)

var (
	ErrInvalidInput     = errors.New("invalid chat input")
	ErrApprovalRequired = errors.New("tool approval is required")
)

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
	UsedByRun(ctx context.Context, runID string) ([]MemorySnippet, error)
}

type ToolProvider interface {
	RuntimeTools(ctx context.Context, request ToolExposureRequest) ([]RuntimeTool, error)
	ExecuteRuntimeTool(ctx context.Context, request ToolExecutionRequest) (ToolExecutionResult, error)
	SetAgentToolPermission(ctx context.Context, workspaceID string, agentID string, toolID string, mode string) error
	DisableTool(ctx context.Context, workspaceID string, toolID string) error
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
	promptMessages, err := s.contextMessages(ctx, principal, conversation, agent, memories, userMessage.ID, assistantMessage.ID, "", "")
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

	runtimeTools, err := s.runtimeTools(ctx, principal, conversation, agent)
	if err != nil {
		_ = s.repo.UpdateRunState(ctx, run.ID, RunFailed, "tool_resolution_failed", err.Error(), true)
		return err
	}
	return s.executeModelLoop(ctx, modelLoopRequest{
		Principal:          principal,
		Conversation:       conversation,
		Agent:              agent,
		Run:                run,
		AssistantMessageID: assistantMessage.ID,
		Provider:           provider,
		APIKey:             apiKey,
		Model:              model,
		Memories:           memories,
		BranchID:           "",
		Messages:           promptMessages,
		Tools:              runtimeTools,
	}, sink)
}

func (s *Service) CancelRun(ctx context.Context, principal PrincipalContext, runID string) error {
	return s.repo.RequestCancellation(ctx, principal.WorkspaceID, principal.UserID, runID, time.Now().UTC())
}

func (s *Service) ListPendingToolApprovals(ctx context.Context, principal PrincipalContext) ([]ToolCallRecord, error) {
	return s.repo.ListPendingToolApprovals(ctx, principal.WorkspaceID, principal.UserID)
}

func (s *Service) ApproveToolCall(ctx context.Context, principal PrincipalContext, toolCallID string, decision string) (ToolCallRecord, error) {
	if decision == "" {
		decision = ToolDecisionApproveOnce
	}
	if decision != ToolDecisionApproveOnce && decision != ToolDecisionApproveConversation && decision != ToolDecisionAllowAgent {
		return ToolCallRecord{}, fmt.Errorf("%w: approval decision is invalid", ErrInvalidInput)
	}
	call, _, conversation, err := s.repo.GetToolCall(ctx, principal.WorkspaceID, principal.UserID, toolCallID)
	if err != nil {
		return ToolCallRecord{}, err
	}
	if call.State == ToolCallSucceeded {
		return call, nil
	}
	if call.State != ToolCallWaitingForApproval && call.State != ToolCallApproved {
		return ToolCallRecord{}, fmt.Errorf("%w: tool call is not waiting for approval", ErrInvalidInput)
	}
	agent, err := s.resolveAgent(ctx, conversation)
	if err != nil {
		return ToolCallRecord{}, err
	}
	if decision == ToolDecisionAllowAgent {
		if s.tools == nil {
			return ToolCallRecord{}, errors.New("tool provider is unavailable")
		}
		if err := s.tools.SetAgentToolPermission(ctx, principal.WorkspaceID, agent.ID, call.ToolID, ToolPermissionAllow); err != nil {
			return ToolCallRecord{}, err
		}
	}
	if _, err := s.repo.RecordToolApproval(ctx, ToolApprovalRecord{
		WorkspaceID:    principal.WorkspaceID,
		ToolCallID:     call.ID,
		ToolID:         call.ToolID,
		AgentID:        agent.ID,
		ConversationID: conversation.ID,
		ActorUserID:    principal.UserID,
		Decision:       decision,
	}); err != nil {
		return ToolCallRecord{}, err
	}
	if err := s.repo.UpdateToolCallState(ctx, call.ID, ToolCallApproved, ToolApprovalApproved, "", ""); err != nil {
		return ToolCallRecord{}, err
	}
	updated, _, _, err := s.repo.GetToolCall(ctx, principal.WorkspaceID, principal.UserID, toolCallID)
	return updated, err
}

func (s *Service) DenyToolCall(ctx context.Context, principal PrincipalContext, toolCallID string, decision string) (ToolCallRecord, error) {
	if decision == "" {
		decision = ToolDecisionDeny
	}
	if decision != ToolDecisionDeny && decision != ToolDecisionDenyDisableTool {
		return ToolCallRecord{}, fmt.Errorf("%w: denial decision is invalid", ErrInvalidInput)
	}
	call, _, conversation, err := s.repo.GetToolCall(ctx, principal.WorkspaceID, principal.UserID, toolCallID)
	if err != nil {
		return ToolCallRecord{}, err
	}
	if call.State != ToolCallWaitingForApproval && call.State != ToolCallApproved && call.State != ToolCallDenied {
		return ToolCallRecord{}, fmt.Errorf("%w: tool call is not waiting for a decision", ErrInvalidInput)
	}
	agent, err := s.resolveAgent(ctx, conversation)
	if err != nil {
		return ToolCallRecord{}, err
	}
	if decision == ToolDecisionDenyDisableTool {
		if s.tools == nil {
			return ToolCallRecord{}, errors.New("tool provider is unavailable")
		}
		if err := s.tools.DisableTool(ctx, principal.WorkspaceID, call.ToolID); err != nil {
			return ToolCallRecord{}, err
		}
	}
	if _, err := s.repo.RecordToolApproval(ctx, ToolApprovalRecord{
		WorkspaceID:    principal.WorkspaceID,
		ToolCallID:     call.ID,
		ToolID:         call.ToolID,
		AgentID:        agent.ID,
		ConversationID: conversation.ID,
		ActorUserID:    principal.UserID,
		Decision:       decision,
	}); err != nil {
		return ToolCallRecord{}, err
	}
	if err := s.repo.UpdateToolCallState(ctx, call.ID, ToolCallDenied, ToolApprovalDenied, "", "Tool call denied by the workspace owner."); err != nil {
		return ToolCallRecord{}, err
	}
	updated, _, _, err := s.repo.GetToolCall(ctx, principal.WorkspaceID, principal.UserID, toolCallID)
	return updated, err
}

func (s *Service) ResumeRun(ctx context.Context, principal PrincipalContext, runID string, sink StreamSink) error {
	run, err := s.repo.GetRun(ctx, principal.WorkspaceID, principal.UserID, runID)
	if err != nil {
		return err
	}
	conversation, err := s.repo.GetConversation(ctx, principal.WorkspaceID, principal.UserID, run.ConversationID)
	if err != nil {
		return err
	}
	agent, err := s.resolveAgent(ctx, conversation)
	if err != nil {
		return err
	}
	if run.ProviderID == "" {
		return fmt.Errorf("%w: run provider is missing", ErrInvalidInput)
	}
	provider, apiKey, err := s.providers.ResolveForChat(ctx, principal.WorkspaceID, run.ProviderID)
	if err != nil {
		return err
	}
	var memories []MemorySnippet
	if s.memories != nil {
		memories, err = s.memories.UsedByRun(ctx, run.ID)
		if err != nil {
			return err
		}
	}
	promptMessages, err := s.contextMessages(ctx, principal, conversation, agent, memories, run.UserMessageID, "", run.BranchID, "")
	if err != nil {
		return err
	}
	toolCalls, err := s.repo.ListToolCallsForRun(ctx, run.ID)
	if err != nil {
		return err
	}
	for _, call := range toolCalls {
		if call.State == ToolCallWaitingForApproval {
			return ErrApprovalRequired
		}
	}
	runtimeTools, err := s.runtimeTools(ctx, principal, conversation, agent)
	if err != nil {
		return err
	}
	request := modelLoopRequest{
		Principal:          principal,
		Conversation:       conversation,
		Agent:              agent,
		Run:                run,
		AssistantMessageID: run.AssistantMessageID,
		Provider:           provider,
		APIKey:             apiKey,
		Model:              run.Model,
		Memories:           memories,
		BranchID:           run.BranchID,
		Messages:           promptMessages,
		Tools:              runtimeTools,
	}
	messages := append([]providers.ChatMessage{}, promptMessages...)
	for _, call := range toolCalls {
		if call.State != ToolCallApproved && call.State != ToolCallDenied {
			continue
		}
		toolMessage, err := s.executePersistedToolCall(ctx, request, call, sink)
		if err != nil {
			return err
		}
		messages = append(messages, toolMessage)
	}
	request.Messages = messages
	return s.executeModelLoop(ctx, request, sink)
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
	promptMessages, err := s.contextMessages(ctx, principal, conversation, agent, memories, userMessage.ID, assistantMessage.ID, branch.ID, input.RegenerationInstruction)
	if err != nil {
		return err
	}
	runtimeTools, err := s.runtimeTools(ctx, principal, conversation, agent)
	if err != nil {
		_ = s.repo.UpdateRunState(ctx, run.ID, RunFailed, "tool_resolution_failed", err.Error(), true)
		return err
	}
	return s.executeModelLoop(ctx, modelLoopRequest{
		Principal:          principal,
		Conversation:       conversation,
		Agent:              agent,
		Run:                run,
		AssistantMessageID: assistantMessage.ID,
		Provider:           provider,
		APIKey:             apiKey,
		Model:              model,
		Memories:           memories,
		BranchID:           branch.ID,
		Messages:           promptMessages,
		Tools:              runtimeTools,
	}, sink)
}

func (s *Service) CleanupInterruptedRuns(ctx context.Context) error {
	_, err := s.repo.CleanupInterruptedRuns(ctx, time.Now().UTC())
	return err
}

func (s *Service) CleanupAbandonedChatRuns(ctx context.Context) (string, error) {
	count, err := s.repo.CleanupInterruptedRuns(ctx, time.Now().UTC())
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("abandoned chat runs cleaned=%d", count), nil
}

func (s *Service) RecalculateConversationTitles(ctx context.Context, limit int) (string, error) {
	count, err := s.repo.RecalculateConversationTitles(ctx, limit)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("conversation titles recalculated=%d", count), nil
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

type modelLoopRequest struct {
	Principal          PrincipalContext
	Conversation       Conversation
	Agent              AgentContext
	Run                ChatRun
	AssistantMessageID string
	Provider           providers.Provider
	APIKey             string
	Model              string
	Memories           []MemorySnippet
	BranchID           string
	Messages           []providers.ChatMessage
	Tools              []RuntimeTool
}

func (s *Service) executeModelLoop(ctx context.Context, request modelLoopRequest, sink StreamSink) error {
	messages := append([]providers.ChatMessage{}, request.Messages...)
	toolByProviderName := map[string]RuntimeTool{}
	for _, tool := range request.Tools {
		toolByProviderName[tool.ProviderName] = tool
	}
	maxIterations := s.maxToolIterations(request.Agent)
	providerTools := providerTools(request.Tools)
	for iteration := 0; ; iteration++ {
		if err := s.repo.UpdateRunState(ctx, request.Run.ID, RunStreaming, "", "", false); err != nil {
			return err
		}
		events, err := s.client.StreamChat(ctx, providers.StreamRequest{
			Provider:    request.Provider,
			APIKey:      request.APIKey,
			Model:       request.Model,
			Messages:    messages,
			Tools:       providerTools,
			Temperature: request.Agent.Temperature,
		})
		if err != nil {
			_ = s.repo.UpdateRunState(ctx, request.Run.ID, RunFailed, "provider_unavailable", err.Error(), true)
			_ = sink("run_failed", map[string]any{"run_id": request.Run.ID, "message": err.Error()})
			return err
		}
		content, usage, toolCalls, err := s.consumeModelStream(ctx, request.Run.ID, request.AssistantMessageID, events, sink)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		if len(toolCalls) == 0 {
			_ = s.repo.UpdateMessageContent(ctx, request.AssistantMessageID, content, usage)
			_ = s.repo.UpdateRunState(ctx, request.Run.ID, RunCompleted, "", "", true)
			s.emitSummaryQueueEvent(ctx, request.Principal, request.Conversation, request.Agent, request.Memories, request.BranchID, sink)
			return sink("run_completed", map[string]any{"run_id": request.Run.ID, "assistant_message_id": request.AssistantMessageID})
		}
		if iteration >= maxIterations {
			message := "The tool iteration limit was reached before the model produced a final answer."
			_ = s.repo.UpdateRunState(ctx, request.Run.ID, RunFailed, "tool_iteration_limit", message, true)
			_ = sink("run_failed", map[string]any{"run_id": request.Run.ID, "message": message})
			return errors.New(message)
		}
		_ = s.repo.UpdateMessageContentAndToolCalls(ctx, request.AssistantMessageID, content, usage, toolCalls)
		persistedCalls, needsApproval, err := s.persistRequestedToolCalls(ctx, request, toolCalls, toolByProviderName)
		if err != nil {
			_ = s.repo.UpdateRunState(ctx, request.Run.ID, RunFailed, "tool_resolution_failed", err.Error(), true)
			_ = sink("run_failed", map[string]any{"run_id": request.Run.ID, "message": err.Error()})
			return err
		}
		if needsApproval {
			_ = s.repo.UpdateRunState(ctx, request.Run.ID, RunWaitingForApproval, "", "", false)
			return sink("tool_approval_required", map[string]any{"run_id": request.Run.ID, "tool_calls": persistedCalls})
		}
		messages = append(messages, providers.ChatMessage{Role: RoleAssistant, Content: content, ToolCalls: toolCalls})
		for _, call := range persistedCalls {
			toolMessage, err := s.executePersistedToolCall(ctx, request, call, sink)
			if err != nil {
				return err
			}
			messages = append(messages, toolMessage)
		}
	}
}

func (s *Service) consumeModelStream(ctx context.Context, runID string, assistantMessageID string, events <-chan providers.StreamEvent, sink StreamSink) (string, UsageValues, []providers.ToolCall, error) {
	var contentBuilder strings.Builder
	var usage UsageValues
	var toolCalls []providers.ToolCall
	for event := range events {
		cancelled, err := s.repo.CancellationRequested(ctx, runID)
		if err != nil {
			return "", usage, nil, err
		}
		if cancelled {
			_ = s.repo.UpdateRunState(ctx, runID, RunCancelled, "", "", true)
			_ = sink("run_cancelled", map[string]any{"run_id": runID})
			return "", usage, nil, context.Canceled
		}
		if event.Error != nil {
			_ = s.repo.UpdateRunState(ctx, runID, RunFailed, "provider_stream_failed", event.Error.Error(), true)
			_ = sink("run_failed", map[string]any{"run_id": runID, "message": event.Error.Error()})
			return "", usage, nil, event.Error
		}
		switch event.Type {
		case "content_delta":
			contentBuilder.WriteString(event.Content)
			_ = s.repo.UpdateMessageContent(ctx, assistantMessageID, contentBuilder.String(), usage)
			if err := sink("content_delta", map[string]string{"delta": event.Content}); err != nil {
				return "", usage, nil, err
			}
		case "reasoning_delta":
			if err := sink("reasoning_delta", map[string]string{"delta": event.Reasoning}); err != nil {
				return "", usage, nil, err
			}
		case "tool_call_delta":
			if event.ToolCall != nil {
				toolCalls = mergeToolCall(toolCalls, *event.ToolCall)
			}
			if err := sink(event.Type, event.ToolCall); err != nil {
				return "", usage, nil, err
			}
		case "tool_call_ready":
			if err := sink(event.Type, map[string]any{"run_id": runID, "tool_calls": toolCalls}); err != nil {
				return "", usage, nil, err
			}
		case "usage":
			if event.Usage != nil {
				usage = UsageValues{PromptTokens: event.Usage.PromptTokens, CompletionTokens: event.Usage.CompletionTokens, TotalTokens: event.Usage.TotalTokens}
				_ = s.repo.UpdateRunUsage(ctx, runID, usage)
				_ = s.repo.UpdateMessageContent(ctx, assistantMessageID, contentBuilder.String(), usage)
				if err := sink("usage", usage); err != nil {
					return "", usage, nil, err
				}
			}
		case "run_completed":
			return contentBuilder.String(), usage, toolCalls, nil
		}
	}
	return contentBuilder.String(), usage, toolCalls, nil
}

func (s *Service) persistRequestedToolCalls(ctx context.Context, request modelLoopRequest, calls []providers.ToolCall, tools map[string]RuntimeTool) ([]ToolCallRecord, bool, error) {
	records := make([]ToolCallRecord, 0, len(calls))
	needsApproval := false
	for _, call := range calls {
		tool, ok := tools[call.Function.Name]
		if !ok {
			return nil, false, fmt.Errorf("%w: tool %q is not available to this agent", ErrInvalidInput, call.Function.Name)
		}
		state := ToolCallApproved
		approvalState := ToolApprovalNotRequired
		if tool.PermissionMode == ToolPermissionAsk {
			state = ToolCallWaitingForApproval
			approvalState = ToolApprovalPending
			needsApproval = true
		}
		record, err := s.repo.CreateToolCall(ctx, ToolCallRecord{
			ChatRunID:          request.Run.ID,
			MessageID:          request.AssistantMessageID,
			ToolID:             tool.ID,
			ProviderToolCallID: call.ID,
			ProviderName:       tool.ProviderName,
			Name:               tool.Name,
			Input:              call.Function.Arguments,
			State:              state,
			ApprovalState:      approvalState,
		})
		if err != nil {
			return nil, false, err
		}
		records = append(records, record)
	}
	return records, needsApproval, nil
}

func (s *Service) executePersistedToolCall(ctx context.Context, request modelLoopRequest, call ToolCallRecord, sink StreamSink) (providers.ChatMessage, error) {
	if call.State == ToolCallSucceeded {
		return providers.ChatMessage{Role: RoleTool, ToolCallID: call.ProviderToolCallID, Content: call.Output}, nil
	}
	if call.State == ToolCallDenied {
		content := call.Output
		if content == "" {
			content = "Tool call denied by the workspace owner."
			_ = s.repo.CompleteToolCall(ctx, call.ID, ToolCallDenied, content, false, "", "Tool call denied by the workspace owner.")
			_, _ = s.repo.CreateMessage(ctx, Message{
				ConversationID:  request.Conversation.ID,
				BranchID:        request.BranchID,
				ParentMessageID: request.AssistantMessageID,
				Role:            RoleTool,
				Content:         content,
				ToolCallID:      call.ProviderToolCallID,
				ProviderID:      request.Provider.ID,
				Model:           request.Model,
			})
			_ = sink("tool_result", map[string]any{"tool_call_id": call.ID, "provider_tool_call_id": call.ProviderToolCallID, "name": call.Name, "result": content, "denied": true})
		}
		return providers.ChatMessage{Role: RoleTool, ToolCallID: call.ProviderToolCallID, Content: content}, nil
	}
	if call.ApprovalState == ToolApprovalPending {
		return providers.ChatMessage{}, ErrApprovalRequired
	}
	if s.tools == nil {
		return providers.ChatMessage{}, errors.New("tool provider is unavailable")
	}
	if err := s.repo.UpdateToolCallState(ctx, call.ID, ToolCallRunning, call.ApprovalState, "", ""); err != nil {
		return providers.ChatMessage{}, err
	}
	result, err := s.tools.ExecuteRuntimeTool(ctx, ToolExecutionRequest{
		WorkspaceID:     request.Principal.WorkspaceID,
		ToolID:          call.ToolID,
		Arguments:       call.Input,
		Timeout:         s.toolTimeout(),
		MaxResultBytes:  32 * 1024,
		ProviderName:    call.ProviderName,
		ToolDisplayName: call.Name,
	})
	if err != nil {
		state := ToolCallFailed
		code := "tool_execution_failed"
		if errors.Is(err, context.DeadlineExceeded) {
			state = ToolCallTimedOut
			code = "tool_timeout"
		}
		_ = s.repo.CompleteToolCall(ctx, call.ID, state, "", false, code, err.Error())
		_ = s.repo.UpdateRunState(ctx, request.Run.ID, RunFailed, code, err.Error(), true)
		_ = sink("run_failed", map[string]any{"run_id": request.Run.ID, "message": err.Error()})
		return providers.ChatMessage{}, err
	}
	if err := s.repo.CompleteToolCall(ctx, call.ID, ToolCallSucceeded, result.Content, result.Truncated, "", ""); err != nil {
		return providers.ChatMessage{}, err
	}
	if _, err := s.repo.CreateMessage(ctx, Message{
		ConversationID:  request.Conversation.ID,
		BranchID:        request.BranchID,
		ParentMessageID: request.AssistantMessageID,
		Role:            RoleTool,
		Content:         result.Content,
		ToolCallID:      call.ProviderToolCallID,
		ProviderID:      request.Provider.ID,
		Model:           request.Model,
	}); err != nil {
		return providers.ChatMessage{}, err
	}
	if err := sink("tool_result", map[string]any{"tool_call_id": call.ID, "provider_tool_call_id": call.ProviderToolCallID, "name": call.Name, "result": result.Content, "truncated": result.Truncated}); err != nil {
		return providers.ChatMessage{}, err
	}
	return providers.ChatMessage{Role: RoleTool, ToolCallID: call.ProviderToolCallID, Content: result.Content}, nil
}

func (s *Service) runtimeTools(ctx context.Context, principal PrincipalContext, conversation Conversation, agent AgentContext) ([]RuntimeTool, error) {
	if s.tools == nil || agent.ID == "" {
		return nil, nil
	}
	return s.tools.RuntimeTools(ctx, ToolExposureRequest{
		WorkspaceID:            principal.WorkspaceID,
		AgentID:                agent.ID,
		ConversationID:         conversation.ID,
		AgentDefaultPermission: agent.ToolPermissionDefault,
	})
}

func providerTools(tools []RuntimeTool) []providers.ChatTool {
	out := make([]providers.ChatTool, 0, len(tools))
	for _, tool := range tools {
		parameters := json.RawMessage(`{}`)
		if strings.TrimSpace(tool.InputSchema) != "" && json.Valid([]byte(tool.InputSchema)) {
			parameters = json.RawMessage(tool.InputSchema)
		}
		out = append(out, providers.ChatTool{
			Type: "function",
			Function: providers.ChatToolFunction{
				Name:        tool.ProviderName,
				Description: tool.Description,
				Parameters:  parameters,
			},
		})
	}
	return out
}

func (s *Service) maxToolIterations(agent AgentContext) int {
	limit := agent.MaxToolIterations
	if limit <= 0 {
		limit = s.cfg.Chat.MaxToolIterations
	}
	if s.cfg.Chat.MaxToolIterations > 0 && limit > s.cfg.Chat.MaxToolIterations {
		limit = s.cfg.Chat.MaxToolIterations
	}
	if limit <= 0 {
		limit = 8
	}
	return limit
}

func (s *Service) toolTimeout() time.Duration {
	if s.cfg.Chat.DefaultTimeout <= 0 {
		return 30 * time.Second
	}
	return s.cfg.Chat.DefaultTimeout
}

func (s *Service) resolveAgent(ctx context.Context, conversation Conversation) (AgentContext, error) {
	if s.agents == nil || conversation.AgentID == "" {
		return AgentContext{MemoryAccessMode: "pinned_only", ToolPermissionDefault: ToolPermissionAsk, MaxToolIterations: s.cfg.Chat.MaxToolIterations, Temperature: 0.7, Active: true}, nil
	}
	agent, err := s.agents.GetChatAgent(ctx, conversation.WorkspaceID, conversation.AgentID)
	if err != nil {
		return AgentContext{}, err
	}
	if agent.MemoryAccessMode == "" {
		agent.MemoryAccessMode = "pinned_only"
	}
	if agent.ToolPermissionDefault == "" {
		agent.ToolPermissionDefault = ToolPermissionAsk
	}
	if agent.MaxToolIterations <= 0 {
		agent.MaxToolIterations = s.cfg.Chat.MaxToolIterations
	}
	if agent.Temperature <= 0 {
		agent.Temperature = 0.7
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
	internalInstruction string,
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
		InternalInstruction:    internalInstruction,
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
