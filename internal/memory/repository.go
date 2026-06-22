package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/yuricunha/nostos/internal/chat"
	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/id"
)

var ErrNotFound = errors.New("memory not found")

type Repository interface {
	List(ctx context.Context, workspaceID string, query string) ([]Memory, error)
	Get(ctx context.Context, workspaceID string, memoryID string) (Memory, error)
	Create(ctx context.Context, memory Memory) (Memory, error)
	Update(ctx context.Context, memory Memory) (Memory, error)
	Delete(ctx context.Context, workspaceID string, memoryID string) error
	Candidates(ctx context.Context, request chat.MemoryRequest) ([]Memory, error)
	RecordRun(ctx context.Context, runID string, memories []chat.MemorySnippet) error
	UsedByRun(ctx context.Context, runID string) ([]chat.MemorySnippet, error)
	RemoveFromRun(ctx context.Context, runID string, memoryID string, now time.Time) error
}

type SQLRepository struct {
	store *database.Store
}

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) List(ctx context.Context, workspaceID string, queryText string) ([]Memory, error) {
	args := []any{workspaceID}
	query := memorySelect(r.store) + ` WHERE workspace_id = ` + r.store.Placeholder(1)
	if strings.TrimSpace(queryText) != "" {
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(queryText))+"%")
		query += ` AND (lower(title) LIKE ` + r.store.Placeholder(2) + ` OR lower(content) LIKE ` + r.store.Placeholder(2) + `)`
	}
	query += ` ORDER BY pinned DESC, importance DESC, updated_at DESC`
	rows, err := r.store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

