package chat

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/id"
)

var ErrNotFound = errors.New("chat record not found")

type Repository interface {
	ListConversations(ctx context.Context, workspaceID string, ownerUserID string, search string) ([]Conversation, error)
	CreateConversation(ctx context.Context, conversation Conversation) (Conversation, error)
	GetConversation(ctx context.Context, workspaceID string, ownerUserID string, conversationID string) (Conversation, error)
	UpdateConversation(ctx context.Context, conversation Conversation) (Conversation, error)
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
	query := `SELECT id, workspace_id, owner_user_id, agent_id, provider_id, model, title, summary, summary_updated_at, archived_at, deleted_at, created_at, updated_at
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
	query := `INSERT INTO conversations (id, workspace_id, owner_user_id, provider_id, model, title, summary, created_at, updated_at)
VALUES (` + placeholders(r.store, 9) + `)`
	_, err := r.store.DB.ExecContext(ctx, query,
		conversation.ID, conversation.WorkspaceID, conversation.OwnerUserID, nullableString(conversation.ProviderID),
		nullableString(conversation.Model), conversation.Title, conversation.Summary, r.store.NowArg(now), r.store.NowArg(now),
	)
	return conversation, err
}

func (r *SQLRepository) GetConversation(ctx context.Context, workspaceID string, ownerUserID string, conversationID string) (Conversation, error) {
	query := `SELECT id, workspace_id, owner_user_id, agent_id, provider_id, model, title, summary, summary_updated_at, archived_at, deleted_at, created_at, updated_at
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
	query := `INSERT INTO messages (id, conversation_id, branch_id, parent_message_id, role, content, markdown, provider_id, model, created_at, updated_at)
VALUES (` + placeholders(r.store, 11) + `)`
	_, err := r.store.DB.ExecContext(ctx, query,
		message.ID, message.ConversationID, nullableString(message.BranchID), nullableString(message.ParentMessageID),
		message.Role, message.Content, message.Content, nullableString(message.ProviderID), nullableString(message.Model),
		r.store.NowArg(now), r.store.NowArg(now),
	)
	return message, err
}

func (r *SQLRepository) GetMessage(ctx context.Context, workspaceID string, ownerUserID string, messageID string) (Message, error) {
	query := `SELECT m.id, m.conversation_id, m.branch_id, m.parent_message_id, m.role, m.content, m.provider_id, m.model,
m.prompt_tokens, m.completion_tokens, m.total_tokens, m.created_at, m.updated_at
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
m.prompt_tokens, m.completion_tokens, m.total_tokens, m.created_at, m.updated_at
FROM messages m JOIN conversations c ON c.id = m.conversation_id`
}

func scanConversation(row rowScanner) (Conversation, error) {
	var item Conversation
	var agentID sql.NullString
	var providerID sql.NullString
	var model sql.NullString
	var summaryUpdatedRaw any
	var archivedRaw any
	var deletedRaw any
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&item.ID, &item.WorkspaceID, &item.OwnerUserID, &agentID, &providerID, &model, &item.Title, &item.Summary,
		&summaryUpdatedRaw, &archivedRaw, &deletedRaw, &createdRaw, &updatedRaw); err != nil {
		return Conversation{}, err
	}
	item.AgentID = agentID.String
	item.ProviderID = providerID.String
	item.Model = model.String
	var err error
	item.SummaryUpdatedAt, err = nullableTime(summaryUpdatedRaw)
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
	var prompt sql.NullInt64
	var completion sql.NullInt64
	var total sql.NullInt64
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&item.ID, &item.ConversationID, &branchID, &parentID, &item.Role, &item.Content, &providerID, &model,
		&prompt, &completion, &total, &createdRaw, &updatedRaw); err != nil {
		return Message{}, err
	}
	item.BranchID = branchID.String
	item.ParentMessageID = parentID.String
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
