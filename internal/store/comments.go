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

const commentSelect = "SELECT id, task_id, number, body, COALESCE(run_id, ''), created_at, updated_at FROM comments"

// AddComment adds a comment to the task with the given ref and returns it
// with the task's next comment number. runID is the agent run adding it, if
// any; it is recorded only if it is a running run of the task.
func (s *Store) AddComment(ctx context.Context, ref, body, runID string) (core.Comment, error) {
	body, err := core.CleanCommentBody(body)
	if err != nil {
		return core.Comment{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return core.Comment{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Comment{}, err
	}
	defer tx.Rollback()
	t, err := getTask(ctx, tx, ref)
	if err != nil {
		return core.Comment{}, err
	}
	c := core.Comment{ID: id.String(), TaskID: t.ID, Body: body, CreatedAt: now()}
	c.UpdatedAt = c.CreatedAt
	if c.RunID, err = activeRun(ctx, tx, runID, t.ID); err != nil {
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
	return c, tx.Commit()
}

// ListComments returns a task's comments, oldest first.
func (s *Store) ListComments(ctx context.Context, taskID string) ([]core.Comment, error) {
	rows, err := s.db.QueryContext(ctx, commentSelect+" WHERE task_id = ? ORDER BY number", taskID)
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

// GetComment returns comment number n of the task with the given ref.
func (s *Store) GetComment(ctx context.Context, ref string, n int) (core.Comment, error) {
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
	c, err := getComment(ctx, tx, ref, n)
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
	c, err := getComment(ctx, tx, ref, n)
	if err != nil {
		return core.Comment{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM comments WHERE id = ?", c.ID); err != nil {
		return core.Comment{}, err
	}
	return c, tx.Commit()
}

func getComment(ctx context.Context, q querier, ref string, n int) (core.Comment, error) {
	t, err := getTask(ctx, q, ref)
	if err != nil {
		return core.Comment{}, err
	}
	c, err := scanComment(q.QueryRowContext(ctx, commentSelect+" WHERE task_id = ? AND number = ?", t.ID, n))
	if errors.Is(err, sql.ErrNoRows) {
		return core.Comment{}, fmt.Errorf("%s has no comment #%d: %w", t.Ref, n, core.ErrNotFound)
	}
	return c, err
}

func scanComment(row interface{ Scan(...any) error }) (core.Comment, error) {
	var c core.Comment
	var created, updated string
	if err := row.Scan(&c.ID, &c.TaskID, &c.Number, &c.Body, &c.RunID, &created, &updated); err != nil {
		return core.Comment{}, err
	}
	var err error
	if c.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return core.Comment{}, err
	}
	c.UpdatedAt, err = time.Parse(timeLayout, updated)
	return c, err
}
