package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/isyuricunha/nostos/internal/auth"
	"github.com/isyuricunha/nostos/internal/chat"
	"github.com/isyuricunha/nostos/internal/config"
	"github.com/isyuricunha/nostos/internal/providers"
)

var ErrInvalidInput = errors.New("invalid task input")

type Service struct {
	cfg             config.Config
	repo            Repository
	audit           auth.Repository
	providers       ProviderResolver
	client          *providers.OpenAIClient
	agents          AgentResolver
	memories        MemoryProvider
	tools           ToolProvider
	summaries       ConversationSummaryHandler
	chatMaintenance ChatMaintenanceHandler
	mcpHealth       MCPHealthHandler
}

type ProviderResolver interface {
	ResolveForChat(ctx context.Context, workspaceID string, providerID string) (providers.Provider, string, error)
}

type AgentResolver interface {
	GetChatAgent(ctx context.Context, workspaceID string, agentID string) (chat.AgentContext, error)
}

type MemoryProvider interface {
	SelectForRun(ctx context.Context, request chat.MemoryRequest) ([]chat.MemorySnippet, error)
}

type ToolProvider interface {
	RuntimeTools(ctx context.Context, request chat.ToolExposureRequest) ([]chat.RuntimeTool, error)
	ExecuteRuntimeTool(ctx context.Context, request chat.ToolExecutionRequest) (chat.ToolExecutionResult, error)
}

type ConversationSummaryHandler interface {
	UpdateConversationSummaries(ctx context.Context, limit int) (string, error)
}

type ChatMaintenanceHandler interface {
	CleanupAbandonedChatRuns(ctx context.Context) (string, error)
	RecalculateConversationTitles(ctx context.Context, limit int) (string, error)
}

type MCPHealthHandler interface {
	CheckMCPServerHealth(ctx context.Context, limit int) (string, error)
}

type ProviderHealthHandler interface {
	CheckProviderHealth(ctx context.Context, limit int) (string, error)
}

func NewService(cfg config.Config, repo Repository, audit auth.Repository) *Service {
	return &Service{cfg: cfg, repo: repo, audit: audit}
}

func (s *Service) WithProviderClient(resolver ProviderResolver, client *providers.OpenAIClient) *Service {
	s.providers = resolver
	s.client = client
	return s
}

func (s *Service) WithAgentRuntime(agentResolver AgentResolver, memoryProvider MemoryProvider, toolProvider ToolProvider) *Service {
	s.agents = agentResolver
	s.memories = memoryProvider
	s.tools = toolProvider
	return s
}

func (s *Service) WithConversationSummaryHandler(handler ConversationSummaryHandler) *Service {
	s.summaries = handler
	return s
}

func (s *Service) WithChatMaintenanceHandler(handler ChatMaintenanceHandler) *Service {
	s.chatMaintenance = handler
	return s
}

func (s *Service) WithMCPHealthHandler(handler MCPHealthHandler) *Service {
	s.mcpHealth = handler
	return s
}

