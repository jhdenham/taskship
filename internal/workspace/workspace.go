// Package workspace works out which registered project a directory belongs
// to, using registered workspaces and read-only Git metadata.
package workspace

import (
	"context"
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
