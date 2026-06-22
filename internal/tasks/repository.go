package tasks

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/isyuricunha/nostos/internal/database"
	"github.com/isyuricunha/nostos/internal/id"
)

var ErrNotFound = errors.New("task record not found")

type Repository interface {
	ListTasks(ctx context.Context, workspaceID string) ([]Task, error)
	GetTask(ctx context.Context, taskID string) (Task, error)
	GetRun(ctx context.Context, runID string) (Run, error)
	CreateTask(ctx context.Context, task Task, schedule Schedule) (Task, Schedule, error)
	UpdateTask(ctx context.Context, task Task, schedule Schedule) (Task, Schedule, error)
	DeleteTask(ctx context.Context, workspaceID string, taskID string) error
	GetSchedule(ctx context.Context, taskID string) (Schedule, error)
	DueSchedules(ctx context.Context, now time.Time) ([]Schedule, error)
	MarkScheduleEnqueued(ctx context.Context, schedule Schedule) error
	EnqueueRun(ctx context.Context, run Run) (Run, error)
	ClaimRun(ctx context.Context, workerID string, leaseUntil time.Time) (Run, error)
	RenewLease(ctx context.Context, runID string, workerID string, leaseUntil time.Time) error
	CompleteRun(ctx context.Context, runID string, state string, result string, errorMessage string) error
	ActiveRunCount(ctx context.Context, taskID string) (int, error)
	CancelActiveRuns(ctx context.Context, taskID string, reason string) (int64, error)
	CancelRun(ctx context.Context, workspaceID string, runID string) error
	ListRuns(ctx context.Context, workspaceID string, taskID string) ([]Run, error)
	ListEvents(ctx context.Context, runID string) ([]Event, error)
	AppendEvent(ctx context.Context, event Event) error
	RecoverExpiredLeases(ctx context.Context, now time.Time) (int64, error)
	CleanupExpiredSessions(ctx context.Context, now time.Time) (int64, error)
	PruneOldTaskRunEvents(ctx context.Context, cutoff time.Time) (int64, error)
	CompactDuplicateSchedulingEvents(ctx context.Context) (int64, error)
	Workspaces(ctx context.Context) ([]string, error)
	SystemTaskExists(ctx context.Context, workspaceID string, name string) (bool, error)
	GetSystemTaskByName(ctx context.Context, workspaceID string, name string) (Task, error)
}

type SQLRepository struct {
	store *database.Store
}

func NewSQLRepository(store *database.Store) *SQLRepository {
	return &SQLRepository{store: store}
}

func (r *SQLRepository) ListTasks(ctx context.Context, workspaceID string) ([]Task, error) {
	query := taskSelect(r.store) + ` WHERE workspace_id = ` + r.store.Placeholder(1) + ` ORDER BY system_managed DESC, name`
	rows, err := r.store.DB.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r *SQLRepository) GetTask(ctx context.Context, taskID string) (Task, error) {
	task, err := scanTask(r.store.DB.QueryRowContext(ctx, taskSelect(r.store)+` WHERE id = `+r.store.Placeholder(1), taskID))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

func (r *SQLRepository) GetRun(ctx context.Context, runID string) (Run, error) {
	run, err := scanRun(r.store.DB.QueryRowContext(ctx, runSelect(r.store)+` WHERE id = `+r.store.Placeholder(1), runID))
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	return run, err
}

func (r *SQLRepository) CreateTask(ctx context.Context, task Task, schedule Schedule) (Task, Schedule, error) {
	now := time.Now().UTC()
	task.ID = id.New()
	task.CreatedAt = now
	task.UpdatedAt = now
	schedule.ID = id.New()
	schedule.TaskID = task.ID
	schedule.CreatedAt = now
	schedule.UpdatedAt = now
	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, Schedule{}, err
	}
	defer tx.Rollback()
	taskQuery := `INSERT INTO tasks (id, workspace_id, name, description, task_type, state, system_managed, agent_id, provider_id, model,
prompt, tool_policy, max_retries, timeout_ms, concurrency_policy, created_at, updated_at) VALUES (` + placeholders(r.store, 17) + `)`
	if _, err := tx.ExecContext(ctx, taskQuery, task.ID, task.WorkspaceID, task.Name, task.Description, task.TaskType, task.State, task.SystemManaged,
		nullableString(task.AgentID), nullableString(task.ProviderID), nullableString(task.Model), task.Prompt, task.ToolPolicy, task.MaxRetries,
		task.TimeoutMS, task.ConcurrencyPolicy, r.store.NowArg(now), r.store.NowArg(now)); err != nil {
		return Task{}, Schedule{}, err
	}
	scheduleQuery := `INSERT INTO task_schedules (id, task_id, mode, cron_expression, interval_seconds, run_at, timezone, enabled, next_run_at, created_at, updated_at)
VALUES (` + placeholders(r.store, 11) + `)`
	if _, err := tx.ExecContext(ctx, scheduleQuery, schedule.ID, schedule.TaskID, schedule.Mode, nullableString(schedule.CronExpression),
		nullInt(schedule.IntervalSeconds), timePtrArg(r.store, schedule.RunAt), schedule.Timezone, schedule.Enabled, timePtrArg(r.store, schedule.NextRunAt),
		r.store.NowArg(now), r.store.NowArg(now)); err != nil {
		return Task{}, Schedule{}, err
	}
	return task, schedule, tx.Commit()
}