func (s *Service) EnsureSystemTasks(ctx context.Context) error {
	workspaces, err := s.repo.Workspaces(ctx)
	if err != nil {
		return err
	}
	for _, workspaceID := range workspaces {
		for _, name := range systemTaskNames() {
			exists, err := s.repo.SystemTaskExists(ctx, workspaceID, name)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			task, schedule, err := s.normalizeInput(workspaceID, TaskInput{
				Name:              name,
				Description:       "System-managed maintenance task.",
				TaskType:          TaskTypeSystem,
				State:             TaskEnabled,
				Prompt:            name,
				ToolPolicy:        "use_preapproved_tools_only",
				MaxRetries:        s.cfg.Tasks.MaxRetries,
				TimeoutMS:         int(s.cfg.Tasks.DefaultTimeout / time.Millisecond),
				ConcurrencyPolicy: "skip",
				ScheduleMode:      "interval",
				IntervalSeconds:   3600,
				Timezone:          s.cfg.Timezone,
			})
			if err != nil {
				return err
			}
			task.SystemManaged = true
			if _, _, err := s.repo.CreateTask(ctx, task, schedule); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) ListTasks(ctx context.Context, principal PrincipalContext) ([]Task, error) {
	if err := s.ensureSystemTasksForWorkspace(ctx, principal.WorkspaceID); err != nil {
		return nil, err
	}
	return s.repo.ListTasks(ctx, principal.WorkspaceID)
}

func (s *Service) ListTaskRecords(ctx context.Context, principal PrincipalContext) ([]TaskRecord, error) {
	tasks, err := s.ListTasks(ctx, principal)
	if err != nil {
		return nil, err
	}
	records := make([]TaskRecord, 0, len(tasks))
	for _, task := range tasks {
		schedule, err := s.repo.GetSchedule(ctx, task.ID)
		if err != nil {
			return nil, err
		}
		records = append(records, TaskRecord{Task: task, Schedule: schedule})
	}
	return records, nil
}

func (s *Service) CreateTask(ctx context.Context, principal PrincipalContext, input TaskInput) (Task, Schedule, error) {
	task, schedule, err := s.normalizeInput(principal.WorkspaceID, input)
	if err != nil {
		return Task{}, Schedule{}, err
	}
	created, createdSchedule, err := s.repo.CreateTask(ctx, task, schedule)
	if err != nil {
		return Task{}, Schedule{}, err
	}
	s.auditEvent(ctx, principal, auth.AuditTaskCreated, created.ID)
	return created, createdSchedule, nil
}

func (s *Service) UpdateTask(ctx context.Context, principal PrincipalContext, taskID string, input TaskInput) (Task, Schedule, error) {
	task, schedule, err := s.normalizeInput(principal.WorkspaceID, input)
	if err != nil {
		return Task{}, Schedule{}, err
	}
	task.ID = taskID
	existing, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return Task{}, Schedule{}, err
	}
	if existing.WorkspaceID != principal.WorkspaceID {
		return Task{}, Schedule{}, ErrNotFound
	}
	task.SystemManaged = existing.SystemManaged
	if task.SystemManaged {
		task.TaskType = existing.TaskType
	}
	schedule.TaskID = taskID
	updated, updatedSchedule, err := s.repo.UpdateTask(ctx, task, schedule)
	if err != nil {
		return Task{}, Schedule{}, err
	}
	s.auditEvent(ctx, principal, auth.AuditTaskUpdated, taskID)
	return updated, updatedSchedule, nil
}

func (s *Service) DeleteTask(ctx context.Context, principal PrincipalContext, taskID string) error {
	existing, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if existing.WorkspaceID != principal.WorkspaceID {
		return ErrNotFound
	}
	if existing.SystemManaged {
		return fmt.Errorf("%w: system-managed tasks cannot be deleted", ErrInvalidInput)
	}
	if err := s.repo.DeleteTask(ctx, principal.WorkspaceID, taskID); err != nil {
		return err
	}
	s.auditEvent(ctx, principal, auth.AuditTaskDeleted, taskID)
	return nil
}

func (s *Service) RunNow(ctx context.Context, principal PrincipalContext, taskID string) (Run, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return Run{}, err
	}
	if task.WorkspaceID != principal.WorkspaceID {
		return Run{}, ErrNotFound
	}
	return s.enqueueTask(ctx, task, "", "manual:"+task.ID+":"+time.Now().UTC().Format(time.RFC3339Nano), 0)
}

func (s *Service) EnqueueConversationSummary(ctx context.Context, workspaceID string, conversationID string) error {
	if err := s.ensureSystemTasksForWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	task, err := s.repo.GetSystemTaskByName(ctx, workspaceID, "update_conversation_summaries")
	if err != nil {
		return err
	}
	_, err = s.enqueueTask(ctx, task, "", "summary:"+conversationID+":"+time.Now().UTC().Format(time.RFC3339Nano), 0)
	return err
}

func (s *Service) CancelRun(ctx context.Context, principal PrincipalContext, runID string) error {
	if err := s.repo.CancelRun(ctx, principal.WorkspaceID, runID); err != nil {
		return err
	}
	s.auditEvent(ctx, principal, auth.AuditTaskRunCancelled, runID)
	return nil
}

