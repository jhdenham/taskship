package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jhdenham/taskship/internal/core"
)

const runSelect = `SELECT id, number, task_id, task_ref, agent, mode, machine, workspace_path, session_id,
state, exit_code, started_at, ended_at FROM runs`

// CreateRun records r as starting now and returns it with its ID, number and
// start time set. Record a run before starting the agent, so failed launches
// are recorded too.
func (s *Store) CreateRun(ctx context.Context, r core.Run) (core.Run, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return core.Run{}, err
	}
	r.ID, r.State, r.StartedAt = id.String(), core.RunStarting, now()
	if r.Mode == "" {
		r.Mode = core.RunModeRun
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Run{}, err
	}
	defer tx.Rollback()
	// Runs are never deleted, so MAX + 1 never reuses a number.
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(number), 0) + 1 FROM runs WHERE task_id = ?", r.TaskID).
		Scan(&r.Number); err != nil {
		return core.Run{}, err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO runs (id, number, task_id, task_ref, agent, mode, machine, workspace_path, session_id, state, started_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Number, r.TaskID, r.TaskRef, r.Agent, r.Mode, r.Machine, r.WorkspacePath, r.SessionID, r.State, r.StartedAt.Format(timeLayout))
	if err != nil {
		return core.Run{}, err
	}
	return r, tx.Commit()
}

// StartRun marks a run as running once its agent process has started.
func (s *Store) StartRun(ctx context.Context, id string) error {
	return s.updateRun(ctx, "UPDATE runs SET state = ? WHERE id = ?", core.RunRunning, id)
}

// FinishRun marks a run as ended with the agent's exit code, or as failed
// when exitCode is nil, and sets its end time.
func (s *Store) FinishRun(ctx context.Context, id string, exitCode *int) error {
	state := core.RunEnded
	if exitCode == nil {
		state = core.RunFailed
	}
	return s.updateRun(ctx, "UPDATE runs SET state = ?, exit_code = ?, ended_at = ? WHERE id = ?",
		state, exitCode, now().Format(timeLayout), id)
}

func (s *Store) updateRun(ctx context.Context, query string, args ...any) error {
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("run %s: %w", args[len(args)-1], core.ErrNotFound)
	}
	return nil
}

// activeRun returns runID and its display name if it is a running run of the
// task, and "" and nil if it isn't (or is empty): a stale or foreign
// TASKSHIP_RUN_ID is ignored.
func activeRun(ctx context.Context, q querier, runID, taskID string) (string, *core.RunRef, error) {
	if runID == "" {
		return "", nil, nil
	}
	var ref core.RunRef
	err := q.QueryRowContext(ctx, "SELECT agent, number FROM runs WHERE id = ? AND task_id = ? AND state = ?",
		runID, taskID, core.RunRunning).Scan(&ref.Agent, &ref.Number)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return runID, &ref, nil
}

// ListRuns returns a task's runs, newest first.
func (s *Store) ListRuns(ctx context.Context, taskID string) ([]core.Run, error) {
	rows, err := s.db.QueryContext(ctx, runSelect+" WHERE task_id = ? ORDER BY started_at DESC, id DESC", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []core.Run{}
	for rows.Next() {
		var r core.Run
		var exit sql.NullInt64
		var started string
		var ended sql.NullString
		if err := rows.Scan(&r.ID, &r.Number, &r.TaskID, &r.TaskRef, &r.Agent, &r.Mode, &r.Machine, &r.WorkspacePath,
			&r.SessionID, &r.State, &exit, &started, &ended); err != nil {
			return nil, err
		}
		if exit.Valid {
			code := int(exit.Int64)
			r.ExitCode = &code
		}
		if r.StartedAt, err = time.Parse(timeLayout, started); err != nil {
			return nil, err
		}
		if r.EndedAt, err = parseOptionalTime(ended); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
