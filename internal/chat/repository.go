package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/isyuricunha/nostos/internal/database"
	"github.com/isyuricunha/nostos/internal/id"
	"github.com/isyuricunha/nostos/internal/providers"
)

var ErrNotFound = errors.New("chat record not found")

type Repository interface {
	ListConversations(ctx context.Context, workspaceID string, ownerUserID string, search string) ([]Conversation, error)
	CreateConversation(ctx context.Context, conversation Conversation) (Conversation, error)
	GetConversation(ctx context.Context, workspaceID string, ownerUserID string, conversationID string) (Conversation, error)
	UpdateConversation(ctx context.Context, conversation Conversation) (Conversation, error)
	MarkSummaryQueued(ctx context.Context, workspaceID string, ownerUserID string, conversationID string) (Conversation, bool, error)
	MarkSummaryRunning(ctx context.Context, conversationID string) error
	SaveConversationSummary(ctx context.Context, conversationID string, update SummaryUpdate) error
	MarkSummaryFailed(ctx context.Context, conversationID string, message string) error
	SummaryCandidates(ctx context.Context, limit int) ([]Conversation, error)
	DeleteConversation(ctx context.Context, workspaceID string, ownerUserID string, conversationID string, now time.Time) error
	CreateMessage(ctx context.Context, message Message) (Message, error)
	GetMessage(ctx context.Context, workspaceID string, ownerUserID string, messageID string) (Message, error)
	ListMessages(ctx context.Context, workspaceID string, ownerUserID string, conversationID string) ([]Message, error)
	RecentMessages(ctx context.Context, workspaceID string, ownerUserID string, conversationID string, limit int) ([]Message, error)
	UpdateMessageContent(ctx context.Context, messageID string, content string, usage UsageValues) error
	CreateBranch(ctx context.Context, branch Branch) (Branch, error)
	CreateRun(ctx context.Context, run ChatRun) (ChatRun, error)
	UpdateRunState(ctx context.Context, runID string, state string, errorCode string, errorMessage string, completed bool) error
	UpdateRunUsage(ctx context.Context, runID string, usage UsageValues) error
	RequestCancellation(ctx context.Context, workspaceID string, ownerUserID string, runID string, now time.Time) error
	CancellationRequested(ctx context.Context, runID string) (bool, error)
	CleanupInterruptedRuns(ctx context.Context, now time.Time) (int64, error)
	RecalculateConversationTitles(ctx context.Context, limit int) (int64, error)
	FindRunByAssistantMessage(ctx context.Context, workspaceID string, ownerUserID string, messageID string) (ChatRun, error)
}

type UsageValues struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type SQLRepository struct {
	store *database.Store
}

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) ListConversations(ctx context.Context, workspaceID string, ownerUserID string, search string) ([]Conversation, error) {
	args := []any{workspaceID, ownerUserID}
	query := conversationSelect() + `
FROM conversations WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND owner_user_id = ` + r.store.Placeholder(2) + ` AND deleted_at IS NULL`
	if strings.TrimSpace(search) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(search))+"%")
		query += ` AND lower(title) LIKE ` + r.store.Placeholder(3)
	}
	query += ` ORDER BY updated_at DESC LIMIT 100`
	rows, err := r.store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var conversations []Conversation
	for rows.Next() {
		item, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, item)
	}
	return conversations, rows.Err()
}

func (r *SQLRepository) CreateConversation(ctx context.Context, conversation Conversation) (Conversation, error) {
	now := time.Now().UTC()
	conversation.ID = id.New()
	conversation.CreatedAt = now
	conversation.UpdatedAt = now
	if strings.TrimSpace(conversation.Title) == "" {
		conversation.Title = "New conversation"
	}
	query := `INSERT INTO conversations (id, workspace_id, owner_user_id, agent_id, provider_id, model, title, summary, created_at, updated_at)
VALUES (` + placeholders(r.store, 10) + `)`
	_, err := r.store.DB.ExecContext(ctx, query,
		conversation.ID, conversation.WorkspaceID, conversation.OwnerUserID, nullableString(conversation.AgentID), nullableString(conversation.ProviderID),
		nullableString(conversation.Model), conversation.Title, conversation.Summary, r.store.NowArg(now), r.store.NowArg(now),
	)
	return conversation, err
}