func (s *Service) ListRuns(ctx context.Context, principal PrincipalContext, taskID string) ([]Run, error) {
	return s.repo.ListRuns(ctx, principal.WorkspaceID, taskID)
}

func (s *Service) GetRunRecord(ctx context.Context, principal PrincipalContext, runID string) (RunRecord, error) {
	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return RunRecord{}, err
	}
	task, err := s.repo.GetTask(ctx, run.TaskID)
	if err != nil {
		return RunRecord{}, err
	}
	if task.WorkspaceID != principal.WorkspaceID {
		return RunRecord{}, ErrNotFound
	}
	events, err := s.repo.ListEvents(ctx, run.ID)
	if err != nil {
		return RunRecord{}, err
	}
	return RunRecord{Run: run, Events: events}, nil
}

func (s *Service) RetryRun(ctx context.Context, principal PrincipalContext, runID string) (Run, error) {
	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return Run{}, err
	}
	task, err := s.repo.GetTask(ctx, run.TaskID)
	if err != nil {
		return Run{}, err
	}
	if task.WorkspaceID != principal.WorkspaceID {
		return Run{}, ErrNotFound
	}
	return s.enqueueTask(ctx, task, run.ScheduleID, "retry:"+run.ID+":"+time.Now().UTC().Format(time.RFC3339Nano), run.Attempt+1)
}

func (s *Service) EnqueueDueSchedules(ctx context.Context) error {
	now := time.Now().UTC()
	schedules, err := s.repo.DueSchedules(ctx, now)
	if err != nil {
		return err
	}
	for _, schedule := range schedules {
		task, err := s.repo.GetTask(ctx, schedule.TaskID)
		if err != nil || task.State != TaskEnabled {
			continue
		}
		occurrence := schedule.NextRunAt
		if occurrence == nil {
			continue
		}
		key := "schedule:" + schedule.ID + ":" + occurrence.UTC().Format(time.RFC3339)
		_, _ = s.enqueueTask(ctx, task, schedule.ID, key, 0)
		next := nextRun(schedule, now, s.cfg.Timezone)
		schedule.LastEnqueuedOccurrence = occurrence.UTC().Format(time.RFC3339)
		schedule.NextRunAt = next
		_ = s.repo.MarkScheduleEnqueued(ctx, schedule)
	}
	return nil
}

func (s *Service) ClaimAndExecute(ctx context.Context, workerID string) error {
	run, err := s.repo.ClaimRun(ctx, workerID, time.Now().UTC().Add(s.cfg.Tasks.DefaultTimeout))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	task, err := s.repo.GetTask(ctx, run.TaskID)
	if err != nil {
		_ = s.repo.CompleteRun(ctx, run.ID, RunFailed, "", err.Error())
		return err
	}
	_ = s.repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "info", Message: "Task run started."})
	timeout := time.Duration(run.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = s.cfg.Tasks.DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, execErr := s.execute(runCtx, run, task)
	if execErr != nil {
		state := RunFailed
		if errors.Is(execErr, context.DeadlineExceeded) {
			state = RunTimedOut
		}
		_ = s.repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "error", Message: execErr.Error()})
		if err := s.repo.CompleteRun(ctx, run.ID, state, "", execErr.Error()); err != nil {
			return err
		}
		if state == RunFailed && run.Attempt < run.MaxRetries {
			_, _ = s.enqueueTask(ctx, task, run.ScheduleID, "retry:"+run.ID+":"+time.Now().UTC().Format(time.RFC3339Nano), run.Attempt+1)
		}
		return nil
	}
	_ = s.repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "info", Message: "Task run succeeded."})
	return s.repo.CompleteRun(ctx, run.ID, RunSucceeded, result, "")
}

func (s *Service) RecoverExpiredLeases(ctx context.Context) error {
	_, err := s.repo.RecoverExpiredLeases(ctx, time.Now().UTC())
	return err
}