func (r *SQLRepository) UpdateTask(ctx context.Context, task Task, schedule Schedule) (Task, Schedule, error) {
	now := time.Now().UTC()
	task.UpdatedAt = now
	schedule.UpdatedAt = now
	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, Schedule{}, err
	}
	defer tx.Rollback()
	taskQuery := `UPDATE tasks SET name = ` + r.store.Placeholder(1) + `, description = ` + r.store.Placeholder(2) + `, task_type = ` + r.store.Placeholder(3) +
		`, state = ` + r.store.Placeholder(4) + `, agent_id = ` + r.store.Placeholder(5) + `, provider_id = ` + r.store.Placeholder(6) +
		`, model = ` + r.store.Placeholder(7) + `, prompt = ` + r.store.Placeholder(8) + `, tool_policy = ` + r.store.Placeholder(9) +
		`, max_retries = ` + r.store.Placeholder(10) + `, timeout_ms = ` + r.store.Placeholder(11) + `, concurrency_policy = ` + r.store.Placeholder(12) +
		`, updated_at = ` + r.store.Placeholder(13) + ` WHERE workspace_id = ` + r.store.Placeholder(14) + ` AND id = ` + r.store.Placeholder(15)
	result, err := tx.ExecContext(ctx, taskQuery, task.Name, task.Description, task.TaskType, task.State, nullableString(task.AgentID), nullableString(task.ProviderID),
		nullableString(task.Model), task.Prompt, task.ToolPolicy, task.MaxRetries, task.TimeoutMS, task.ConcurrencyPolicy, r.store.NowArg(now), task.WorkspaceID, task.ID)
	if err != nil {
		return Task{}, Schedule{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Task{}, Schedule{}, err
	}
	if affected == 0 {
		return Task{}, Schedule{}, ErrNotFound
	}
	scheduleQuery := `UPDATE task_schedules SET mode = ` + r.store.Placeholder(1) + `, cron_expression = ` + r.store.Placeholder(2) +
		`, interval_seconds = ` + r.store.Placeholder(3) + `, run_at = ` + r.store.Placeholder(4) + `, timezone = ` + r.store.Placeholder(5) +
		`, enabled = ` + r.store.Placeholder(6) + `, next_run_at = ` + r.store.Placeholder(7) + `, updated_at = ` + r.store.Placeholder(8) +
		` WHERE task_id = ` + r.store.Placeholder(9)
	if _, err := tx.ExecContext(ctx, scheduleQuery, schedule.Mode, nullableString(schedule.CronExpression), nullInt(schedule.IntervalSeconds),
		timePtrArg(r.store, schedule.RunAt), schedule.Timezone, schedule.Enabled, timePtrArg(r.store, schedule.NextRunAt), r.store.NowArg(now), task.ID); err != nil {
		return Task{}, Schedule{}, err
	}
	return task, schedule, tx.Commit()
}

