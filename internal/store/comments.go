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

const commentSelect = `SELECT c.id, c.task_id, c.number, c.body, COALESCE(c.run_id, ''), COALESCE(r.agent, ''), COALESCE(r.number, 0),
	c.created_at, c.updated_at FROM comments c LEFT JOIN runs r ON r.id = c.run_id`

// AddComment adds a comment to the task with the given ref and returns it
// with the task's next comment number. runID is the agent run adding it, if
// any; it is recorded only if it is a running run of the task.
func (s *Store) AddComment(ctx context.Context, ref, body, runID string) (core.Comment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Comment{}, err
	}
	defer tx.Rollback()
	t, err := getTask(ctx, tx, ref)
	if err != nil {
		return core.Comment{}, err
	}
	c, err := addComment(ctx, tx, t, body, runID)
	if err != nil {
		return core.Comment{}, err
	}
	return c, tx.Commit()
}

func addComment(ctx context.Context, tx *sql.Tx, t core.Task, body, runID string) (core.Comment, error) {
	body, err := core.CleanCommentBody(body)
	if err != nil {
		return core.Comment{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return core.Comment{}, err
	}
	c := core.Comment{ID: id.String(), TaskID: t.ID, Body: body, CreatedAt: now()}
	c.UpdatedAt = c.CreatedAt
	if c.RunID, c.Run, err = activeRun(ctx, tx, runID, t.ID); err != nil {
		return core.Comment{}, err
	}
	var run any
	if c.RunID != "" {
		run = c.RunID
	}
	if err := tx.QueryRowContext(ctx,
		"UPDATE tasks SET last_comment_number = last_comment_number + 1 WHERE id = ? RETURNING last_comment_number",
		t.ID).Scan(&c.Number); err != nil {
		return core.Comment{}, err
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO comments (id, task_id, number, body, run_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		c.ID, c.TaskID, c.Number, c.Body, run, c.CreatedAt.Format(timeLayout), c.UpdatedAt.Format(timeLayout)); err != nil {
		return core.Comment{}, err
	}
	return c, nil
}

// ListComments returns a task's comments, oldest first.
func (s *Store) ListComments(ctx context.Context, taskID string) ([]core.Comment, error) {
	rows, err := s.db.QueryContext(ctx, commentSelect+" WHERE c.task_id = ? ORDER BY c.number", taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	comments := []core.Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// GetComment returns the task with the given ref and its comment number n.
func (s *Store) GetComment(ctx context.Context, ref string, n int) (core.Task, core.Comment, error) {
	return getComment(ctx, s.db, ref, n)
}

// UpdateComment replaces the body of comment number n of the task with the
// given ref.
func (s *Store) UpdateComment(ctx context.Context, ref string, n int, body string) (core.Comment, error) {
	body, err := core.CleanCommentBody(body)
	if err != nil {
		return core.Comment{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Comment{}, err
	}
	defer tx.Rollback()
	_, c, err := getComment(ctx, tx, ref, n)
	if err != nil {
		return core.Comment{}, err
	}
	if body == c.Body {
		return c, nil
	}
	c.Body, c.UpdatedAt = body, now()
	if _, err := tx.ExecContext(ctx, "UPDATE comments SET body = ?, updated_at = ? WHERE id = ?",
		c.Body, c.UpdatedAt.Format(timeLayout), c.ID); err != nil {
		return core.Comment{}, err
	}
	return c, tx.Commit()
}

// DeleteComment permanently deletes comment number n of the task with the
// given ref and returns it.
func (s *Store) DeleteComment(ctx context.Context, ref string, n int) (core.Comment, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Comment{}, err
	}
	defer tx.Rollback()
	_, c, err := getComment(ctx, tx, ref, n)
	if err != nil {
		return core.Comment{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM comments WHERE id = ?", c.ID); err != nil {
		return core.Comment{}, err
	}
	return c, tx.Commit()
}

func getComment(ctx context.Context, q querier, ref string, n int) (core.Task, core.Comment, error) {
	t, err := getTask(ctx, q, ref)
	if err != nil {
		return core.Task{}, core.Comment{}, err
	}
	c, err := scanComment(q.QueryRowContext(ctx, commentSelect+" WHERE c.task_id = ? AND c.number = ?", t.ID, n))
	if errors.Is(err, sql.ErrNoRows) {
		return core.Task{}, core.Comment{}, fmt.Errorf("%s has no comment #%d: %w", t.Ref, n, core.ErrNotFound)
	}
	return t, c, err
}

func scanComment(row interface{ Scan(...any) error }) (core.Comment, error) {
	var c core.Comment
	var created, updated string
	var run core.RunRef
	if err := row.Scan(&c.ID, &c.TaskID, &c.Number, &c.Body, &c.RunID, &run.Agent, &run.Number, &created, &updated); err != nil {
		return core.Comment{}, err
	}
	if c.RunID != "" {
		c.Run = &run
	}
	var err error
	if c.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return core.Comment{}, err
	}
	c.UpdatedAt, err = time.Parse(timeLayout, updated)
	return c, err
}
