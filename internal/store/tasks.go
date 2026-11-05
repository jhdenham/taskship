package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhdenham/taskship/internal/core"
)

const taskSelect = `SELECT t.id, p.task_prefix, t.project_id, p.slug, t.sequence_number, t.title,
	t.description, t.status, t.priority, t.archived_at, t.created_at, t.updated_at
	FROM tasks t JOIN projects p ON p.id = t.project_id`

// CreateTask validates t and stores it in the project with the given slug,
// assigning the next sequence number in that project.
func (s *Store) CreateTask(ctx context.Context, projectSlug string, t core.Task) (core.Task, error) {
	if err := core.ValidateTask(t); err != nil {
		return core.Task{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return core.Task{}, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)

	// The transaction holds SQLite's write lock from BEGIN (_txlock=immediate),
	// so concurrent creators cannot read the same MAX(sequence_number).
	// Tasks are archived, never deleted, so numbers are never reused.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Task{}, err
	}
	defer tx.Rollback()

	var prefix string
	err = tx.QueryRowContext(ctx, "SELECT id, task_prefix FROM projects WHERE slug = ?", projectSlug).
		Scan(&t.ProjectID, &prefix)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Task{}, fmt.Errorf("project %q: %w", projectSlug, core.ErrNotFound)
	} else if err != nil {
		return core.Task{}, err
	}
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(sequence_number), 0) + 1 FROM tasks WHERE project_id = ?", t.ProjectID).
		Scan(&t.Sequence); err != nil {
		return core.Task{}, err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO tasks (id, project_id, sequence_number, title, description,
		status, priority, archived_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
		id.String(), t.ProjectID, t.Sequence, t.Title, t.Description, t.Status, t.Priority,
		now.Format(timeLayout), now.Format(timeLayout))
	if err != nil {
		return core.Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return core.Task{}, err
	}
	t.ID, t.Project, t.Ref = id.String(), projectSlug, core.FormatTaskRef(prefix, t.Sequence)
	t.CreatedAt, t.UpdatedAt, t.ArchivedAt = now, now, nil
	return t, nil
}

// GetTask returns the task with the given ref, e.g. PROJ-42.
func (s *Store) GetTask(ctx context.Context, ref string) (core.Task, error) {
	prefix, seq, err := core.ParseTaskRef(ref)
	if err != nil {
		return core.Task{}, err
	}
	row := s.db.QueryRowContext(ctx, taskSelect+" WHERE p.task_prefix = ? AND t.sequence_number = ?", prefix, seq)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Task{}, fmt.Errorf("task %s: %w", core.FormatTaskRef(prefix, seq), core.ErrNotFound)
	}
	return t, err
}

// TaskFilter selects tasks for ListTasks. Zero values match everything.
type TaskFilter struct {
	ProjectSlug string
	Statuses    []core.Status
}

// ListTasks returns unarchived tasks matching f, ordered by project slug and
// sequence number. Filtering by an unknown project returns ErrNotFound.
func (s *Store) ListTasks(ctx context.Context, f TaskFilter) ([]core.Task, error) {
	query := taskSelect + " WHERE t.archived_at IS NULL"
	var args []any
	if f.ProjectSlug != "" {
		if _, err := s.GetProject(ctx, f.ProjectSlug); err != nil {
			return nil, err
		}
		query += " AND p.slug = ?"
		args = append(args, f.ProjectSlug)
	}
	if len(f.Statuses) > 0 {
		query += " AND t.status IN (?" + strings.Repeat(", ?", len(f.Statuses)-1) + ")"
		for _, st := range f.Statuses {
			args = append(args, st)
		}
	}
	query += " ORDER BY p.slug, t.sequence_number"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []core.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func scanTask(row interface{ Scan(...any) error }) (core.Task, error) {
	var t core.Task
	var prefix, created, updated string
	var archived sql.NullString
	if err := row.Scan(&t.ID, &prefix, &t.ProjectID, &t.Project, &t.Sequence, &t.Title,
		&t.Description, &t.Status, &t.Priority, &archived, &created, &updated); err != nil {
		return core.Task{}, err
	}
	t.Ref = core.FormatTaskRef(prefix, t.Sequence)
	var err error
	if t.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return core.Task{}, err
	}
	if t.UpdatedAt, err = time.Parse(timeLayout, updated); err != nil {
		return core.Task{}, err
	}
	if archived.Valid {
		a, err := time.Parse(timeLayout, archived.String)
		if err != nil {
			return core.Task{}, err
		}
		t.ArchivedAt = &a
	}
	return t, nil
}