func (s *Service) ensureSystemTasksForWorkspace(ctx context.Context, workspaceID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return nil
	}
	for _, name := range systemTaskNames() {
		exists, err := s.repo.SystemTaskExists(ctx, workspaceID, name)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		task, schedule, err := s.normalizeInput(workspaceID, TaskInput{
			Name:              name,
			Description:       "System-managed maintenance task.",
			TaskType:          TaskTypeSystem,
			State:             TaskEnabled,
			Prompt:            name,
			ToolPolicy:        "use_preapproved_tools_only",
			MaxRetries:        s.cfg.Tasks.MaxRetries,
			TimeoutMS:         int(s.cfg.Tasks.DefaultTimeout / time.Millisecond),
			ConcurrencyPolicy: "skip",
			ScheduleMode:      "interval",
			IntervalSeconds:   3600,
			Timezone:          s.cfg.Timezone,
		})
		if err != nil {
			return err
		}
		task.SystemManaged = true
		if _, _, err := s.repo.CreateTask(ctx, task, schedule); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) normalizeInput(workspaceID string, input TaskInput) (Task, Schedule, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Task{}, Schedule{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	taskType := input.TaskType
	if taskType == "" {
		taskType = TaskTypeAgent
	}
	if taskType != TaskTypeAgent && taskType != TaskTypeSystem {
		return Task{}, Schedule{}, fmt.Errorf("%w: task_type is invalid", ErrInvalidInput)
	}
	state := input.State
	if state == "" {
		state = TaskDraft
	}
	if state != TaskDraft && state != TaskEnabled && state != TaskDisabled {
		return Task{}, Schedule{}, fmt.Errorf("%w: state is invalid", ErrInvalidInput)
	}
	timeoutMS := input.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = int(s.cfg.Tasks.DefaultTimeout / time.Millisecond)
	}
	retries := input.MaxRetries
	if retries < 0 {
		retries = s.cfg.Tasks.MaxRetries
	}
	policy := input.ToolPolicy
	if policy == "" {
		policy = "use_preapproved_tools_only"
	}
	if policy != "fail_if_approval_required" && policy != "use_preapproved_tools_only" {
		return Task{}, Schedule{}, fmt.Errorf("%w: tool_policy is invalid", ErrInvalidInput)
	}
	concurrency := input.ConcurrencyPolicy
	if concurrency == "" {
		concurrency = "skip"
	}
	if concurrency != "allow" && concurrency != "skip" && concurrency != "replace" {
		return Task{}, Schedule{}, fmt.Errorf("%w: concurrency_policy is invalid", ErrInvalidInput)
	}
	task := Task{
		WorkspaceID:       workspaceID,
		Name:              name,
		Description:       strings.TrimSpace(input.Description),
		TaskType:          taskType,
		State:             state,
		AgentID:           strings.TrimSpace(input.AgentID),
		ProviderID:        strings.TrimSpace(input.ProviderID),
		Model:             strings.TrimSpace(input.Model),
		Prompt:            strings.TrimSpace(input.Prompt),
		ToolPolicy:        policy,
		MaxRetries:        retries,
		TimeoutMS:         timeoutMS,
		ConcurrencyPolicy: concurrency,
	}
	schedule := Schedule{
		Mode:            input.ScheduleMode,
		CronExpression:  strings.TrimSpace(input.CronExpression),
		IntervalSeconds: input.IntervalSeconds,
		Timezone:        input.Timezone,
		Enabled:         state == TaskEnabled,
	}
	if schedule.Mode == "" {
		schedule.Mode = "manual"
	}
	if schedule.Timezone == "" {
		schedule.Timezone = s.cfg.Timezone
	}
	if _, err := time.LoadLocation(schedule.Timezone); err != nil {
		return Task{}, Schedule{}, fmt.Errorf("%w: timezone is invalid", ErrInvalidInput)
	}
	runAt, err := parseRunAt(input.RunAt)
	if err != nil {
		return Task{}, Schedule{}, err
	}
	schedule.RunAt = runAt
	if err := validateSchedule(schedule); err != nil {
		return Task{}, Schedule{}, err
	}
	schedule.NextRunAt = nextRun(schedule, time.Now().UTC(), s.cfg.Timezone)
	return task, schedule, nil
}

