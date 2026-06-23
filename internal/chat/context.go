package chat

import (
	"strings"

	"github.com/isyuricunha/nostos/internal/providers"
)

const outputTokenReserveRatio = 0.2

type ContextRequest struct {
	Conversation           Conversation
	Agent                  AgentContext
	Memories               []MemorySnippet
	Messages               []Message
	CurrentUserMessageID   string
	AssistantPlaceholderID string
	BranchID               string
	InternalInstruction    string
	RecentMessageLimit     int
	ContextThreshold       int
}

type ContextBuildResult struct {
	Messages        []providers.ChatMessage
	EstimatedTokens int
	TrimmedMessages int
}

func BuildPromptMessages(request ContextRequest) ContextBuildResult {
	systemMessages := buildSystemMessages(request.Conversation, request.Agent, request.Memories, request.InternalInstruction)
	history := branchAwareHistory(request)
	trimmed := 0
	if request.RecentMessageLimit > 0 && len(history) > request.RecentMessageLimit {
		trimmed = len(history) - request.RecentMessageLimit
		history = history[trimmed:]
	}

	result := joinPromptMessages(systemMessages, history)
	estimated := estimateMessages(result)
	if request.ContextThreshold <= 0 || estimated <= reservedContextBudget(request.ContextThreshold) {
		return ContextBuildResult{Messages: result, EstimatedTokens: estimated, TrimmedMessages: trimmed}
	}

	budget := reservedContextBudget(request.ContextThreshold)
	for len(history) > 1 && estimateMessages(joinPromptMessages(systemMessages, history)) > budget {
		history = history[1:]
		trimmed++
	}
	result = joinPromptMessages(systemMessages, history)
	return ContextBuildResult{Messages: result, EstimatedTokens: estimateMessages(result), TrimmedMessages: trimmed}
}

func joinPromptMessages(systemMessages []providers.ChatMessage, history []providers.ChatMessage) []providers.ChatMessage {
	result := make([]providers.ChatMessage, 0, len(systemMessages)+len(history))
	result = append(result, systemMessages...)
	result = append(result, history...)
	return result
}

func buildSystemMessages(conversation Conversation, agent AgentContext, memories []MemorySnippet, internalInstruction string) []providers.ChatMessage {
	messages := make([]providers.ChatMessage, 0, 4)
	if strings.TrimSpace(agent.SystemPrompt) != "" {
		messages = append(messages, providers.ChatMessage{Role: RoleSystem, Content: strings.TrimSpace(agent.SystemPrompt)})
	}
	messages = append(messages, providers.ChatMessage{
		Role:    RoleSystem,
		Content: "Use the persisted conversation history below as the active context. Treat explicit memories as user-controlled context, not automatically inferred facts.",
	})
	if strings.TrimSpace(conversation.Summary) != "" {
		messages = append(messages, providers.ChatMessage{Role: RoleSystem, Content: "Conversation summary:\n" + strings.TrimSpace(conversation.Summary)})
	}
	if len(memories) > 0 {
		var builder strings.Builder
		builder.WriteString("Explicit memories selected for this run:\n")
		for _, memory := range memories {
			builder.WriteString("- ")
			builder.WriteString(memory.Title)
			builder.WriteString(": ")
			builder.WriteString(memory.Content)
			builder.WriteString("\n")
		}
		messages = append(messages, providers.ChatMessage{Role: RoleSystem, Content: strings.TrimSpace(builder.String())})
	}
	if instruction := strings.TrimSpace(internalInstruction); instruction != "" {
		messages = append(messages, providers.ChatMessage{Role: RoleSystem, Content: "Internal instruction for this generation only:\n" + instruction})
	}
	return messages
}

func branchAwareHistory(request ContextRequest) []providers.ChatMessage {
	selected := selectContextMessages(request)
	history := make([]providers.ChatMessage, 0, len(selected))
	currentIncluded := false
	for _, message := range selected {
		if message.ID == request.AssistantPlaceholderID {
			continue
		}
		if message.Role == RoleAssistant && strings.TrimSpace(message.Content) == "" && len(message.ToolCalls) == 0 {
			continue
		}
		if message.ID == request.CurrentUserMessageID {
			if currentIncluded {
				continue
			}
			currentIncluded = true
		}
		converted, ok := providerMessage(message)
		if ok {
			history = append(history, converted)
		}
	}
	return history
}

func selectContextMessages(request ContextRequest) []Message {
	if request.BranchID == "" {
		out := make([]Message, 0, len(request.Messages))
		for _, message := range request.Messages {
			if message.BranchID == "" {
				out = append(out, message)
			}
		}
		return out
	}

	var branchMessages []Message
	var branchParent Message
	for _, message := range request.Messages {
		if message.BranchID == request.BranchID {
			branchMessages = append(branchMessages, message)
			if message.ID == request.CurrentUserMessageID && message.ParentMessageID != "" {
				branchParent = findMessage(request.Messages, message.ParentMessageID)
			}
		}
	}
	var out []Message
	if branchParent.ID != "" {
		for _, message := range request.Messages {
			if message.BranchID == "" && !message.CreatedAt.After(branchParent.CreatedAt) {
				out = append(out, message)
			}
		}
	}
	out = append(out, branchMessages...)
	return out
}

func providerMessage(message Message) (providers.ChatMessage, bool) {
	switch message.Role {
	case RoleSystem, RoleUser:
		return providers.ChatMessage{Role: message.Role, Content: message.Content}, true
	case RoleAssistant:
		return providers.ChatMessage{Role: RoleAssistant, Content: message.Content, ToolCalls: message.ToolCalls}, true
	case RoleTool:
		if strings.TrimSpace(message.ToolCallID) == "" {
			return providers.ChatMessage{}, false
		}
		return providers.ChatMessage{Role: RoleTool, ToolCallID: message.ToolCallID, Content: message.Content}, true
	default:
		return providers.ChatMessage{}, false
	}
}

func findMessage(messages []Message, id string) Message {
	for _, message := range messages {
		if message.ID == id {
			return message
		}
	}
	return Message{}
}

func estimateMessages(messages []providers.ChatMessage) int {
	total := 0
	for _, message := range messages {
		total += estimateTextTokens(message.Role)
		total += estimateTextTokens(message.Content)
		total += estimateTextTokens(message.ToolCallID)
		for _, call := range message.ToolCalls {
			total += estimateTextTokens(call.ID)
			total += estimateTextTokens(call.Function.Name)
			total += estimateTextTokens(call.Function.Arguments)
		}
	}
	return total
}

func estimateTextTokens(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	return (len(value) / 4) + 1
}

func reservedContextBudget(threshold int) int {
	reserve := int(float64(threshold) * outputTokenReserveRatio)
	if reserve < 512 {
		reserve = 512
	}
	if threshold-reserve < 1 {
		return threshold
	}
	return threshold - reserve
}
