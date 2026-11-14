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

const noteSelect = `SELECT n.id, n.project_id, n.number, n.body, COALESCE(n.run_id, ''), COALESCE(r.agent, ''), COALESCE(r.number, 0),
	n.created_at, n.updated_at FROM notes n LEFT JOIN runs r ON r.id = n.run_id`

// AddNote adds a note to the project with the given name and returns it with
// the project's next note number. runID is the agent run writing it, if any;
// it is recorded only if it is a running run of a task in the project.
func (s *Store) AddNote(ctx context.Context, project, body, runID string) (core.Note, error) {
	body, err := core.CleanNoteBody(body)
	if err != nil {
		return core.Note{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Note{}, err
	}
	defer tx.Rollback()
	p, err := getProject(ctx, tx, project)
	if err != nil {
		return core.Note{}, err
	}
	if p.ArchivedAt != nil {
		return core.Note{}, fmt.Errorf("%w: project %q is archived; restore it with: taskship project restore %s", core.ErrInvalid, p.Name, p.Name)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return core.Note{}, err
	}
	n := core.Note{ID: id.String(), ProjectID: p.ID, Body: body, CreatedAt: now()}
	n.UpdatedAt = n.CreatedAt
	if n.RunID, n.Run, err = projectRun(ctx, tx, runID, p.ID); err != nil {
		return core.Note{}, err
	}
	var run any
	if n.RunID != "" {
		run = n.RunID
	}
	if err := tx.QueryRowContext(ctx,
		"UPDATE projects SET last_note_number = last_note_number + 1 WHERE id = ? RETURNING last_note_number",
		p.ID).Scan(&n.Number); err != nil {
		return core.Note{}, err
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO notes (id, project_id, number, body, run_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		n.ID, n.ProjectID, n.Number, n.Body, run, n.CreatedAt.Format(timeLayout), n.UpdatedAt.Format(timeLayout)); err != nil {
		return core.Note{}, err
	}
	return n, tx.Commit()
}

// projectRun returns runID and its name if it is a running run of a task in
// the project, and nothing otherwise.
func projectRun(ctx context.Context, q querier, runID, projectID string) (string, *core.RunRef, error) {
	return runningRun(ctx, q, runID, `SELECT r.agent, r.number FROM runs r JOIN tasks t ON t.id = r.task_id
		WHERE r.id = ? AND t.project_id = ? AND r.state = ?`, runID, projectID, core.RunRunning)
}

// ListNotes returns a project's notes, newest first.
func (s *Store) ListNotes(ctx context.Context, projectID string) ([]core.Note, error) {
	rows, err := s.db.QueryContext(ctx, noteSelect+" WHERE n.project_id = ? ORDER BY n.number DESC", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notes := []core.Note{}
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

// GetNote returns note number n of the project with the given name.
func (s *Store) GetNote(ctx context.Context, project string, n int) (core.Note, error) {
	return getNote(ctx, s.db, project, n)
}

// UpdateNote replaces the body of note number n of the project with the
// given name.
func (s *Store) UpdateNote(ctx context.Context, project string, n int, body string) (core.Note, error) {
	body, err := core.CleanNoteBody(body)
	if err != nil {
		return core.Note{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Note{}, err
	}
	defer tx.Rollback()
	note, err := getNote(ctx, tx, project, n)
	if err != nil {
		return core.Note{}, err
	}
	if body == note.Body {
		return note, nil
	}
	note.Body, note.UpdatedAt = body, now()
	if _, err := tx.ExecContext(ctx, "UPDATE notes SET body = ?, updated_at = ? WHERE id = ?",
		note.Body, note.UpdatedAt.Format(timeLayout), note.ID); err != nil {
		return core.Note{}, err
	}
	return note, tx.Commit()
}

// DeleteNote permanently deletes note number n of the project with the given
// name and returns it.
func (s *Store) DeleteNote(ctx context.Context, project string, n int) (core.Note, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Note{}, err
	}
	defer tx.Rollback()
	note, err := getNote(ctx, tx, project, n)
	if err != nil {
		return core.Note{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM notes WHERE id = ?", note.ID); err != nil {
		return core.Note{}, err
	}
	return note, tx.Commit()
}

// TaskFromNote creates t in the project with the given name from its note
// number n, and deletes the note.
func (s *Store) TaskFromNote(ctx context.Context, project string, n int, t core.Task) (core.Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Task{}, err
	}
	defer tx.Rollback()
	note, err := getNote(ctx, tx, project, n)
	if err != nil {
		return core.Task{}, err
	}
	if t, err = createTask(ctx, tx, project, t); err != nil {
		return core.Task{}, err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM notes WHERE id = ?", note.ID); err != nil {
		return core.Task{}, err
	}
	return t, tx.Commit()
}

func getNote(ctx context.Context, q querier, project string, n int) (core.Note, error) {
	p, err := getProject(ctx, q, project)
	if err != nil {
		return core.Note{}, err
	}
	note, err := scanNote(q.QueryRowContext(ctx, noteSelect+" WHERE n.project_id = ? AND n.number = ?", p.ID, n))
	if errors.Is(err, sql.ErrNoRows) {
		return core.Note{}, fmt.Errorf("%s has no note #%d: %w", p.Name, n, core.ErrNotFound)
	}
	return note, err
}

func scanNote(row interface{ Scan(...any) error }) (core.Note, error) {
	var n core.Note
	var created, updated string
	var run core.RunRef
	if err := row.Scan(&n.ID, &n.ProjectID, &n.Number, &n.Body, &n.RunID, &run.Agent, &run.Number, &created, &updated); err != nil {
		return core.Note{}, err
	}
	if n.RunID != "" {
		n.Run = &run
	}
	var err error
	if n.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return core.Note{}, err
	}
	n.UpdatedAt, err = time.Parse(timeLayout, updated)
	return n, err
}
