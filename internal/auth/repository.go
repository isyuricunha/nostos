package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yuricunha/nostos/internal/database"
	"github.com/yuricunha/nostos/internal/id"
)

type Repository interface {
	OwnerExists(ctx context.Context) (bool, error)
	CreateOwner(ctx context.Context, user User, passwordHash string) (User, error)
	FindUserByEmail(ctx context.Context, email string) (UserRecord, error)
	FindUserByID(ctx context.Context, userID string) (UserRecord, error)
	CreateSession(ctx context.Context, session Session) error
	FindSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error)
	ListSessions(ctx context.Context, userID string) ([]Session, error)
	RevokeSession(ctx context.Context, sessionID string, userID string, now time.Time) error
	RevokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) error
	CleanupExpiredSessions(ctx context.Context, now time.Time) (int64, error)
	InsertAuditEvent(ctx context.Context, event AuditEvent) error
}

type SQLRepository struct {
	store *database.Store
}

type UserRecord struct {
	User
	PasswordHash string
}

var ErrNotFound = errors.New("auth record not found")

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) OwnerExists(ctx context.Context) (bool, error) {
	var count int
	err := r.store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE role = "+r.store.Placeholder(1), "owner").Scan(&count)
	return count > 0, err
}

func (r *SQLRepository) CreateOwner(ctx context.Context, user User, passwordHash string) (User, error) {
	now := time.Now().UTC()
	workspaceID := id.New()
	user.ID = id.New()
	user.WorkspaceID = workspaceID
	user.Role = "owner"
	user.CreatedAt = now
	user.UpdatedAt = now

	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	workspaceQuery := `INSERT INTO workspaces (id, name, owner_user_id, created_at, updated_at) VALUES (` +
		r.store.Placeholder(1) + `, ` + r.store.Placeholder(2) + `, ` + r.store.Placeholder(3) + `, ` + r.store.Placeholder(4) + `, ` + r.store.Placeholder(5) + `)`
	if _, err := tx.ExecContext(ctx, workspaceQuery, workspaceID, DefaultWorkspaceName, user.ID, r.store.NowArg(now), r.store.NowArg(now)); err != nil {
		return User{}, err
	}

	userQuery := `INSERT INTO users (id, workspace_id, email, display_name, password_hash, role, created_at, updated_at) VALUES (` +
		r.store.Placeholder(1) + `, ` + r.store.Placeholder(2) + `, ` + r.store.Placeholder(3) + `, ` + r.store.Placeholder(4) + `, ` +
		r.store.Placeholder(5) + `, ` + r.store.Placeholder(6) + `, ` + r.store.Placeholder(7) + `, ` + r.store.Placeholder(8) + `)`
	if _, err := tx.ExecContext(ctx, userQuery, user.ID, workspaceID, user.Email, user.DisplayName, passwordHash, user.Role, r.store.NowArg(now), r.store.NowArg(now)); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return user, nil
}

func (r *SQLRepository) FindUserByEmail(ctx context.Context, email string) (UserRecord, error) {
	query := `SELECT id, workspace_id, email, display_name, password_hash, role, disabled_at, created_at, updated_at
FROM users WHERE lower(email) = lower(` + r.store.Placeholder(1) + `)`
	return r.scanUser(ctx, query, email)
}

func (r *SQLRepository) FindUserByID(ctx context.Context, userID string) (UserRecord, error) {
	query := `SELECT id, workspace_id, email, display_name, password_hash, role, disabled_at, created_at, updated_at
FROM users WHERE id = ` + r.store.Placeholder(1)
	return r.scanUser(ctx, query, userID)
}

func (r *SQLRepository) CreateSession(ctx context.Context, session Session) error {
	query := `INSERT INTO sessions (id, user_id, token_hash, csrf_token_hash, ip_address, user_agent, expires_at, created_at, updated_at)
VALUES (` + r.store.Placeholder(1) + `, ` + r.store.Placeholder(2) + `, ` + r.store.Placeholder(3) + `, ` + r.store.Placeholder(4) + `, ` +
		r.store.Placeholder(5) + `, ` + r.store.Placeholder(6) + `, ` + r.store.Placeholder(7) + `, ` + r.store.Placeholder(8) + `, ` + r.store.Placeholder(9) + `)`
	_, err := r.store.DB.ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.CSRFTokenHash,
		nullableString(session.IPAddress),
		nullableString(session.UserAgent),
		r.store.NowArg(session.ExpiresAt),
		r.store.NowArg(session.CreatedAt),
		r.store.NowArg(session.UpdatedAt),
	)
	return err
}

func (r *SQLRepository) FindSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	query := `SELECT id, user_id, token_hash, csrf_token_hash, ip_address, user_agent, expires_at, revoked_at, created_at, updated_at
FROM sessions WHERE token_hash = ` + r.store.Placeholder(1)
	return r.scanSession(ctx, query, tokenHash)
}