func (s *Service) enqueueTask(ctx context.Context, task Task, scheduleID string, key string, attempt int) (Run, error) {
	return s.repo.EnqueueRun(ctx, Run{
		TaskID:         task.ID,
		ScheduleID:     scheduleID,
		IdempotencyKey: key,
		State:          RunQueued,
		Attempt:        attempt,
		MaxRetries:     task.MaxRetries,
		TimeoutMS:      task.TimeoutMS,
	})
}

func (s *Service) execute(ctx context.Context, run Run, task Task) (string, error) {
	if task.TaskType == TaskTypeSystem {
		return s.executeSystemTask(ctx, task)
	}
	if task.Prompt == "" {
		return "", errors.New("agent task prompt is required")
	}
	if s.providers == nil || s.client == nil {
		return "", errors.New("provider execution is not configured")
	}
	return s.executeAgentTask(ctx, run, task)
}

func (s *Service) executeAgentTask(ctx context.Context, run Run, task Task) (string, error) {
	agent, err := s.resolveTaskAgent(ctx, task)
	if err != nil {
		return "", err
	}
	providerID := strings.TrimSpace(task.ProviderID)
	if providerID == "" {
		providerID = agent.DefaultProviderID
	}
	if providerID == "" {
		return "", errors.New("agent task requires provider_id or agent default provider")
	}
	provider, apiKey, err := s.providers.ResolveForChat(ctx, task.WorkspaceID, providerID)
	if err != nil {
		return "", err
	}
	model := strings.TrimSpace(task.Model)
	if model == "" {
		model = agent.DefaultModel
	}
	if model == "" {
		model = provider.DefaultModel
	}
	if model == "" {
		return "", errors.New("agent task requires model or provider default model")
	}
	memories, err := s.selectTaskMemories(ctx, task, agent)
	if err != nil {
		return "", err
	}
	runtimeTools, err := s.taskRuntimeTools(ctx, task, agent)
	if err != nil {
		return "", err
	}
	if len(memories) > 0 {
		_ = s.repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "info", Message: fmt.Sprintf("Task injected %d explicit memories.", len(memories))})
	}
	return s.executeTaskModelLoop(ctx, run, task, agent, provider, apiKey, model, memories, runtimeTools)
}

func (s *Service) resolveTaskAgent(ctx context.Context, task Task) (chat.AgentContext, error) {
	if strings.TrimSpace(task.AgentID) == "" || s.agents == nil {
		return chat.AgentContext{
			SystemPrompt:          "You are executing an unattended workspace task. Return a concise result.",
			MemoryAccessMode:      "pinned_only",
			ToolPermissionDefault: chat.ToolPermissionAsk,
			MaxToolIterations:     s.cfg.Chat.MaxToolIterations,
			Temperature:           0.7,
			Active:                true,
		}, nil
	}
	agent, err := s.agents.GetChatAgent(ctx, task.WorkspaceID, task.AgentID)
	if err != nil {
		return chat.AgentContext{}, err
	}
	if agent.SystemPrompt == "" {
		agent.SystemPrompt = "You are executing an unattended workspace task. Return a concise result."
	}
	if agent.MemoryAccessMode == "" {
		agent.MemoryAccessMode = "pinned_only"
	}
	if agent.ToolPermissionDefault == "" {
		agent.ToolPermissionDefault = chat.ToolPermissionAsk
	}
	if agent.MaxToolIterations <= 0 {
		agent.MaxToolIterations = s.cfg.Chat.MaxToolIterations
	}
	if agent.Temperature <= 0 {
		agent.Temperature = 0.7
	}
	return agent, nil
}

func (s *Service) selectTaskMemories(ctx context.Context, task Task, agent chat.AgentContext) ([]chat.MemorySnippet, error) {
	if s.memories == nil {
		return nil, nil
	}
	return s.memories.SelectForRun(ctx, chat.MemoryRequest{
		WorkspaceID: task.WorkspaceID,
		AgentID:     agent.ID,
		AccessMode:  agent.MemoryAccessMode,
		Query:       task.Prompt,
	})
}

