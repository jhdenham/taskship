package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jhdenham/taskship/internal/core"
)

const taskSelect = `SELECT t.id, p.task_prefix, t.project_id, p.slug, t.sequence_number, t.title,
	t.description, t.status, t.priority, t.archived_at, t.created_at, t.updated_at,
	(SELECT group_concat(label, ',') FROM task_labels WHERE task_id = t.id)
	FROM tasks t JOIN projects p ON p.id = t.project_id`

// CreateTask validates t and stores it in the project with the given name,
// assigning the next sequence number in that project.
func (s *Store) CreateTask(ctx context.Context, project string, t core.Task) (core.Task, error) {
	if err := core.ValidateTask(t); err != nil {
		return core.Task{}, err
	}
	labels, err := core.NormalizeLabels(t.Labels)
	if err != nil {
		return core.Task{}, err
	}
	t.Labels = labels
	id, err := uuid.NewV7()
	if err != nil {
		return core.Task{}, err
	}
	ts := now()

	// The transaction holds SQLite's write lock from BEGIN (_txlock=immediate),
	// so concurrent creators cannot read the same MAX(sequence_number).
	// Tasks are archived, never deleted, so numbers are never reused.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Task{}, err
	}
	defer tx.Rollback()

	p, err := getProject(ctx, tx, project)
	if err != nil {
		return core.Task{}, err
	}
	if p.ArchivedAt != nil {
		return core.Task{}, fmt.Errorf("%w: project %q is archived; restore it with: taskship project restore %s", core.ErrInvalid, p.Name, p.Name)
	}
	t.ProjectID = p.ID
	if err := tx.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(sequence_number), 0) + 1 FROM tasks WHERE project_id = ?", t.ProjectID).
		Scan(&t.Sequence); err != nil {
		return core.Task{}, err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO tasks (id, project_id, sequence_number, title, description,
		status, priority, archived_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
		id.String(), t.ProjectID, t.Sequence, t.Title, t.Description, t.Status, t.Priority,
		ts.Format(timeLayout), ts.Format(timeLayout))
	if err != nil {
		return core.Task{}, err
	}
	if err := setLabels(ctx, tx, id.String(), t.Labels); err != nil {
		return core.Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return core.Task{}, err
	}
	t.ID, t.Project, t.Ref = id.String(), p.Name, core.FormatTaskRef(p.TaskPrefix, t.Sequence)
	t.CreatedAt, t.UpdatedAt, t.ArchivedAt = ts, ts, nil
	return t, nil
}

// GetTask returns the task with the given ref, e.g. PROJ-42, archived or not.
func (s *Store) GetTask(ctx context.Context, ref string) (core.Task, error) {
	return getTask(ctx, s.db, ref)
}

func getTask(ctx context.Context, q querier, ref string) (core.Task, error) {
	prefix, seq, err := core.ParseTaskRef(ref)
	if err != nil {
		return core.Task{}, err
	}
	row := q.QueryRowContext(ctx, taskSelect+" WHERE p.task_prefix = ? AND t.sequence_number = ?", prefix, seq)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Task{}, fmt.Errorf("task %s: %w", core.FormatTaskRef(prefix, seq), core.ErrNotFound)
	}
	return t, err
}

// TaskPatch lists task fields to change; nil fields are left as is.
type TaskPatch struct {
	Title, Description *string
	Status             *core.Status
	Priority           *core.Priority
	Labels             *[]string // replaces the task's labels
}

// UpdateTask applies patch to the task with the given ref.
func (s *Store) UpdateTask(ctx context.Context, ref string, patch TaskPatch) (core.Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Task{}, err
	}
	defer tx.Rollback()

	t, err := getTask(ctx, tx, ref)
	if err != nil {
		return core.Task{}, err
	}
	if patch.Title != nil {
		t.Title = *patch.Title
	}
	if patch.Description != nil {
		t.Description = *patch.Description
	}
	if patch.Status != nil {
		t.Status = *patch.Status
	}
	if patch.Priority != nil {
		t.Priority = *patch.Priority
	}
	if err := core.ValidateTask(t); err != nil {
		return core.Task{}, err
	}
	if patch.Labels != nil {
		if t.Labels, err = core.NormalizeLabels(*patch.Labels); err != nil {
			return core.Task{}, err
		}
		if err := setLabels(ctx, tx, t.ID, t.Labels); err != nil {
			return core.Task{}, err
		}
	}
	t.UpdatedAt = now()
	_, err = tx.ExecContext(ctx,
		"UPDATE tasks SET title = ?, description = ?, status = ?, priority = ?, updated_at = ? WHERE id = ?",
		t.Title, t.Description, t.Status, t.Priority, t.UpdatedAt.Format(timeLayout), t.ID)
	if err != nil {
		return core.Task{}, err
	}
	return t, tx.Commit()
}