func (r *SQLRepository) ListSessions(ctx context.Context, userID string) ([]Session, error) {
	query := `SELECT id, user_id, token_hash, csrf_token_hash, ip_address, user_agent, expires_at, revoked_at, created_at, updated_at
FROM sessions WHERE user_id = ` + r.store.Placeholder(1) + ` AND revoked_at IS NULL AND expires_at > ` + r.store.Placeholder(2) + ` ORDER BY created_at DESC`
	rows, err := r.store.DB.QueryContext(ctx, query, userID, r.store.NowArg(time.Now().UTC()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		session, err := scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (r *SQLRepository) RevokeSession(ctx context.Context, sessionID string, userID string, now time.Time) error {
	query := `UPDATE sessions SET revoked_at = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
		` WHERE id = ` + r.store.Placeholder(3) + ` AND user_id = ` + r.store.Placeholder(4)
	result, err := r.store.DB.ExecContext(ctx, query, r.store.NowArg(now), r.store.NowArg(now), sessionID, userID)
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

func (r *SQLRepository) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, now time.Time) error {
	query := `UPDATE sessions SET revoked_at = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
		` WHERE token_hash = ` + r.store.Placeholder(3)
	_, err := r.store.DB.ExecContext(ctx, query, r.store.NowArg(now), r.store.NowArg(now), tokenHash)
	return err
}

func (r *SQLRepository) CleanupExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	query := `DELETE FROM sessions WHERE expires_at < ` + r.store.Placeholder(1)
	result, err := r.store.DB.ExecContext(ctx, query, r.store.NowArg(now))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *SQLRepository) InsertAuditEvent(ctx context.Context, event AuditEvent) error {
	if event.ID == "" {
		event.ID = id.New()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	metadataBytes, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	query := `INSERT INTO audit_logs (id, workspace_id, actor_user_id, event_type, ip_address, user_agent, metadata, created_at)
VALUES (` + r.store.Placeholder(1) + `, ` + r.store.Placeholder(2) + `, ` + r.store.Placeholder(3) + `, ` + r.store.Placeholder(4) + `, ` +
		r.store.Placeholder(5) + `, ` + r.store.Placeholder(6) + `, ` + r.store.Placeholder(7) + `, ` + r.store.Placeholder(8) + `)`
	_, err = r.store.DB.ExecContext(ctx, query,
		event.ID,
		nullableString(event.WorkspaceID),
		nullableString(event.ActorUserID),
		event.EventType,
		nullableString(event.IPAddress),
		nullableString(event.UserAgent),
		string(metadataBytes),
		r.store.NowArg(event.CreatedAt),
	)
	return err
}

func (r *SQLRepository) scanUser(ctx context.Context, query string, args ...any) (UserRecord, error) {
	row := r.store.DB.QueryRowContext(ctx, query, args...)
	var record UserRecord
	var disabledRaw any
	var createdRaw any
	var updatedRaw any
	var workspaceID sql.NullString
	err := row.Scan(
		&record.ID,
		&workspaceID,
		&record.Email,
		&record.DisplayName,
		&record.PasswordHash,
		&record.Role,
		&disabledRaw,
		&createdRaw,
		&updatedRaw,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return UserRecord{}, ErrNotFound
	}
	if err != nil {
		return UserRecord{}, err
	}
	record.WorkspaceID = workspaceID.String
	disabledAt, err := nullableTime(disabledRaw)
	if err != nil {
		return UserRecord{}, err
	}
	createdAt, err := database.ParseTime(createdRaw)
	if err != nil {
		return UserRecord{}, err
	}
	updatedAt, err := database.ParseTime(updatedRaw)
	if err != nil {
		return UserRecord{}, err
	}
	record.DisabledAt = disabledAt
	record.CreatedAt = createdAt
	record.UpdatedAt = updatedAt
	return record, nil
}

func (r *SQLRepository) scanSession(ctx context.Context, query string, args ...any) (Session, error) {
	session, err := scanSessionRow(r.store.DB.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return session, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSessionRow(row rowScanner) (Session, error) {
	var session Session
	var ip sql.NullString
	var userAgent sql.NullString
	var expiresRaw any
	var revokedRaw any
	var createdRaw any
	var updatedRaw any
	if err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.CSRFTokenHash,
		&ip,
		&userAgent,
		&expiresRaw,
		&revokedRaw,
		&createdRaw,
		&updatedRaw,
	); err != nil {
		return Session{}, err
	}
	session.IPAddress = ip.String
	session.UserAgent = userAgent.String
	expiresAt, err := database.ParseTime(expiresRaw)
	if err != nil {
		return Session{}, err
	}
	revokedAt, err := nullableTime(revokedRaw)
	if err != nil {
		return Session{}, err
	}
	createdAt, err := database.ParseTime(createdRaw)
	if err != nil {
		return Session{}, err
	}
	updatedAt, err := database.ParseTime(updatedRaw)
	if err != nil {
		return Session{}, err
	}
	session.ExpiresAt = expiresAt
	session.RevokedAt = revokedAt
	session.CreatedAt = createdAt
	session.UpdatedAt = updatedAt
	return session, nil
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

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func WrapNotFound(entity string, err error) error {
	if errors.Is(err, ErrNotFound) {
		return fmt.Errorf("%s: %w", entity, ErrNotFound)
	}
	return err
}