func (s *Service) taskRuntimeTools(ctx context.Context, task Task, agent chat.AgentContext) ([]chat.RuntimeTool, error) {
	if s.tools == nil || agent.ID == "" {
		return nil, nil
	}
	tools, err := s.tools.RuntimeTools(ctx, chat.ToolExposureRequest{
		WorkspaceID:            task.WorkspaceID,
		AgentID:                agent.ID,
		AgentDefaultPermission: agent.ToolPermissionDefault,
	})
	if err != nil {
		return nil, err
	}
	if task.ToolPolicy == "use_preapproved_tools_only" {
		allowed := tools[:0]
		for _, tool := range tools {
			if tool.PermissionMode == chat.ToolPermissionAllow {
				allowed = append(allowed, tool)
			}
		}
		return allowed, nil
	}
	return tools, nil
}

func (s *Service) executeTaskModelLoop(
	ctx context.Context,
	run Run,
	task Task,
	agent chat.AgentContext,
	provider providers.Provider,
	apiKey string,
	model string,
	memories []chat.MemorySnippet,
	runtimeTools []chat.RuntimeTool,
) (string, error) {
	messages := taskPromptMessages(agent, memories, task.Prompt)
	toolByName := map[string]chat.RuntimeTool{}
	for _, tool := range runtimeTools {
		toolByName[tool.ProviderName] = tool
	}
	providerTools := taskProviderTools(runtimeTools)
	maxIterations := agent.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = s.cfg.Chat.MaxToolIterations
	}
	if maxIterations <= 0 {
		maxIterations = 8
	}
	var final strings.Builder
	for iteration := 0; ; iteration++ {
		events, err := s.client.StreamChat(ctx, providers.StreamRequest{
			Provider:    provider,
			APIKey:      apiKey,
			Model:       model,
			Messages:    messages,
			Tools:       providerTools,
			Temperature: agent.Temperature,
		})
		if err != nil {
			if agent.FallbackModel != "" && model != agent.FallbackModel {
				model = agent.FallbackModel
				_ = s.repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "warn", Message: "Retrying task with agent fallback model."})
				continue
			}
			return "", err
		}
		content, toolCalls, err := consumeTaskStream(events)
		if err != nil {
			return "", err
		}
		if len(toolCalls) == 0 {
			final.WriteString(content)
			break
		}
		if iteration >= maxIterations {
			return "", errors.New("task stopped because the tool iteration limit was reached")
		}
		messages = append(messages, providers.ChatMessage{Role: "assistant", Content: content, ToolCalls: toolCalls})
		for _, call := range toolCalls {
			tool, ok := toolByName[call.Function.Name]
			if !ok {
				return "", fmt.Errorf("task requested unavailable tool %q", call.Function.Name)
			}
			if tool.PermissionMode != chat.ToolPermissionAllow {
				return "", errors.New("task stopped because tool approval would be required")
			}
			_ = s.repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "info", Message: "Executing tool " + tool.Name + "."})
			result, err := s.tools.ExecuteRuntimeTool(ctx, chat.ToolExecutionRequest{
				WorkspaceID:     task.WorkspaceID,
				ToolID:          tool.ID,
				Arguments:       call.Function.Arguments,
				Timeout:         time.Duration(task.TimeoutMS) * time.Millisecond,
				MaxResultBytes:  32 * 1024,
				ProviderName:    tool.ProviderName,
				ToolDisplayName: tool.Name,
			})
			if err != nil {
				return "", err
			}
			if result.Truncated {
				_ = s.repo.AppendEvent(ctx, Event{RunID: run.ID, Level: "warn", Message: "Tool result was truncated."})
			}
			messages = append(messages, providers.ChatMessage{Role: "tool", ToolCallID: call.ID, Content: result.Content})
		}
	}
	text := strings.TrimSpace(final.String())
	if text == "" {
		return "", errors.New("provider returned an empty task result")
	}
	return text, nil
}

