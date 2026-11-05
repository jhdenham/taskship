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

const projectColumns = "id, slug, name, task_prefix, description, archived_at, created_at, updated_at"

// CreateProject validates p, assigns its ID and timestamps, and stores it.
func (s *Store) CreateProject(ctx context.Context, p core.Project) (core.Project, error) {
	if err := core.ValidateProject(p); err != nil {
		return core.Project{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return core.Project{}, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	p.ID, p.CreatedAt, p.UpdatedAt, p.ArchivedAt = id.String(), now, now, nil

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Project{}, err
	}
	defer tx.Rollback()

	var existing string
	err = tx.QueryRowContext(ctx, "SELECT slug FROM projects WHERE slug = ?", p.Slug).Scan(&existing)
	if err == nil {
		return core.Project{}, fmt.Errorf("%w: project %q already exists", core.ErrInvalid, p.Slug)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return core.Project{}, err
	}
	err = tx.QueryRowContext(ctx, "SELECT slug FROM projects WHERE task_prefix = ?", p.TaskPrefix).Scan(&existing)
	if err == nil {
		return core.Project{}, fmt.Errorf("%w: task prefix %q is already used by project %q", core.ErrInvalid, p.TaskPrefix, existing)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return core.Project{}, err
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO projects ("+projectColumns+") VALUES (?, ?, ?, ?, ?, NULL, ?, ?)",
		p.ID, p.Slug, p.Name, p.TaskPrefix, p.Description, now.Format(timeLayout), now.Format(timeLayout))
	if err != nil {
		return core.Project{}, err
	}
	return p, tx.Commit()
}

// GetProject returns the project with the given slug.
func (s *Store) GetProject(ctx context.Context, slug string) (core.Project, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE slug = ?", slug)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return core.Project{}, fmt.Errorf("project %q: %w", slug, core.ErrNotFound)
	}
	return p, err
}

// ListProjects returns unarchived projects ordered by slug.
func (s *Store) ListProjects(ctx context.Context) ([]core.Project, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+projectColumns+" FROM projects WHERE archived_at IS NULL ORDER BY slug")
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
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.TaskPrefix, &p.Description, &archived, &created, &updated); err != nil {
		return core.Project{}, err
	}
	var err error
	if p.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return core.Project{}, err
	}
	if p.UpdatedAt, err = time.Parse(timeLayout, updated); err != nil {
		return core.Project{}, err
	}
	if archived.Valid {
		t, err := time.Parse(timeLayout, archived.String)
		if err != nil {
			return core.Project{}, err
		}
		p.ArchivedAt = &t
	}
	return p, nil
}