func (r *SQLRepository) Get(ctx context.Context, workspaceID string, memoryID string) (Memory, error) {
	query := memorySelect(r.store) + ` WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	item, err := scanMemory(r.store.DB.QueryRowContext(ctx, query, workspaceID, memoryID))
	if errors.Is(err, sql.ErrNoRows) {
		return Memory{}, ErrNotFound
	}
	return item, err
}

func (r *SQLRepository) Create(ctx context.Context, memory Memory) (Memory, error) {
	now := time.Now().UTC()
	memory.ID = id.New()
	memory.CreatedAt = now
	memory.UpdatedAt = now
	query := `INSERT INTO memories (id, workspace_id, owner_user_id, agent_id, conversation_id, title, content, tags, scope, importance,
pinned, active, source, source_message_id, created_at, updated_at) VALUES (` + placeholders(r.store, 16) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, memory.ID, memory.WorkspaceID, memory.OwnerUserID, nullableString(memory.AgentID),
		nullableString(memory.ConversationID), memory.Title, memory.Content, jsonArray(memory.Tags), memory.Scope, memory.Importance,
		memory.Pinned, memory.Active, memory.Source, nullableString(memory.SourceMessageID), r.store.NowArg(now), r.store.NowArg(now))
	return memory, err
}

func (r *SQLRepository) Update(ctx context.Context, memory Memory) (Memory, error) {
	now := time.Now().UTC()
	memory.UpdatedAt = now
	query := `UPDATE memories SET agent_id = ` + r.store.Placeholder(1) + `, conversation_id = ` + r.store.Placeholder(2) +
		`, title = ` + r.store.Placeholder(3) + `, content = ` + r.store.Placeholder(4) + `, tags = ` + r.store.Placeholder(5) +
		`, scope = ` + r.store.Placeholder(6) + `, importance = ` + r.store.Placeholder(7) + `, pinned = ` + r.store.Placeholder(8) +
		`, active = ` + r.store.Placeholder(9) + `, source = ` + r.store.Placeholder(10) + `, source_message_id = ` + r.store.Placeholder(11) +
		`, updated_at = ` + r.store.Placeholder(12) + ` WHERE workspace_id = ` + r.store.Placeholder(13) + ` AND id = ` + r.store.Placeholder(14)
	result, err := r.store.DB.ExecContext(ctx, query, nullableString(memory.AgentID), nullableString(memory.ConversationID), memory.Title,
		memory.Content, jsonArray(memory.Tags), memory.Scope, memory.Importance, memory.Pinned, memory.Active, memory.Source,
		nullableString(memory.SourceMessageID), r.store.NowArg(now), memory.WorkspaceID, memory.ID)
	if err != nil {
		return Memory{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Memory{}, err
	}
	if affected == 0 {
		return Memory{}, ErrNotFound
	}
	return memory, nil
}

func (r *SQLRepository) Delete(ctx context.Context, workspaceID string, memoryID string) error {
	query := `DELETE FROM memories WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND id = ` + r.store.Placeholder(2)
	result, err := r.store.DB.ExecContext(ctx, query, workspaceID, memoryID)
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

func (r *SQLRepository) Candidates(ctx context.Context, request chat.MemoryRequest) ([]Memory, error) {
	query := memorySelect(r.store) + ` WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND active = ` + r.store.Placeholder(2) +
		` AND (scope IN ('global', 'workspace') OR (scope = 'agent' AND agent_id = ` + r.store.Placeholder(3) + `) OR (scope = 'conversation' AND conversation_id = ` + r.store.Placeholder(4) + `))`
	rows, err := r.store.DB.QueryContext(ctx, query, request.WorkspaceID, true, nullableString(request.AgentID), nullableString(request.ConversationID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

func (r *SQLRepository) RecordRun(ctx context.Context, runID string, memories []chat.MemorySnippet) error {
	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	for _, memory := range memories {
		insert := `INSERT INTO chat_run_memories (chat_run_id, memory_id, rank_score, created_at) VALUES (` + placeholders(r.store, 4) + `)`
		if _, err := tx.ExecContext(ctx, insert, runID, memory.ID, memory.Score, r.store.NowArg(now)); err != nil {
			return err
		}
		update := `UPDATE memories SET last_used_at = ` + r.store.Placeholder(1) + `, use_count = use_count + 1, updated_at = ` + r.store.Placeholder(2) + ` WHERE id = ` + r.store.Placeholder(3)
		if _, err := tx.ExecContext(ctx, update, r.store.NowArg(now), r.store.NowArg(now), memory.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLRepository) UsedByRun(ctx context.Context, runID string) ([]chat.MemorySnippet, error) {
	query := `SELECT m.id, m.title, m.content, crm.rank_score FROM chat_run_memories crm JOIN memories m ON m.id = crm.memory_id
WHERE crm.chat_run_id = ` + r.store.Placeholder(1) + ` AND crm.removed_at IS NULL ORDER BY crm.rank_score DESC`
	rows, err := r.store.DB.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snippets []chat.MemorySnippet
	for rows.Next() {
		var snippet chat.MemorySnippet
		if err := rows.Scan(&snippet.ID, &snippet.Title, &snippet.Content, &snippet.Score); err != nil {
			return nil, err
		}
		snippets = append(snippets, snippet)
	}
	return snippets, rows.Err()
}

func (r *SQLRepository) RemoveFromRun(ctx context.Context, runID string, memoryID string, now time.Time) error {
	query := `UPDATE chat_run_memories SET removed_at = ` + r.store.Placeholder(1) + ` WHERE chat_run_id = ` + r.store.Placeholder(2) + ` AND memory_id = ` + r.store.Placeholder(3)
	_, err := r.store.DB.ExecContext(ctx, query, r.store.NowArg(now), runID, memoryID)
	return err
}

func RankMemories(candidates []Memory, request chat.MemoryRequest) []chat.MemorySnippet {
	terms := keywordSet(request.Query)
	var snippets []chat.MemorySnippet
	for _, memory := range candidates {
		if request.AccessMode == "none" {
			continue
		}
		if request.AccessMode == "pinned_only" && !memory.Pinned {
			continue
		}
		score := float64(memory.Importance) / 100
		if memory.Pinned {
			score += 2
		}
		if memory.Scope == "conversation" {
			score += 0.8
		}
		if memory.Scope == "agent" {
			score += 0.5
		}
		for _, tag := range memory.Tags {
			if terms[strings.ToLower(tag)] {
				score += 0.6
			}
		}
		for term := range terms {
			if strings.Contains(strings.ToLower(memory.Title+" "+memory.Content), term) {
				score += 0.25
			}
		}
		if memory.LastUsedAt != nil {
			score += 0.1
		}
		score += float64(memory.UseCount) * 0.01
		if request.AccessMode == "all" || memory.Pinned || score > 0.75 {
			snippets = append(snippets, chat.MemorySnippet{ID: memory.ID, Title: memory.Title, Content: memory.Content, Score: score})
		}
	}
	sort.SliceStable(snippets, func(i, j int) bool {
		return snippets[i].Score > snippets[j].Score
	})
	if len(snippets) > 8 {
		return snippets[:8]
	}
	return snippets
}

func memorySelect(store *database.Store) string {
	return `SELECT id, workspace_id, owner_user_id, agent_id, conversation_id, title, content, tags, scope, importance, pinned,
active, source, source_message_id, last_used_at, use_count, created_at, updated_at FROM memories`
}

func scanMemories(rows *sql.Rows) ([]Memory, error) {
	var memories []Memory
	for rows.Next() {
		memory, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, memory)
	}
	return memories, rows.Err()
}

func scanMemory(row rowScanner) (Memory, error) {
	var memory Memory
	var agentID sql.NullString
	var conversationID sql.NullString
	var tagsRaw string
	var sourceMessageID sql.NullString
	var lastUsedRaw any
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(&memory.ID, &memory.WorkspaceID, &memory.OwnerUserID, &agentID, &conversationID, &memory.Title, &memory.Content,
		&tagsRaw, &memory.Scope, &memory.Importance, &memory.Pinned, &memory.Active, &memory.Source, &sourceMessageID, &lastUsedRaw,
		&memory.UseCount, &createdRaw, &updatedRaw); err != nil {
		return Memory{}, err
	}
	memory.AgentID = agentID.String
	memory.ConversationID = conversationID.String
	memory.Tags = parseTags(tagsRaw)
	memory.SourceMessageID = sourceMessageID.String
	var err error
	memory.LastUsedAt, err = nullableTime(lastUsedRaw)
	if err != nil {
		return Memory{}, err
	}
	memory.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Memory{}, err
	}
	memory.UpdatedAt, err = database.ParseTime(updatedRaw)
	if err != nil {
		return Memory{}, err
	}
	return memory, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func keywordSet(value string) map[string]bool {
	out := map[string]bool{}
	for _, field := range strings.Fields(strings.ToLower(value)) {
		field = strings.Trim(field, ".,:;!?()[]{}\"'")
		if len(field) >= 3 {
			out[field] = true
		}
	}
	return out
}

func parseTags(raw string) []string {
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

func jsonArray(values []string) string {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			clean = append(clean, value)
		}
	}
	encoded, err := json.Marshal(clean)
	if err != nil {
		return "[]"
	}
	return string(encoded)
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
