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

const workspaceSelect = `SELECT w.id, w.project_id, p.slug, w.path, w.git_common_dir, w.created_at
FROM workspaces w JOIN projects p ON p.id = w.project_id`

// AddWorkspace registers w.Path (and w.GitCommonDir) for a project. A path
// can belong to only one project.
func (s *Store) AddWorkspace(ctx context.Context, project string, w core.Workspace) (core.Workspace, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Workspace{}, err
	}
	defer tx.Rollback()
	p, err := getProject(ctx, tx, project)
	if err != nil {
		return core.Workspace{}, err
	}
	if w, err = addWorkspace(ctx, tx, p, w); err != nil {
		return core.Workspace{}, err
	}
	return w, tx.Commit()
}

// CreateProjectWithWorkspace creates project p with workspace w in one
// transaction: either both are stored or neither is.
func (s *Store) CreateProjectWithWorkspace(ctx context.Context, p core.Project, w core.Workspace) (core.Project, core.Workspace, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Project{}, core.Workspace{}, err
	}
	defer tx.Rollback()
	if p, err = createProject(ctx, tx, p); err != nil {
		return core.Project{}, core.Workspace{}, err
	}
	if w, err = addWorkspace(ctx, tx, p, w); err != nil {
		return core.Project{}, core.Workspace{}, err
	}
	return p, w, tx.Commit()
}

func addWorkspace(ctx context.Context, tx *sql.Tx, p core.Project, w core.Workspace) (core.Workspace, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return core.Workspace{}, err
	}
	var owner string
	err = tx.QueryRowContext(ctx, "SELECT p.slug FROM workspaces w JOIN projects p ON p.id = w.project_id WHERE w.path = ?", w.Path).Scan(&owner)
	if err == nil {
		return core.Workspace{}, fmt.Errorf("%w: %s is already a workspace of project %q", core.ErrInvalid, w.Path, owner)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return core.Workspace{}, err
	}

	w.ID, w.ProjectID, w.Project, w.CreatedAt = id.String(), p.ID, p.Name, now()
	_, err = tx.ExecContext(ctx,
		"INSERT INTO workspaces (id, project_id, path, git_common_dir, created_at) VALUES (?, ?, ?, ?, ?)",
		w.ID, w.ProjectID, w.Path, w.GitCommonDir, w.CreatedAt.Format(timeLayout))
	if err != nil {
		return core.Workspace{}, err
	}
	return w, nil
}

// ListWorkspaces returns registered workspaces, optionally for one project,
// ordered by project name and path.
func (s *Store) ListWorkspaces(ctx context.Context, project string) ([]core.Workspace, error) {
	query, args := workspaceSelect, []any{}
	if project != "" {
		if _, err := getProject(ctx, s.db, project); err != nil {
			return nil, err
		}
		query += " WHERE p.slug = ?"
		args = append(args, project)
	}
	rows, err := s.db.QueryContext(ctx, query+" ORDER BY p.slug, w.path", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Workspace
	for rows.Next() {
		var w core.Workspace
		var created string
		if err := rows.Scan(&w.ID, &w.ProjectID, &w.Project, &w.Path, &w.GitCommonDir, &created); err != nil {
			return nil, err
		}
		if w.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// RemoveWorkspace unregisters the workspace with the given path.
func (s *Store) RemoveWorkspace(ctx context.Context, path string) (core.Workspace, error) {
	var w core.Workspace
	var created string
	err := s.db.QueryRowContext(ctx, workspaceSelect+" WHERE w.path = ?", path).
		Scan(&w.ID, &w.ProjectID, &w.Project, &w.Path, &w.GitCommonDir, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return w, fmt.Errorf("%w: no workspace is registered at %s", core.ErrNotFound, path)
	} else if err != nil {
		return w, err
	}
	if w.CreatedAt, err = time.Parse(timeLayout, created); err != nil {
		return w, err
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM workspaces WHERE id = ?", w.ID)
	return w, err
}