// setLabels replaces a task's labels with labels, which must be normalized.
func setLabels(ctx context.Context, tx *sql.Tx, taskID string, labels []string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM task_labels WHERE task_id = ?", taskID); err != nil {
		return err
	}
	for _, l := range labels {
		if _, err := tx.ExecContext(ctx, "INSERT INTO task_labels (task_id, label) VALUES (?, ?)", taskID, l); err != nil {
			return err
		}
	}
	return nil
}

// SetTaskArchived archives or restores a task. It is a no-op if the task is
// already in the requested state.
func (s *Store) SetTaskArchived(ctx context.Context, ref string, archived bool) (core.Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Task{}, err
	}
	defer tx.Rollback()

	t, err := getTask(ctx, tx, ref)
	if err != nil || (t.ArchivedAt != nil) == archived {
		return t, err
	}
	ts := now()
	var archivedAt any
	t.ArchivedAt, t.UpdatedAt = nil, ts
	if archived {
		archivedAt, t.ArchivedAt = ts.Format(timeLayout), &ts
	}
	_, err = tx.ExecContext(ctx, "UPDATE tasks SET archived_at = ?, updated_at = ? WHERE id = ?",
		archivedAt, ts.Format(timeLayout), t.ID)
	if err != nil {
		return core.Task{}, err
	}
	return t, tx.Commit()
}

// TaskFilter selects tasks for ListTasks. Zero values match everything
// except archived tasks and tasks in archived projects.
type TaskFilter struct {
	Project         string
	Statuses        []core.Status
	Labels          []string // tasks with all of these labels
	IncludeArchived bool
}

// ListTasks returns tasks matching f, ordered by project name and sequence
// number. Filtering by an unknown project returns ErrNotFound.
func (s *Store) ListTasks(ctx context.Context, f TaskFilter) ([]core.Task, error) {
	var where []string
	var args []any
	if !f.IncludeArchived {
		where = append(where, "t.archived_at IS NULL", "p.archived_at IS NULL")
	}
	if f.Project != "" {
		if _, err := s.GetProject(ctx, f.Project); err != nil {
			return nil, err
		}
		where = append(where, "p.slug = ?")
		args = append(args, f.Project)
	}
	if len(f.Statuses) > 0 {
		where = append(where, "t.status IN (?"+strings.Repeat(", ?", len(f.Statuses)-1)+")")
		for _, st := range f.Statuses {
			args = append(args, st)
		}
	}
	labels, err := core.NormalizeLabels(f.Labels)
	if err != nil {
		return nil, err
	}
	for _, l := range labels {
		where = append(where, "EXISTS (SELECT 1 FROM task_labels l WHERE l.task_id = t.id AND l.label = ?)")
		args = append(args, l)
	}
	query := taskSelect
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
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
	var archived, labels sql.NullString
	if err := row.Scan(&t.ID, &prefix, &t.ProjectID, &t.Project, &t.Sequence, &t.Title,
		&t.Description, &t.Status, &t.Priority, &archived, &created, &updated, &labels); err != nil {
		return core.Task{}, err
	}
	t.Labels = []string{}
	if labels.Valid {
		t.Labels = strings.Split(labels.String, ",")
		slices.Sort(t.Labels)
	}
	t.Ref = core.FormatTaskRef(prefix, t.Sequence)
	var err error
	if t.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return core.Task{}, err
	}
	if t.UpdatedAt, err = time.Parse(timeLayout, updated); err != nil {
		return core.Task{}, err
	}
	if t.ArchivedAt, err = parseOptionalTime(archived); err != nil {
		return core.Task{}, err
	}
	return t, nil
}