func taskPromptMessages(agent chat.AgentContext, memories []chat.MemorySnippet, prompt string) []providers.ChatMessage {
	messages := []providers.ChatMessage{
		{Role: "system", Content: strings.TrimSpace(agent.SystemPrompt)},
		{Role: "system", Content: "This is an unattended task. Do not wait for interactive approval; use only tools already available in this request."},
	}
	if len(memories) > 0 {
		var builder strings.Builder
		builder.WriteString("Explicit memories selected for this task:\n")
		for _, memory := range memories {
			builder.WriteString("- ")
			builder.WriteString(memory.Title)
			builder.WriteString(": ")
			builder.WriteString(memory.Content)
			builder.WriteString("\n")
		}
		messages = append(messages, providers.ChatMessage{Role: "system", Content: strings.TrimSpace(builder.String())})
	}
	messages = append(messages, providers.ChatMessage{Role: "user", Content: prompt})
	return messages
}

func taskProviderTools(tools []chat.RuntimeTool) []providers.ChatTool {
	out := make([]providers.ChatTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, providers.ChatTool{
			Type: "function",
			Function: providers.ChatToolFunction{
				Name:        tool.ProviderName,
				Description: tool.Description,
				Parameters:  json.RawMessage(validTaskSchema(tool.InputSchema)),
			},
		})
	}
	return out
}

func validTaskSchema(schema string) string {
	if strings.TrimSpace(schema) == "" || !json.Valid([]byte(schema)) {
		return "{}"
	}
	return schema
}

func consumeTaskStream(events <-chan providers.StreamEvent) (string, []providers.ToolCall, error) {
	var content strings.Builder
	var calls []providers.ToolCall
	for event := range events {
		if event.Error != nil {
			return "", nil, event.Error
		}
		switch event.Type {
		case "content_delta":
			content.WriteString(event.Content)
		case "tool_call_delta":
			if event.ToolCall != nil {
				calls = mergeTaskToolCall(calls, *event.ToolCall)
			}
		case "run_completed":
			return content.String(), calls, nil
		}
	}
	return content.String(), calls, nil
}

func mergeTaskToolCall(calls []providers.ToolCall, next providers.ToolCall) []providers.ToolCall {
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

func (s *Service) executeSystemTask(ctx context.Context, task Task) (string, error) {
	switch task.Name {
	case "cleanup_expired_sessions":
		count, err := s.repo.CleanupExpiredSessions(ctx, time.Now().UTC())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("expired sessions revoked=%d", count), nil
	case "recover_expired_task_leases":
		count, err := s.repo.RecoverExpiredLeases(ctx, time.Now().UTC())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("expired task leases recovered=%d", count), nil
	case "prune_old_task_run_events":
		cutoff := time.Now().UTC().Add(-s.cfg.Tasks.RunRetention)
		count, err := s.repo.PruneOldTaskRunEvents(ctx, cutoff)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("old task run events pruned=%d cutoff=%s", count, cutoff.Format(time.RFC3339)), nil
	case "update_conversation_summaries":
		if s.summaries == nil {
			return "", errors.New("conversation summary handler is not configured")
		}
		return s.summaries.UpdateConversationSummaries(ctx, 10)
	case "check_provider_health":
		handler, ok := s.providers.(ProviderHealthHandler)
		if !ok {
			return "", errors.New("provider health handler is not configured")
		}
		return handler.CheckProviderHealth(ctx, 25)
	case "check_mcp_server_health":
		if s.mcpHealth == nil {
			return "", errors.New("MCP health handler is not configured")
		}
		return s.mcpHealth.CheckMCPServerHealth(ctx, 25)
	case "cleanup_abandoned_chat_runs":
		if s.chatMaintenance == nil {
			return "", errors.New("chat maintenance handler is not configured")
		}
		return s.chatMaintenance.CleanupAbandonedChatRuns(ctx)
	case "recalculate_conversation_titles":
		if s.chatMaintenance == nil {
			return "", errors.New("chat maintenance handler is not configured")
		}
		return s.chatMaintenance.RecalculateConversationTitles(ctx, 25)
	case "compact_duplicate_task_scheduling_events":
		count, err := s.repo.CompactDuplicateSchedulingEvents(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("duplicate task scheduling events compacted=%d", count), nil
	case "cleanup_temporary_files":
		files, bytes, err := s.cleanupTemporaryFiles(ctx, time.Now().UTC().Add(-s.cfg.Tasks.RunRetention))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("temporary files deleted=%d bytes=%d", files, bytes), nil
	default:
		return "", fmt.Errorf("%w: unknown system task %s", ErrInvalidInput, task.Name)
	}
}

func (s *Service) cleanupTemporaryFiles(ctx context.Context, cutoff time.Time) (int, int64, error) {
	root := filepath.Clean(filepath.Join(s.cfg.DataDir, "tmp"))
	info, err := os.Lstat(root)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return 0, 0, fmt.Errorf("%w: temporary directory is unsafe", ErrInvalidInput)
	}
	deletedFiles := 0
	var deletedBytes int64
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == root {
			return nil
		}
		cleanPath := filepath.Clean(path)
		relative, err := filepath.Rel(root, cleanPath)
		if err != nil || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
			return fmt.Errorf("%w: temporary path escapes data directory", ErrInvalidInput)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() || info.ModTime().After(cutoff) {
			return nil
		}
		size := info.Size()
		if err := os.Remove(cleanPath); err != nil {
			return err
		}
		deletedFiles++
		deletedBytes += size
		return nil
	})
	return deletedFiles, deletedBytes, err
}