func (r *SQLRepository) GetConversation(ctx context.Context, workspaceID string, ownerUserID string, conversationID string) (Conversation, error) {
	query := conversationSelect() + `
FROM conversations WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND owner_user_id = ` + r.store.Placeholder(2) + ` AND id = ` + r.store.Placeholder(3) + ` AND deleted_at IS NULL`
	item, err := scanConversation(r.store.DB.QueryRowContext(ctx, query, workspaceID, ownerUserID, conversationID))
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, ErrNotFound
	}
	return item, err
}

func (r *SQLRepository) UpdateConversation(ctx context.Context, conversation Conversation) (Conversation, error) {
	now := time.Now().UTC()
	conversation.UpdatedAt = now
	query := `UPDATE conversations SET title = ` + r.store.Placeholder(1) + `, archived_at = ` + r.store.Placeholder(2) +
		`, summary = ` + r.store.Placeholder(3) + `, summary_updated_at = ` + r.store.Placeholder(4) +
		`, updated_at = ` + r.store.Placeholder(5) + ` WHERE workspace_id = ` + r.store.Placeholder(6) +
		` AND owner_user_id = ` + r.store.Placeholder(7) + ` AND id = ` + r.store.Placeholder(8)
	result, err := r.store.DB.ExecContext(ctx, query, conversation.Title, timePtrArg(r.store, conversation.ArchivedAt), conversation.Summary,
		timePtrArg(r.store, conversation.SummaryUpdatedAt), r.store.NowArg(now), conversation.WorkspaceID, conversation.OwnerUserID, conversation.ID)
	if err != nil {
		return Conversation{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Conversation{}, err
	}
	if affected == 0 {
		return Conversation{}, ErrNotFound
	}
	return conversation, nil
}

func (r *SQLRepository) MarkSummaryQueued(ctx context.Context, workspaceID string, ownerUserID string, conversationID string) (Conversation, bool, error) {
	now := time.Now().UTC()
	query := `UPDATE conversations SET summary_status = ` + r.store.Placeholder(1) + `, summary_error = NULL, updated_at = ` + r.store.Placeholder(2) +
		` WHERE workspace_id = ` + r.store.Placeholder(3) + ` AND owner_user_id = ` + r.store.Placeholder(4) + ` AND id = ` + r.store.Placeholder(5) +
		` AND summary_status NOT IN (` + r.store.Placeholder(6) + `, ` + r.store.Placeholder(7) + `)`
	result, err := r.store.DB.ExecContext(ctx, query, "queued", r.store.NowArg(now), workspaceID, ownerUserID, conversationID, "queued", "running")
	if err != nil {
		return Conversation{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Conversation{}, false, err
	}
	conversation, getErr := r.GetConversation(ctx, workspaceID, ownerUserID, conversationID)
	if getErr != nil {
		return Conversation{}, false, getErr
	}
	return conversation, affected > 0, nil
}

func (r *SQLRepository) MarkSummaryRunning(ctx context.Context, conversationID string) error {
	now := time.Now().UTC()
	query := `UPDATE conversations SET summary_status = ` + r.store.Placeholder(1) + `, summary_error = NULL, updated_at = ` + r.store.Placeholder(2) +
		` WHERE id = ` + r.store.Placeholder(3) + ` AND summary_status = ` + r.store.Placeholder(4)
	_, err := r.store.DB.ExecContext(ctx, query, "running", r.store.NowArg(now), conversationID, "queued")
	return err
}

func (r *SQLRepository) SaveConversationSummary(ctx context.Context, conversationID string, update SummaryUpdate) error {
	now := time.Now().UTC()
	generatedAt := update.GeneratedAt
	if generatedAt == nil {
		generatedAt = &now
	}
	versionExpr := "summary_version"
	if update.IncrementVersion {
		versionExpr = "summary_version + 1"
	}
	query := `UPDATE conversations SET summary = ` + r.store.Placeholder(1) + `, summary_updated_at = ` + r.store.Placeholder(2) +
		`, summary_status = ` + r.store.Placeholder(3) + `, summary_error = ` + r.store.Placeholder(4) +
		`, summary_source_start_message_id = ` + r.store.Placeholder(5) + `, summary_source_end_message_id = ` + r.store.Placeholder(6) +
		`, summary_provider_id = ` + r.store.Placeholder(7) + `, summary_model = ` + r.store.Placeholder(8) +
		`, summary_generated_at = ` + r.store.Placeholder(9) + `, summary_estimated_input_tokens = ` + r.store.Placeholder(10) +
		`, summary_version = ` + versionExpr + `, updated_at = ` + r.store.Placeholder(11) + ` WHERE id = ` + r.store.Placeholder(12)
	_, err := r.store.DB.ExecContext(ctx, query,
		update.Summary,
		r.store.NowArg(now),
		update.Status,
		nullableString(update.Error),
		nullableString(update.SourceStartMessageID),
		nullableString(update.SourceEndMessageID),
		nullableString(update.ProviderID),
		nullableString(update.Model),
		timePtrArg(r.store, generatedAt),
		nullInt(update.EstimatedInputTokens),
		r.store.NowArg(now),
		conversationID,
	)
	return err
}

func (r *SQLRepository) MarkSummaryFailed(ctx context.Context, conversationID string, message string) error {
	now := time.Now().UTC()
	query := `UPDATE conversations SET summary_status = ` + r.store.Placeholder(1) + `, summary_error = ` + r.store.Placeholder(2) +
		`, updated_at = ` + r.store.Placeholder(3) + ` WHERE id = ` + r.store.Placeholder(4)
	_, err := r.store.DB.ExecContext(ctx, query, "failed", nullableString(message), r.store.NowArg(now), conversationID)
	return err
}

func (r *SQLRepository) SummaryCandidates(ctx context.Context, limit int) ([]Conversation, error) {
	if limit <= 0 {
		limit = 10
	}
	query := conversationSelect() + ` FROM conversations WHERE deleted_at IS NULL AND summary_status = ` + r.store.Placeholder(1) +
		` ORDER BY updated_at ASC LIMIT ` + r.store.Placeholder(2)
	rows, err := r.store.DB.QueryContext(ctx, query, "queued", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var conversations []Conversation
	for rows.Next() {
		item, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, item)
	}
	return conversations, rows.Err()
}

func (r *SQLRepository) DeleteConversation(ctx context.Context, workspaceID string, ownerUserID string, conversationID string, now time.Time) error {
	query := `UPDATE conversations SET deleted_at = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
		` WHERE workspace_id = ` + r.store.Placeholder(3) + ` AND owner_user_id = ` + r.store.Placeholder(4) + ` AND id = ` + r.store.Placeholder(5)
	result, err := r.store.DB.ExecContext(ctx, query, r.store.NowArg(now), r.store.NowArg(now), workspaceID, ownerUserID, conversationID)
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

func (r *SQLRepository) CreateMessage(ctx context.Context, message Message) (Message, error) {
	now := time.Now().UTC()
	message.ID = id.New()
	message.CreatedAt = now
	message.UpdatedAt = now
	query := `INSERT INTO messages (id, conversation_id, branch_id, parent_message_id, role, content, markdown, tool_call_id, provider_id, model, metadata, created_at, updated_at)
VALUES (` + placeholders(r.store, 13) + `)`
	_, err := r.store.DB.ExecContext(ctx, query,
		message.ID, message.ConversationID, nullableString(message.BranchID), nullableString(message.ParentMessageID),
		message.Role, message.Content, message.Content, nullableString(message.ToolCallID), nullableString(message.ProviderID), nullableString(message.Model), messageMetadata(message),
		r.store.NowArg(now), r.store.NowArg(now),
	)
	return message, err
}

func (r *SQLRepository) GetMessage(ctx context.Context, workspaceID string, ownerUserID string, messageID string) (Message, error) {
	query := `SELECT m.id, m.conversation_id, m.branch_id, m.parent_message_id, m.role, m.content, m.provider_id, m.model,
m.tool_call_id, m.metadata, m.prompt_tokens, m.completion_tokens, m.total_tokens, m.created_at, m.updated_at
FROM messages m JOIN conversations c ON c.id = m.conversation_id
WHERE c.workspace_id = ` + r.store.Placeholder(1) + ` AND c.owner_user_id = ` + r.store.Placeholder(2) + ` AND m.id = ` + r.store.Placeholder(3)
	item, err := scanMessage(r.store.DB.QueryRowContext(ctx, query, workspaceID, ownerUserID, messageID))
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, ErrNotFound
	}
	return item, err
}

func (r *SQLRepository) ListMessages(ctx context.Context, workspaceID string, ownerUserID string, conversationID string) ([]Message, error) {
	query := messageSelect(r.store) + ` WHERE c.workspace_id = ` + r.store.Placeholder(1) + ` AND c.owner_user_id = ` + r.store.Placeholder(2) +
		` AND m.conversation_id = ` + r.store.Placeholder(3) + ` ORDER BY m.created_at`
	return r.queryMessages(ctx, query, workspaceID, ownerUserID, conversationID)
}

func (r *SQLRepository) RecentMessages(ctx context.Context, workspaceID string, ownerUserID string, conversationID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = DefaultConversationTTL
	}
	query := messageSelect(r.store) + ` WHERE c.workspace_id = ` + r.store.Placeholder(1) + ` AND c.owner_user_id = ` + r.store.Placeholder(2) +
		` AND m.conversation_id = ` + r.store.Placeholder(3) + ` ORDER BY m.created_at DESC LIMIT ` + r.store.Placeholder(4)
	messages, err := r.queryMessages(ctx, query, workspaceID, ownerUserID, conversationID, limit)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

func (r *SQLRepository) UpdateMessageContent(ctx context.Context, messageID string, content string, usage UsageValues) error {
	now := time.Now().UTC()
	query := `UPDATE messages SET content = ` + r.store.Placeholder(1) + `, markdown = ` + r.store.Placeholder(2) +
		`, prompt_tokens = ` + r.store.Placeholder(3) + `, completion_tokens = ` + r.store.Placeholder(4) +
		`, total_tokens = ` + r.store.Placeholder(5) + `, updated_at = ` + r.store.Placeholder(6) + ` WHERE id = ` + r.store.Placeholder(7)
	_, err := r.store.DB.ExecContext(ctx, query, content, content, nullInt(usage.PromptTokens), nullInt(usage.CompletionTokens), nullInt(usage.TotalTokens), r.store.NowArg(now), messageID)
	return err
}

func (r *SQLRepository) CreateBranch(ctx context.Context, branch Branch) (Branch, error) {
	now := time.Now().UTC()
	branch.ID = id.New()
	branch.CreatedAt = now
	branch.Active = true
	query := `INSERT INTO message_branches (id, conversation_id, parent_message_id, source_message_id, name, active, created_at)
VALUES (` + placeholders(r.store, 7) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, branch.ID, branch.ConversationID, nullableString(branch.ParentMessageID), nullableString(branch.SourceMessageID), branch.Name, branch.Active, r.store.NowArg(now))
	return branch, err
}

func (r *SQLRepository) CreateRun(ctx context.Context, run ChatRun) (ChatRun, error) {
	now := time.Now().UTC()
	run.ID = id.New()
	run.State = RunStreaming
	run.CreatedAt = now
	run.UpdatedAt = now
	run.StartedAt = &now
	query := `INSERT INTO chat_runs (id, conversation_id, user_message_id, assistant_message_id, branch_id, provider_id, model, state, started_at, created_at, updated_at)
VALUES (` + placeholders(r.store, 11) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, run.ID, run.ConversationID, nullableString(run.UserMessageID), nullableString(run.AssistantMessageID), nullableString(run.BranchID),
		nullableString(run.ProviderID), nullableString(run.Model), run.State, r.store.NowArg(now), r.store.NowArg(now), r.store.NowArg(now))
	return run, err
}

func (r *SQLRepository) UpdateRunState(ctx context.Context, runID string, state string, errorCode string, errorMessage string, completed bool) error {
	now := time.Now().UTC()
	completedAt := any(nil)
	if completed {
		completedAt = r.store.NowArg(now)
	}
	query := `UPDATE chat_runs SET state = ` + r.store.Placeholder(1) + `, error_code = ` + r.store.Placeholder(2) +
		`, error_message = ` + r.store.Placeholder(3) + `, completed_at = ` + r.store.Placeholder(4) +
		`, updated_at = ` + r.store.Placeholder(5) + ` WHERE id = ` + r.store.Placeholder(6)
	_, err := r.store.DB.ExecContext(ctx, query, state, nullableString(errorCode), nullableString(errorMessage), completedAt, r.store.NowArg(now), runID)
	return err
}

func (r *SQLRepository) UpdateRunUsage(ctx context.Context, runID string, usage UsageValues) error {
	now := time.Now().UTC()
	query := `UPDATE chat_runs SET prompt_tokens = ` + r.store.Placeholder(1) + `, completion_tokens = ` + r.store.Placeholder(2) +
		`, total_tokens = ` + r.store.Placeholder(3) + `, updated_at = ` + r.store.Placeholder(4) + ` WHERE id = ` + r.store.Placeholder(5)
	_, err := r.store.DB.ExecContext(ctx, query, nullInt(usage.PromptTokens), nullInt(usage.CompletionTokens), nullInt(usage.TotalTokens), r.store.NowArg(now), runID)
	return err
}

func (r *SQLRepository) RequestCancellation(ctx context.Context, workspaceID string, ownerUserID string, runID string, now time.Time) error {
	query := `UPDATE chat_runs SET cancellation_requested_at = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
		` FROM conversations WHERE chat_runs.conversation_id = conversations.id AND conversations.workspace_id = ` + r.store.Placeholder(3) +
		` AND conversations.owner_user_id = ` + r.store.Placeholder(4) + ` AND chat_runs.id = ` + r.store.Placeholder(5)
	if r.store.Dialect == database.SQLite {
		query = `UPDATE chat_runs SET cancellation_requested_at = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
			` WHERE conversation_id IN (SELECT id FROM conversations WHERE workspace_id = ` + r.store.Placeholder(3) +
			` AND owner_user_id = ` + r.store.Placeholder(4) + `) AND id = ` + r.store.Placeholder(5)
	}
	result, err := r.store.DB.ExecContext(ctx, query, r.store.NowArg(now), r.store.NowArg(now), workspaceID, ownerUserID, runID)
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

func (r *SQLRepository) CancellationRequested(ctx context.Context, runID string) (bool, error) {
	var raw any
	query := `SELECT cancellation_requested_at FROM chat_runs WHERE id = ` + r.store.Placeholder(1)
	err := r.store.DB.QueryRowContext(ctx, query, runID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return raw != nil, nil
}

func (r *SQLRepository) CleanupInterruptedRuns(ctx context.Context, now time.Time) (int64, error) {
	query := `UPDATE chat_runs SET state = ` + r.store.Placeholder(1) + `, error_code = ` + r.store.Placeholder(2) +
		`, error_message = ` + r.store.Placeholder(3) + `, completed_at = ` + r.store.Placeholder(4) +
		`, updated_at = ` + r.store.Placeholder(5) + ` WHERE state IN (` + r.store.Placeholder(6) + `, ` + r.store.Placeholder(7) + `)`
	result, err := r.store.DB.ExecContext(ctx, query, RunFailed, "interrupted", "Generation was interrupted and can be regenerated.", r.store.NowArg(now), r.store.NowArg(now), RunPending, RunStreaming)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *SQLRepository) RecalculateConversationTitles(ctx context.Context, limit int) (int64, error) {
	if limit <= 0 {
		limit = 25
	}
	query := `SELECT id FROM conversations WHERE deleted_at IS NULL AND title = ` + r.store.Placeholder(1) +
		` ORDER BY created_at ASC LIMIT ` + r.store.Placeholder(2)
	rows, err := r.store.DB.QueryContext(ctx, query, "New conversation", limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var conversationIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		conversationIDs = append(conversationIDs, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	var updated int64
	for _, conversationID := range conversationIDs {
		var content string
		titleQuery := `SELECT content FROM messages WHERE conversation_id = ` + r.store.Placeholder(1) +
			` AND role = ` + r.store.Placeholder(2) + ` ORDER BY created_at ASC LIMIT 1`
		if err := r.store.DB.QueryRowContext(ctx, titleQuery, conversationID, RoleUser).Scan(&content); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return updated, err
		}
		title := generatedTitle(content)
		if title == "" {
			continue
		}
		updateQuery := `UPDATE conversations SET title = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
			` WHERE id = ` + r.store.Placeholder(3) + ` AND title = ` + r.store.Placeholder(4)
		result, err := r.store.DB.ExecContext(ctx, updateQuery, title, r.store.NowArg(time.Now().UTC()), conversationID, "New conversation")
		if err != nil {
			return updated, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return updated, err
		}
		updated += affected
	}
	return updated, nil
}

func (r *SQLRepository) FindRunByAssistantMessage(ctx context.Context, workspaceID string, ownerUserID string, messageID string) (ChatRun, error) {
	query := `SELECT cr.id, cr.conversation_id, cr.user_message_id, cr.assistant_message_id, cr.branch_id, cr.provider_id, cr.model, cr.state,
cr.error_code, cr.error_message, cr.cancellation_requested_at, cr.started_at, cr.completed_at, cr.prompt_tokens, cr.completion_tokens,
cr.total_tokens, cr.created_at, cr.updated_at
FROM chat_runs cr JOIN conversations c ON c.id = cr.conversation_id
WHERE c.workspace_id = ` + r.store.Placeholder(1) + ` AND c.owner_user_id = ` + r.store.Placeholder(2) + ` AND cr.assistant_message_id = ` + r.store.Placeholder(3) +
		` ORDER BY cr.created_at DESC LIMIT 1`
	run, err := scanRun(r.store.DB.QueryRowContext(ctx, query, workspaceID, ownerUserID, messageID))
	if errors.Is(err, sql.ErrNoRows) {
		return ChatRun{}, ErrNotFound
	}
	return run, err
}

func (r *SQLRepository) queryMessages(ctx context.Context, query string, args ...any) ([]Message, error) {
	rows, err := r.store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func messageSelect(store *database.Store) string {
	return `SELECT m.id, m.conversation_id, m.branch_id, m.parent_message_id, m.role, m.content, m.provider_id, m.model,
m.tool_call_id, m.metadata, m.prompt_tokens, m.completion_tokens, m.total_tokens, m.created_at, m.updated_at
FROM messages m JOIN conversations c ON c.id = m.conversation_id`
}

func conversationSelect() string {
	return `SELECT id, workspace_id, owner_user_id, agent_id, provider_id, model, title, summary, summary_updated_at,
summary_status, summary_error, summary_source_start_message_id, summary_source_end_message_id, summary_provider_id,
summary_model, summary_generated_at, summary_estimated_input_tokens, summary_version, archived_at, deleted_at, created_at, updated_at`
}

func scanConversation(row rowScanner) (Conversation, error) {
	var item Conversation
	var agentID sql.NullString
	var providerID sql.NullString
	var model sql.NullString
	var summaryUpdatedRaw any
	var summaryStatus sql.NullString
	var summaryError sql.NullString
	var summarySourceStart sql.NullString
	var summarySourceEnd sql.NullString
	var summaryProviderID sql.NullString
	var summaryModel sql.NullString
	var summaryGeneratedRaw any
	var summaryEstimated sql.NullInt64
	var summaryVersion sql.NullInt64
	var archivedRaw any
	var deletedRaw any
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&item.ID, &item.WorkspaceID, &item.OwnerUserID, &agentID, &providerID, &model, &item.Title, &item.Summary,
		&summaryUpdatedRaw, &summaryStatus, &summaryError, &summarySourceStart, &summarySourceEnd, &summaryProviderID, &summaryModel,
		&summaryGeneratedRaw, &summaryEstimated, &summaryVersion, &archivedRaw, &deletedRaw, &createdRaw, &updatedRaw); err != nil {
		return Conversation{}, err
	}
	item.AgentID = agentID.String
	item.ProviderID = providerID.String
	item.Model = model.String
	item.SummaryStatus = summaryStatus.String
	if item.SummaryStatus == "" {
		item.SummaryStatus = "idle"
	}
	item.SummaryError = summaryError.String
	item.SummarySourceStartMessageID = summarySourceStart.String
	item.SummarySourceEndMessageID = summarySourceEnd.String
	item.SummaryProviderID = summaryProviderID.String
	item.SummaryModel = summaryModel.String
	item.SummaryEstimatedInputTokens = int(summaryEstimated.Int64)
	item.SummaryVersion = int(summaryVersion.Int64)
	var err error
	item.SummaryUpdatedAt, err = nullableTime(summaryUpdatedRaw)
	if err != nil {
		return Conversation{}, err
	}
	item.SummaryGeneratedAt, err = nullableTime(summaryGeneratedRaw)
	if err != nil {
		return Conversation{}, err
	}
	item.ArchivedAt, err = nullableTime(archivedRaw)
	if err != nil {
		return Conversation{}, err
	}
	item.DeletedAt, err = nullableTime(deletedRaw)
	if err != nil {
		return Conversation{}, err
	}
	item.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Conversation{}, err
	}
	item.UpdatedAt, err = database.ParseTime(updatedRaw)
	if err != nil {
		return Conversation{}, err
	}
	return item, nil
}

func scanMessage(row rowScanner) (Message, error) {
	var item Message
	var branchID sql.NullString
	var parentID sql.NullString
	var providerID sql.NullString
	var model sql.NullString
	var toolCallID sql.NullString
	var metadataRaw any
	var prompt sql.NullInt64
	var completion sql.NullInt64
	var total sql.NullInt64
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&item.ID, &item.ConversationID, &branchID, &parentID, &item.Role, &item.Content, &providerID, &model,
		&toolCallID, &metadataRaw, &prompt, &completion, &total, &createdRaw, &updatedRaw); err != nil {
		return Message{}, err
	}
	item.BranchID = branchID.String
	item.ParentMessageID = parentID.String
	item.ToolCallID = toolCallID.String
	item.ToolCalls = toolCallsFromMetadata(metadataRaw)
	item.ProviderID = providerID.String
	item.Model = model.String
	item.PromptTokens = int(prompt.Int64)
	item.CompletionTokens = int(completion.Int64)
	item.TotalTokens = int(total.Int64)
	var err error
	item.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Message{}, err
	}
	item.UpdatedAt, err = database.ParseTime(updatedRaw)
	if err != nil {
		return Message{}, err
	}
	return item, nil
}

func scanRun(row rowScanner) (ChatRun, error) {
	var run ChatRun
	var userID sql.NullString
	var assistantID sql.NullString
	var branchID sql.NullString
	var providerID sql.NullString
	var model sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var cancelRaw any
	var startedRaw any
	var completedRaw any
	var prompt sql.NullInt64
	var completion sql.NullInt64
	var total sql.NullInt64
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&run.ID, &run.ConversationID, &userID, &assistantID, &branchID, &providerID, &model, &run.State, &errorCode, &errorMessage,
		&cancelRaw, &startedRaw, &completedRaw, &prompt, &completion, &total, &createdRaw, &updatedRaw); err != nil {
		return ChatRun{}, err
	}
	run.UserMessageID = userID.String
	run.AssistantMessageID = assistantID.String
	run.BranchID = branchID.String
	run.ProviderID = providerID.String
	run.Model = model.String
	run.ErrorCode = errorCode.String
	run.ErrorMessage = errorMessage.String
	var err error
	run.CancellationRequested, err = nullableTime(cancelRaw)
	if err != nil {
		return ChatRun{}, err
	}
	run.StartedAt, err = nullableTime(startedRaw)
	if err != nil {
		return ChatRun{}, err
	}
	run.CompletedAt, err = nullableTime(completedRaw)
	if err != nil {
		return ChatRun{}, err
	}
	run.PromptTokens = int(prompt.Int64)
	run.CompletionTokens = int(completion.Int64)
	run.TotalTokens = int(total.Int64)
	run.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return ChatRun{}, err
	}
	run.UpdatedAt, err = database.ParseTime(updatedRaw)
	if err != nil {
		return ChatRun{}, err
	}
	return run, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func nullableTime(value any) (*time.Time, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		if strings.TrimSpace(string(typed)) == "" {
			return nil, nil
		}
	case string:
		if strings.TrimSpace(typed) == "" {
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

func nullInt(value int) any {
	if value <= 0 {
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

func messageMetadata(message Message) string {
	payload := map[string]any{}
	if len(message.ToolCalls) > 0 {
		payload["tool_calls"] = message.ToolCalls
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func toolCallsFromMetadata(raw any) []providers.ToolCall {
	var text string
	switch typed := raw.(type) {
	case nil:
		return nil
	case string:
		text = typed
	case []byte:
		text = string(typed)
	default:
		return nil
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var payload struct {
		ToolCalls []providers.ToolCall `json:"tool_calls"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil
	}
	return payload.ToolCalls
}

func generatedTitle(content string) string {
	content = strings.Join(strings.Fields(content), " ")
	if content == "" {
		return ""
	}
	if len(content) > 60 {
		content = strings.TrimSpace(content[:60])
	}
	return content
}