func (r *SQLRepository) DeleteTask(ctx context.Context, workspaceID string, taskID string) error {
	result, err := r.store.DB.ExecContext(ctx, `DELETE FROM tasks WHERE workspace_id = `+r.store.Placeholder(1)+` AND id = `+r.store.Placeholder(2), workspaceID, taskID)
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

func (r *SQLRepository) GetSchedule(ctx context.Context, taskID string) (Schedule, error) {
	schedule, err := scanSchedule(r.store.DB.QueryRowContext(ctx, scheduleSelect(r.store)+` WHERE task_id = `+r.store.Placeholder(1)+` LIMIT 1`, taskID))
	if errors.Is(err, sql.ErrNoRows) {
		return Schedule{}, ErrNotFound
	}
	return schedule, err
}

func (r *SQLRepository) DueSchedules(ctx context.Context, now time.Time) ([]Schedule, error) {
	query := scheduleSelect(r.store) + ` WHERE enabled = ` + r.store.Placeholder(1) + ` AND next_run_at IS NOT NULL AND next_run_at <= ` + r.store.Placeholder(2)
	rows, err := r.store.DB.QueryContext(ctx, query, true, r.store.NowArg(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var schedules []Schedule
	for rows.Next() {
		schedule, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	return schedules, rows.Err()
}

func (r *SQLRepository) MarkScheduleEnqueued(ctx context.Context, schedule Schedule) error {
	query := `UPDATE task_schedules SET last_enqueued_occurrence = ` + r.store.Placeholder(1) + `, next_run_at = ` + r.store.Placeholder(2) +
		`, enabled = ` + r.store.Placeholder(3) + `, updated_at = ` + r.store.Placeholder(4) + ` WHERE id = ` + r.store.Placeholder(5)
	_, err := r.store.DB.ExecContext(ctx, query, schedule.LastEnqueuedOccurrence, timePtrArg(r.store, schedule.NextRunAt), schedule.Enabled, r.store.NowArg(time.Now().UTC()), schedule.ID)
	return err
}

func (r *SQLRepository) EnqueueRun(ctx context.Context, run Run) (Run, error) {
	now := time.Now().UTC()
	run.ID = id.New()
	run.State = RunQueued
	run.QueuedAt = now
	run.CreatedAt = now
	run.UpdatedAt = now
	query := `INSERT INTO task_runs (id, task_id, schedule_id, idempotency_key, state, attempt, max_retries, timeout_ms, queued_at, created_at, updated_at)
VALUES (` + placeholders(r.store, 11) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, run.ID, run.TaskID, nullableString(run.ScheduleID), run.IdempotencyKey, run.State, run.Attempt, run.MaxRetries, run.TimeoutMS, r.store.NowArg(now), r.store.NowArg(now), r.store.NowArg(now))
	return run, err
}

func (r *SQLRepository) ClaimRun(ctx context.Context, workerID string, leaseUntil time.Time) (Run, error) {
	now := time.Now().UTC()
	if r.store.Dialect == database.Postgres {
		query := `UPDATE task_runs SET state = $1, lease_owner = $2, lease_expires_at = $3, started_at = COALESCE(started_at, $4), updated_at = $5
WHERE id = (SELECT id FROM task_runs WHERE state = $6 OR (state IN ($7, $8) AND lease_expires_at < $9) ORDER BY queued_at LIMIT 1 FOR UPDATE SKIP LOCKED)
RETURNING id, task_id, schedule_id, idempotency_key, state, attempt, max_retries, timeout_ms, lease_owner, lease_expires_at, queued_at, started_at, completed_at, result, error_message, created_at, updated_at`
		run, err := scanRun(r.store.DB.QueryRowContext(ctx, query, RunRunning, workerID, r.store.NowArg(leaseUntil), r.store.NowArg(now), r.store.NowArg(now), RunQueued, RunClaimed, RunRunning, r.store.NowArg(now)))
		if errors.Is(err, sql.ErrNoRows) {
			return Run{}, ErrNotFound
		}
		return run, err
	}
	tx, err := r.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback()
	selectQuery := `SELECT id FROM task_runs WHERE state = ` + r.store.Placeholder(1) + ` OR (state IN (` + r.store.Placeholder(2) + `, ` + r.store.Placeholder(3) + `) AND lease_expires_at < ` + r.store.Placeholder(4) + `) ORDER BY queued_at LIMIT 1`
	var runID string
	if err := tx.QueryRowContext(ctx, selectQuery, RunQueued, RunClaimed, RunRunning, r.store.NowArg(now)).Scan(&runID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Run{}, ErrNotFound
		}
		return Run{}, err
	}
	updateQuery := `UPDATE task_runs SET state = ` + r.store.Placeholder(1) + `, lease_owner = ` + r.store.Placeholder(2) + `, lease_expires_at = ` + r.store.Placeholder(3) + `, started_at = COALESCE(started_at, ` + r.store.Placeholder(4) + `), updated_at = ` + r.store.Placeholder(5) + ` WHERE id = ` + r.store.Placeholder(6)
	if _, err := tx.ExecContext(ctx, updateQuery, RunRunning, workerID, r.store.NowArg(leaseUntil), r.store.NowArg(now), r.store.NowArg(now), runID); err != nil {
		return Run{}, err
	}
	run, err := scanRun(tx.QueryRowContext(ctx, runSelect(r.store)+` WHERE id = `+r.store.Placeholder(1), runID))
	if err != nil {
		return Run{}, err
	}
	return run, tx.Commit()
}

func (r *SQLRepository) RenewLease(ctx context.Context, runID string, workerID string, leaseUntil time.Time) error {
	query := `UPDATE task_runs SET lease_expires_at = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
		` WHERE id = ` + r.store.Placeholder(3) + ` AND lease_owner = ` + r.store.Placeholder(4) +
		` AND state = ` + r.store.Placeholder(5)
	result, err := r.store.DB.ExecContext(ctx, query, r.store.NowArg(leaseUntil), r.store.NowArg(time.Now().UTC()), runID, workerID, RunRunning)
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

func (r *SQLRepository) CompleteRun(ctx context.Context, runID string, state string, resultText string, errorMessage string) error {
	now := time.Now().UTC()
	query := `UPDATE task_runs SET state = ` + r.store.Placeholder(1) + `, result = ` + r.store.Placeholder(2) + `, error_message = ` + r.store.Placeholder(3) +
		`, completed_at = ` + r.store.Placeholder(4) + `, lease_owner = NULL, lease_expires_at = NULL, updated_at = ` + r.store.Placeholder(5) + ` WHERE id = ` + r.store.Placeholder(6)
	_, err := r.store.DB.ExecContext(ctx, query, state, nullableString(resultText), nullableString(errorMessage), r.store.NowArg(now), r.store.NowArg(now), runID)
	return err
}

func (r *SQLRepository) ActiveRunCount(ctx context.Context, taskID string) (int, error) {
	query := `SELECT COUNT(*) FROM task_runs WHERE task_id = ` + r.store.Placeholder(1) +
		` AND state IN (` + r.store.Placeholder(2) + `, ` + r.store.Placeholder(3) + `, ` + r.store.Placeholder(4) + `, ` + r.store.Placeholder(5) + `)`
	var count int
	err := r.store.DB.QueryRowContext(ctx, query, taskID, RunQueued, RunClaimed, RunRunning, RunWaiting).Scan(&count)
	return count, err
}

func (r *SQLRepository) CancelActiveRuns(ctx context.Context, taskID string, reason string) (int64, error) {
	now := time.Now().UTC()
	query := `UPDATE task_runs SET state = ` + r.store.Placeholder(1) + `, error_message = ` + r.store.Placeholder(2) +
		`, completed_at = ` + r.store.Placeholder(3) + `, lease_owner = NULL, lease_expires_at = NULL, updated_at = ` + r.store.Placeholder(4) +
		` WHERE task_id = ` + r.store.Placeholder(5) + ` AND state IN (` + r.store.Placeholder(6) + `, ` + r.store.Placeholder(7) + `, ` + r.store.Placeholder(8) + `, ` + r.store.Placeholder(9) + `)`
	result, err := r.store.DB.ExecContext(ctx, query, RunCancelled, nullableString(reason), r.store.NowArg(now), r.store.NowArg(now), taskID, RunQueued, RunClaimed, RunRunning, RunWaiting)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *SQLRepository) CancelRun(ctx context.Context, workspaceID string, runID string) error {
	query := `UPDATE task_runs SET state = ` + r.store.Placeholder(1) + ` WHERE id = ` + r.store.Placeholder(2) + ` AND task_id IN (SELECT id FROM tasks WHERE workspace_id = ` + r.store.Placeholder(3) + `)`
	result, err := r.store.DB.ExecContext(ctx, query, RunCancelled, runID, workspaceID)
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

func (r *SQLRepository) ListRuns(ctx context.Context, workspaceID string, taskID string) ([]Run, error) {
	args := []any{workspaceID}
	query := runSelect(r.store) + ` WHERE task_id IN (SELECT id FROM tasks WHERE workspace_id = ` + r.store.Placeholder(1) + `)`
	if strings.TrimSpace(taskID) != "" {
		args = append(args, taskID)
		query += ` AND task_id = ` + r.store.Placeholder(2)
	}
	query += ` ORDER BY queued_at DESC LIMIT 100`
	rows, err := r.store.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *SQLRepository) ListEvents(ctx context.Context, runID string) ([]Event, error) {
	rows, err := r.store.DB.QueryContext(ctx, eventSelect(r.store)+` WHERE task_run_id = `+r.store.Placeholder(1)+` ORDER BY created_at`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *SQLRepository) AppendEvent(ctx context.Context, event Event) error {
	now := time.Now().UTC()
	event.ID = id.New()
	event.CreatedAt = now
	query := `INSERT INTO task_run_events (id, task_run_id, level, message, created_at) VALUES (` + placeholders(r.store, 5) + `)`
	_, err := r.store.DB.ExecContext(ctx, query, event.ID, event.RunID, event.Level, event.Message, r.store.NowArg(now))
	return err
}

func (r *SQLRepository) RecoverExpiredLeases(ctx context.Context, now time.Time) (int64, error) {
	query := `UPDATE task_runs SET state = ` + r.store.Placeholder(1) + `, lease_owner = NULL, lease_expires_at = NULL, updated_at = ` + r.store.Placeholder(2) +
		` WHERE state IN (` + r.store.Placeholder(3) + `, ` + r.store.Placeholder(4) + `) AND lease_expires_at < ` + r.store.Placeholder(5)
	result, err := r.store.DB.ExecContext(ctx, query, RunQueued, r.store.NowArg(now), RunClaimed, RunRunning, r.store.NowArg(now))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *SQLRepository) CleanupExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	query := `UPDATE sessions SET revoked_at = ` + r.store.Placeholder(1) + `, updated_at = ` + r.store.Placeholder(2) +
		` WHERE revoked_at IS NULL AND expires_at < ` + r.store.Placeholder(3)
	result, err := r.store.DB.ExecContext(ctx, query, r.store.NowArg(now), r.store.NowArg(now), r.store.NowArg(now))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *SQLRepository) PruneOldTaskRunEvents(ctx context.Context, cutoff time.Time) (int64, error) {
	query := `DELETE FROM task_run_events WHERE created_at < ` + r.store.Placeholder(1)
	result, err := r.store.DB.ExecContext(ctx, query, r.store.NowArg(cutoff))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *SQLRepository) CompactDuplicateSchedulingEvents(ctx context.Context) (int64, error) {
	rows, err := r.store.DB.QueryContext(ctx, eventSelect(r.store)+` ORDER BY task_run_id, level, message, created_at`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var duplicates []string
	var previous Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return 0, err
		}
		if event.RunID == previous.RunID && event.Level == previous.Level && event.Message == previous.Message && event.CreatedAt.Sub(previous.CreatedAt) <= time.Second {
			duplicates = append(duplicates, event.ID)
			continue
		}
		previous = event
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	var deleted int64
	for _, eventID := range duplicates {
		result, err := r.store.DB.ExecContext(ctx, `DELETE FROM task_run_events WHERE id = `+r.store.Placeholder(1), eventID)
		if err != nil {
			return deleted, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return deleted, err
		}
		deleted += affected
	}
	return deleted, nil
}

func (r *SQLRepository) Workspaces(ctx context.Context) ([]string, error) {
	rows, err := r.store.DB.QueryContext(ctx, "SELECT id FROM workspaces")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *SQLRepository) SystemTaskExists(ctx context.Context, workspaceID string, name string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM tasks WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND name = ` + r.store.Placeholder(2) + ` AND system_managed = ` + r.store.Placeholder(3)
	err := r.store.DB.QueryRowContext(ctx, query, workspaceID, name, true).Scan(&count)
	return count > 0, err
}

func (r *SQLRepository) GetSystemTaskByName(ctx context.Context, workspaceID string, name string) (Task, error) {
	query := taskSelect(r.store) + ` WHERE workspace_id = ` + r.store.Placeholder(1) + ` AND name = ` + r.store.Placeholder(2) +
		` AND system_managed = ` + r.store.Placeholder(3) + ` LIMIT 1`
	task, err := scanTask(r.store.DB.QueryRowContext(ctx, query, workspaceID, name, true))
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	return task, err
}

func taskSelect(store *database.Store) string {
	return `SELECT id, workspace_id, name, description, task_type, state, system_managed, agent_id, provider_id, model, prompt, tool_policy,
max_retries, timeout_ms, concurrency_policy, result, last_error, created_at, updated_at FROM tasks`
}

func scheduleSelect(store *database.Store) string {
	return `SELECT id, task_id, mode, cron_expression, interval_seconds, run_at, timezone, enabled, next_run_at, last_enqueued_occurrence, created_at, updated_at FROM task_schedules`
}

func runSelect(store *database.Store) string {
	return `SELECT id, task_id, schedule_id, idempotency_key, state, attempt, max_retries, timeout_ms, lease_owner, lease_expires_at, queued_at, started_at, completed_at, result, error_message, created_at, updated_at FROM task_runs`
}

func eventSelect(store *database.Store) string {
	return `SELECT id, task_run_id, level, message, created_at FROM task_run_events`
}

func scanTask(row rowScanner) (Task, error) {
	var task Task
	var agentID, providerID, model, resultText, lastError sql.NullString
	var createdRaw, updatedRaw any
	if err := row.Scan(&task.ID, &task.WorkspaceID, &task.Name, &task.Description, &task.TaskType, &task.State, &task.SystemManaged,
		&agentID, &providerID, &model, &task.Prompt, &task.ToolPolicy, &task.MaxRetries, &task.TimeoutMS, &task.ConcurrencyPolicy,
		&resultText, &lastError, &createdRaw, &updatedRaw); err != nil {
		return Task{}, err
	}
	task.AgentID = agentID.String
	task.ProviderID = providerID.String
	task.Model = model.String
	task.Result = resultText.String
	task.LastError = lastError.String
	var err error
	task.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Task{}, err
	}
	task.UpdatedAt, err = database.ParseTime(updatedRaw)
	return task, err
}

func scanSchedule(row rowScanner) (Schedule, error) {
	var schedule Schedule
	var cronExpression sql.NullString
	var interval sql.NullInt64
	var runAtRaw, nextRunRaw, createdRaw, updatedRaw any
	var lastOccurrence sql.NullString
	if err := row.Scan(&schedule.ID, &schedule.TaskID, &schedule.Mode, &cronExpression, &interval, &runAtRaw, &schedule.Timezone,
		&schedule.Enabled, &nextRunRaw, &lastOccurrence, &createdRaw, &updatedRaw); err != nil {
		return Schedule{}, err
	}
	schedule.CronExpression = cronExpression.String
	schedule.IntervalSeconds = int(interval.Int64)
	schedule.LastEnqueuedOccurrence = lastOccurrence.String
	var err error
	schedule.RunAt, err = nullableTime(runAtRaw)
	if err != nil {
		return Schedule{}, err
	}
	schedule.NextRunAt, err = nullableTime(nextRunRaw)
	if err != nil {
		return Schedule{}, err
	}
	schedule.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Schedule{}, err
	}
	schedule.UpdatedAt, err = database.ParseTime(updatedRaw)
	return schedule, err
}