func nextRun(schedule Schedule, now time.Time, defaultTimezone string) *time.Time {
	if !schedule.Enabled {
		return nil
	}
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		location, _ = time.LoadLocation(defaultTimezone)
	}
	if location == nil {
		location = time.UTC
	}
	switch schedule.Mode {
	case "manual":
		return nil
	case "one_time":
		return schedule.RunAt
	case "interval":
		if schedule.IntervalSeconds <= 0 {
			return nil
		}
		next := now.Add(time.Duration(schedule.IntervalSeconds) * time.Second)
		return &next
	case "cron":
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		parsed, err := parser.Parse(schedule.CronExpression)
		if err != nil {
			return nil
		}
		localNext := parsed.Next(now.In(location))
		next := localNext.UTC()
		return &next
	default:
		return nil
	}
}

func parseRunAt(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("%w: run_at must be RFC3339", ErrInvalidInput)
	}
	utc := parsed.UTC()
	return &utc, nil
}

func validateSchedule(schedule Schedule) error {
	switch schedule.Mode {
	case "manual":
		return nil
	case "one_time":
		if schedule.RunAt == nil {
			return fmt.Errorf("%w: run_at is required for one_time schedules", ErrInvalidInput)
		}
	case "interval":
		if schedule.IntervalSeconds < 1 {
			return fmt.Errorf("%w: interval_seconds must be positive", ErrInvalidInput)
		}
	case "cron":
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		if _, err := parser.Parse(schedule.CronExpression); err != nil {
			return fmt.Errorf("%w: cron_expression is invalid", ErrInvalidInput)
		}
	default:
		return fmt.Errorf("%w: schedule_mode is invalid", ErrInvalidInput)
	}
	return nil
}

func systemTaskNames() []string {
	return []string{
		"cleanup_expired_sessions",
		"recover_expired_task_leases",
		"prune_old_task_run_events",
		"update_conversation_summaries",
		"check_provider_health",
		"check_mcp_server_health",
		"cleanup_abandoned_chat_runs",
		"recalculate_conversation_titles",
		"compact_duplicate_task_scheduling_events",
		"cleanup_temporary_files",
	}
}

func (s *Service) auditEvent(ctx context.Context, principal PrincipalContext, eventType string, taskID string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.InsertAuditEvent(ctx, auth.AuditEvent{
		WorkspaceID: principal.WorkspaceID,
		ActorUserID: principal.UserID,
		EventType:   eventType,
		IPAddress:   principal.IPAddress,
		UserAgent:   principal.UserAgent,
		Metadata:    map[string]any{"task_id": taskID},
	})
}
