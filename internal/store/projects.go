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

// timeLayout is fixed-width so stored timestamps sort lexically.
const timeLayout = "2006-01-02T15:04:05.000000Z"

// The slug column holds the project's name (core.Project.Name).
const projectColumns = "id, slug, task_prefix, description, archived_at, created_at, updated_at"

// querier is satisfied by both *sql.DB and *sql.Tx.
type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// now returns the current time at the precision timestamps are stored with.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

// CreateProject validates p, assigns its ID and timestamps, and stores it.
func (s *Store) CreateProject(ctx context.Context, p core.Project) (core.Project, error) {
	if err := core.ValidateProject(p); err != nil {
		return core.Project{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return core.Project{}, err
	}
	ts := now()
	p.ID, p.CreatedAt, p.UpdatedAt, p.ArchivedAt = id.String(), ts, ts, nil

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Project{}, err
	}
	defer tx.Rollback()

	if err := checkNameFree(ctx, tx, p.Name, ""); err != nil {
		return core.Project{}, err
	}
	var existing string
	err = tx.QueryRowContext(ctx, "SELECT slug FROM projects WHERE task_prefix = ?", p.TaskPrefix).Scan(&existing)
	if err == nil {
		return core.Project{}, fmt.Errorf("%w: task prefix %q is already used by project %q", core.ErrInvalid, p.TaskPrefix, existing)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return core.Project{}, err
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO projects ("+projectColumns+") VALUES (?, ?, ?, ?, NULL, ?, ?)",
		p.ID, p.Name, p.TaskPrefix, p.Description, ts.Format(timeLayout), ts.Format(timeLayout))
	if err != nil {
		return core.Project{}, err
	}
	return p, tx.Commit()
}

// checkNameFree reports an error if a project other than exceptID uses name.
func checkNameFree(ctx context.Context, q querier, name, exceptID string) error {
	var id string
	err := q.QueryRowContext(ctx, "SELECT id FROM projects WHERE slug = ?", name).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows), err == nil && id == exceptID:
		return nil
	case err == nil:
		return fmt.Errorf("%w: project %q already exists", core.ErrInvalid, name)
	default:
		return err
	}
}

// GetProject returns the project with the given name, archived or not.
func (s *Store) GetProject(ctx context.Context, name string) (core.Project, error) {
	return getProject(ctx, s.db, name)
}

func getProject(ctx context.Context, q querier, name string) (core.Project, error) {
	row := q.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE slug = ?", name)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Project{}, fmt.Errorf("project %q: %w", name, core.ErrNotFound)
	}
	return p, err
}

// ProjectPatch lists project fields to change; nil fields are left as is.
// The task prefix is not editable until old-ref aliasing is decided.
type ProjectPatch struct {
	Name, Description *string
}

// UpdateProject applies patch to the project with the given name.
func (s *Store) UpdateProject(ctx context.Context, name string, patch ProjectPatch) (core.Project, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Project{}, err
	}
	defer tx.Rollback()

	p, err := getProject(ctx, tx, name)
	if err != nil {
		return core.Project{}, err
	}
	if patch.Name != nil {
		p.Name = *patch.Name
	}
	if patch.Description != nil {
		p.Description = *patch.Description
	}
	if err := core.ValidateProject(p); err != nil {
		return core.Project{}, err
	}
	if err := checkNameFree(ctx, tx, p.Name, p.ID); err != nil {
		return core.Project{}, err
	}
	p.UpdatedAt = now()
	_, err = tx.ExecContext(ctx, "UPDATE projects SET slug = ?, description = ?, updated_at = ? WHERE id = ?",
		p.Name, p.Description, p.UpdatedAt.Format(timeLayout), p.ID)
	if err != nil {
		return core.Project{}, err
	}
	return p, tx.Commit()
}

// SetProjectArchived archives or restores a project. It is a no-op if the
// project is already in the requested state.
func (s *Store) SetProjectArchived(ctx context.Context, name string, archived bool) (core.Project, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Project{}, err
	}
	defer tx.Rollback()

	p, err := getProject(ctx, tx, name)
	if err != nil || (p.ArchivedAt != nil) == archived {
		return p, err
	}
	ts := now()
	var archivedAt any
	p.ArchivedAt, p.UpdatedAt = nil, ts
	if archived {
		archivedAt, p.ArchivedAt = ts.Format(timeLayout), &ts
	}
	_, err = tx.ExecContext(ctx, "UPDATE projects SET archived_at = ?, updated_at = ? WHERE id = ?",
		archivedAt, ts.Format(timeLayout), p.ID)
	if err != nil {
		return core.Project{}, err
	}
	return p, tx.Commit()
}

// ListProjects returns projects ordered by name, excluding archived ones
// unless includeArchived is set.
func (s *Store) ListProjects(ctx context.Context, includeArchived bool) ([]core.Project, error) {
	query := "SELECT " + projectColumns + " FROM projects"
	if !includeArchived {
		query += " WHERE archived_at IS NULL"
	}
	rows, err := s.db.QueryContext(ctx, query+" ORDER BY slug")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []core.Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func scanProject(row interface{ Scan(...any) error }) (core.Project, error) {
	var p core.Project
	var archived sql.NullString
	var created, updated string
	if err := row.Scan(&p.ID, &p.Name, &p.TaskPrefix, &p.Description, &archived, &created, &updated); err != nil {
		return core.Project{}, err
	}
	var err error
	if p.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return core.Project{}, err
	}
	if p.UpdatedAt, err = time.Parse(timeLayout, updated); err != nil {
		return core.Project{}, err
	}
	if p.ArchivedAt, err = parseOptionalTime(archived); err != nil {
		return core.Project{}, err
	}
	return p, nil
}

func parseOptionalTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := time.Parse(timeLayout, s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