func scanRun(row rowScanner) (Run, error) {
	var run Run
	var scheduleID, leaseOwner, resultText, errorMessage sql.NullString
	var leaseRaw, queuedRaw, startedRaw, completedRaw, createdRaw, updatedRaw any
	if err := row.Scan(&run.ID, &run.TaskID, &scheduleID, &run.IdempotencyKey, &run.State, &run.Attempt, &run.MaxRetries, &run.TimeoutMS,
		&leaseOwner, &leaseRaw, &queuedRaw, &startedRaw, &completedRaw, &resultText, &errorMessage, &createdRaw, &updatedRaw); err != nil {
		return Run{}, err
	}
	run.ScheduleID = scheduleID.String
	run.LeaseOwner = leaseOwner.String
	run.Result = resultText.String
	run.ErrorMessage = errorMessage.String
	var err error
	run.LeaseExpiresAt, err = nullableTime(leaseRaw)
	if err != nil {
		return Run{}, err
	}
	run.StartedAt, err = nullableTime(startedRaw)
	if err != nil {
		return Run{}, err
	}
	run.CompletedAt, err = nullableTime(completedRaw)
	if err != nil {
		return Run{}, err
	}
	run.QueuedAt, err = database.ParseTime(queuedRaw)
	if err != nil {
		return Run{}, err
	}
	run.CreatedAt, err = database.ParseTime(createdRaw)
	if err != nil {
		return Run{}, err
	}
	run.UpdatedAt, err = database.ParseTime(updatedRaw)
	return run, err
}

func scanEvent(row rowScanner) (Event, error) {
	var event Event
	var createdRaw any
	if err := row.Scan(&event.ID, &event.RunID, &event.Level, &event.Message, &createdRaw); err != nil {
		return Event{}, err
	}
	createdAt, err := database.ParseTime(createdRaw)
	if err != nil {
		return Event{}, err
	}
	event.CreatedAt = createdAt
	return event, nil
}

type rowScanner interface{ Scan(dest ...any) error }

func nullableTime(value any) (*time.Time, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil, nil
		}
	case []byte:
		if strings.TrimSpace(string(typed)) == "" {
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
