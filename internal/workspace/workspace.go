// Package workspace works out which registered project a directory belongs
// to, using registered workspaces and read-only Git metadata.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jhdenham/taskship/internal/core"
)

// Inspect returns dir as an absolute, symlink-free path and, if dir is inside
// a Git repository, the repository's common Git directory. Every linked
// worktree of a repository shares the same common directory.
func Inspect(ctx context.Context, dir string) (path, gitCommonDir string, err error) {
	if path, err = filepath.Abs(dir); err != nil {
		return "", "", err
	}
	if path, err = filepath.EvalSymlinks(path); err != nil {
		return "", "", fmt.Errorf("%w: %v", core.ErrInvalid, err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("%w: %s is not a directory", core.ErrInvalid, path)
	}
	return path, commonDir(ctx, path), nil
}

// commonDir returns the common Git directory for dir, or "" if dir is not in
// a Git repository or git is unavailable. rev-parse only reads metadata; it
// runs no hooks or repository-configured commands.
func commonDir(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	d, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return ""
	}
	return d
}

// Resolve returns the registered workspace that path belongs to, given
// path's common Git directory from Inspect. Inside a Git repository, any
// workspace of the same repository matches, so linked worktrees are
// recognized without being registered. Otherwise, a workspace matches if it
// contains path. Among matches, the most specific workspace containing path
// wins; if none contains it and the matches belong to different projects,
// the result is ambiguous. ok is false if nothing matches.
func Resolve(path, gitCommonDir string, workspaces []core.Workspace) (w core.Workspace, ok bool, err error) {
	var matches []core.Workspace
	if gitCommonDir != "" {
		for _, w := range workspaces {
			if w.GitCommonDir == gitCommonDir {
				matches = append(matches, w)
			}
		}
	}
	if len(matches) == 0 {
		for _, w := range workspaces {
			if contains(w.Path, path) {
				matches = append(matches, w)
			}
		}
	}
	if len(matches) == 0 {
		return core.Workspace{}, false, nil
	}

	best := -1
	for i, w := range matches {
		if contains(w.Path, path) && (best < 0 || len(w.Path) > len(matches[best].Path)) {
			best = i
		}
	}
	if best >= 0 {
		return matches[best], true, nil
	}
	var projects []string
	for _, w := range matches {
		if !slices.Contains(projects, w.Project) {
			projects = append(projects, w.Project)
		}
	}
	if len(projects) > 1 {
		return core.Workspace{}, false, fmt.Errorf("%w: %s is a worktree of a repository registered for several projects (%s)", core.ErrAmbiguous, path, strings.Join(projects, ", "))
	}
	return matches[0], true, nil
}

// contains reports whether path is base or inside it.
func contains(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ForTask chooses the directory to run task t in, given a directory and its
// common Git directory from Inspect. If explicit is true, dir was passed by
// the user and must belong to the task's project. Otherwise dir is the
// current directory: if it belongs to the task's project, its workspace root
// is used (see root), and failing that, the project's only registered
// workspace is. With several workspaces, the choice is ambiguous; with none,
// the error says how to register one.
func ForTask(ctx context.Context, t core.Task, dir, gitCommonDir string, explicit bool, all []core.Workspace) (string, error) {
	var own []core.Workspace
	for _, w := range all {
		if w.ProjectID == t.ProjectID {
			own = append(own, w)
		}
	}
	// Match against every project first, so a directory that most
	// specifically belongs to another project isn't claimed. If that is
	// ambiguous, the task's project settles it.
	w, ok, err := Resolve(dir, gitCommonDir, all)
	if errors.Is(err, core.ErrAmbiguous) {
		w, ok, err = Resolve(dir, gitCommonDir, own)
	}
	if err != nil {
		return "", err
	}
	if ok && w.ProjectID == t.ProjectID {
		return root(ctx, dir, gitCommonDir, w), nil
	}
	if explicit {
		return "", fmt.Errorf("%w: %s is not a workspace of %s (see: taskship workspace list -p %s)", core.ErrInvalid, dir, t.Project, t.Project)
	}

	switch len(own) {
	case 0:
		return "", fmt.Errorf("%w: no workspace is registered for %s; register a checkout with: taskship workspace add %s --path PATH", core.ErrNotFound, t.Project, t.Project)
	case 1:
		if info, err := os.Stat(own[0].Path); err != nil || !info.IsDir() {
			return "", fmt.Errorf("%w: the workspace %s of %s no longer exists; remove it with: taskship workspace remove %s", core.ErrNotFound, own[0].Path, t.Project, own[0].Path)
		}
		return own[0].Path, nil
	}
	paths := make([]string, len(own))
	for i, w := range own {
		paths[i] = w.Path
	}
	return "", fmt.Errorf("%w: %s has several workspaces (%s); run from inside one or pass --workspace PATH", core.ErrAmbiguous, t.Project, strings.Join(paths, ", "))
}

// root returns the workspace root for dir, which belongs to workspace w. In a
// Git checkout of w's repository, that is the folder at w's position within
// dir's worktree: w itself in the checkout w was registered from, and the
// same subfolder (often the top level) in any other worktree, including one
// nested inside w. Otherwise it is w.
func root(ctx context.Context, dir, gitCommonDir string, w core.Workspace) string {
	if w.GitCommonDir == "" || w.GitCommonDir != gitCommonDir {
		return w.Path
	}
	top := toplevel(ctx, dir)
	if top == "" {
		return w.Path
	}
	// If w's own worktree is gone, its position is unknown: use the top level.
	if wTop := toplevel(ctx, w.Path); wTop != "" {
		if rel, err := filepath.Rel(wTop, w.Path); err == nil {
			return filepath.Join(top, rel)
		}
	}
	return top
}

// toplevel returns the symlink-free top-level directory of the worktree
// containing dir, or "" if there is none. Like commonDir, it only reads
// metadata.
func toplevel(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	d, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return ""
	}
	return d
}
