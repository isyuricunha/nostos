package feedback

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/id"
)

var ErrNotFound = errors.New("feedback record not found")

type Repository interface {
	ListForConversation(ctx context.Context, workspaceID string, userID string, conversationID string) ([]MessageFeedback, error)
	Upsert(ctx context.Context, workspaceID string, userID string, messageID string, input FeedbackInput) (MessageFeedback, error)
	Delete(ctx context.Context, workspaceID string, userID string, messageID string) error
	Stats(ctx context.Context, workspaceID string) (FeedbackStats, error)
}

type SQLRepository struct {
	store *database.Store
}

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) ListForConversation(ctx context.Context, workspaceID string, userID string, conversationID string) ([]MessageFeedback, error) {
	query := feedbackSelect(r.store) + ` JOIN messages m ON m.id = mf.message_id
JOIN conversations c ON c.id = m.conversation_id
WHERE c.workspace_id = ` + r.store.Placeholder(1) + ` AND c.owner_user_id = ` + r.store.Placeholder(2) +
		` AND c.id = ` + r.store.Placeholder(3) + ` AND mf.user_id = ` + r.store.Placeholder(4) + ` ORDER BY mf.updated_at DESC`
	rows, err := r.store.DB.QueryContext(ctx, query, workspaceID, userID, conversationID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []MessageFeedback
	for rows.Next() {
		item, err := scanFeedback(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *SQLRepository) Upsert(ctx context.Context, workspaceID string, userID string, messageID string, input FeedbackInput) (MessageFeedback, error) {
	providerID, model, err := r.assistantMessageProvider(ctx, workspaceID, userID, messageID)
	if err != nil {
		return MessageFeedback{}, err
	}
	now := time.Now().UTC()
	item := MessageFeedback{
		ID:         id.New(),
		MessageID:  messageID,
		UserID:     userID,
		Rating:     input.Rating,
		Reason:     strings.TrimSpace(input.Reason),
		Comment:    strings.TrimSpace(input.Comment),
		ProviderID: providerID,
		Model:      model,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	query := `INSERT INTO message_feedback (id, message_id, user_id, rating, reason, comment, provider_id, model, created_at, updated_at)
VALUES (` + placeholders(r.store, 10) + `)
ON CONFLICT(message_id, user_id) DO UPDATE SET rating = excluded.rating, reason = excluded.reason, comment = excluded.comment,
provider_id = excluded.provider_id, model = excluded.model, updated_at = excluded.updated_at`
	if _, err := r.store.DB.ExecContext(ctx, query, item.ID, item.MessageID, item.UserID, item.Rating, nullableString(item.Reason),
		nullableString(item.Comment), nullableString(item.ProviderID), nullableString(item.Model), r.store.NowArg(now), r.store.NowArg(now)); err != nil {
		return MessageFeedback{}, err
	}
	return r.get(ctx, workspaceID, userID, messageID)
}

func (r *SQLRepository) Delete(ctx context.Context, workspaceID string, userID string, messageID string) error {
	query := `DELETE FROM message_feedback WHERE message_id = ` + r.store.Placeholder(1) + ` AND user_id = ` + r.store.Placeholder(2) +
		` AND message_id IN (SELECT m.id FROM messages m JOIN conversations c ON c.id = m.conversation_id WHERE c.workspace_id = ` +
		r.store.Placeholder(3) + ` AND c.owner_user_id = ` + r.store.Placeholder(4) + `)`
	result, err := r.store.DB.ExecContext(ctx, query, messageID, userID, workspaceID, userID)
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

func (r *SQLRepository) Stats(ctx context.Context, workspaceID string) (FeedbackStats, error) {
	query := `SELECT rating, COUNT(*) FROM message_feedback mf JOIN messages m ON m.id = mf.message_id
JOIN conversations c ON c.id = m.conversation_id WHERE c.workspace_id = ` + r.store.Placeholder(1) + ` GROUP BY rating`
	rows, err := r.store.DB.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return FeedbackStats{}, err
	}
	defer rows.Close()
	var stats FeedbackStats
	for rows.Next() {
		var rating string
		var count int
		if err := rows.Scan(&rating, &count); err != nil {
			return FeedbackStats{}, err
		}
		switch rating {
		case RatingPositive:
			stats.Positive = count
		case RatingNegative:
			stats.Negative = count
		}
	}
	return stats, rows.Err()
}

func (r *SQLRepository) get(ctx context.Context, workspaceID string, userID string, messageID string) (MessageFeedback, error) {
	query := feedbackSelect(r.store) + ` JOIN messages m ON m.id = mf.message_id
JOIN conversations c ON c.id = m.conversation_id
WHERE c.workspace_id = ` + r.store.Placeholder(1) + ` AND c.owner_user_id = ` + r.store.Placeholder(2) +
		` AND mf.message_id = ` + r.store.Placeholder(3) + ` AND mf.user_id = ` + r.store.Placeholder(4)
	item, err := scanFeedback(r.store.DB.QueryRowContext(ctx, query, workspaceID, userID, messageID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return MessageFeedback{}, ErrNotFound
	}
	return item, err
}

func (r *SQLRepository) assistantMessageProvider(ctx context.Context, workspaceID string, userID string, messageID string) (string, string, error) {
	var providerID sql.NullString
	var model sql.NullString
	query := `SELECT m.provider_id, m.model FROM messages m JOIN conversations c ON c.id = m.conversation_id
WHERE c.workspace_id = ` + r.store.Placeholder(1) + ` AND c.owner_user_id = ` + r.store.Placeholder(2) +
		` AND m.id = ` + r.store.Placeholder(3) + ` AND m.role = ` + r.store.Placeholder(4)
	err := r.store.DB.QueryRowContext(ctx, query, workspaceID, userID, messageID, "assistant").Scan(&providerID, &model)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return providerID.String, model.String, err
}

func feedbackSelect(store *database.Store) string {
	return `SELECT mf.id, mf.message_id, mf.user_id, mf.rating, mf.reason, mf.comment, mf.provider_id, mf.model, mf.created_at, mf.updated_at FROM message_feedback mf`
}

func scanFeedback(row rowScanner) (MessageFeedback, error) {
	var item MessageFeedback
	var reason, comment, providerID, model sql.NullString
	var createdRaw, updatedRaw any
	if err := row.Scan(&item.ID, &item.MessageID, &item.UserID, &item.Rating, &reason, &comment, &providerID, &model, &createdRaw, &updatedRaw); err != nil {
		return MessageFeedback{}, err
	}
	item.Reason = reason.String
	item.Comment = comment.String
	item.ProviderID = providerID.String
	item.Model = model.String
	var err error
	item.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return MessageFeedback{}, err
	}
	item.UpdatedAt, err = database.ParseTime(updatedRaw)
	return item, err
}

type rowScanner interface{ Scan(dest ...any) error }

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
