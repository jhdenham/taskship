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

// Location is a folder as Inspect found it.
type Location struct {
	// Path is absolute and symlink-free.
	Path string
	// GitCommonDir is the common Git directory of the repository containing
	// Path, shared by all its linked worktrees; "" outside a repository.
	GitCommonDir string
	// MainPath is, for a folder in a linked worktree, the same folder in the
	// repository's main checkout, so worktrees match workspaces registered
	// there by position; "" otherwise.
	MainPath string
}

// Inspect returns the Location of dir.
func Inspect(ctx context.Context, dir string) (Location, error) {
	path, err := filepath.Abs(dir)
	if err != nil {
		return Location{}, err
	}
	if path, err = filepath.EvalSymlinks(path); err != nil {
		return Location{}, fmt.Errorf("%w: %v", core.ErrInvalid, err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return Location{}, fmt.Errorf("%w: %s is not a directory", core.ErrInvalid, path)
	}
	loc := Location{Path: path, GitCommonDir: commonDir(ctx, path)}
	if loc.GitCommonDir != "" {
		top := toplevel(ctx, path)
		main, _ := MainCheckout(ctx, path)
		if top != "" && main != "" && top != main {
			if rel, err := filepath.Rel(top, path); err == nil {
				loc.MainPath = filepath.Join(main, rel)
			}
		}
	}
	return loc, nil
}

// commonDir returns the common Git directory for dir, or "" if dir is not in
// a Git repository or git is unavailable. rev-parse only reads metadata; it
// runs no hooks or repository-configured commands.
func commonDir(ctx context.Context, dir string) string {
	out, ok := gitOutput(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !ok {
		return ""
	}
	d, err := filepath.EvalSymlinks(out)
	if err != nil {
		return ""
	}
	return d
}

// Resolve returns the registered workspace that loc belongs to. Inside a
// Git repository, only workspaces of the same repository match, so linked
// worktrees are recognized without being registered and a plain folder
// never claims a repository inside it. Outside one, a workspace matches if
// it contains loc. Among matches, the most specific workspace containing loc
// (or, in a linked worktree, its position in the main checkout) wins; if
// none contains it and the matches belong to different projects, the result
// is ambiguous. ok is false if nothing matches.
func Resolve(loc Location, workspaces []core.Workspace) (w core.Workspace, ok bool, err error) {
	holds := func(w core.Workspace) bool {
		return contains(w.Path, loc.Path) || loc.MainPath != "" && contains(w.Path, loc.MainPath)
	}
	var matches []core.Workspace
	for _, w := range workspaces {
		if loc.GitCommonDir != "" && w.GitCommonDir == loc.GitCommonDir ||
			loc.GitCommonDir == "" && holds(w) {
			matches = append(matches, w)
		}
	}
	if len(matches) == 0 {
		return core.Workspace{}, false, nil
	}

	best := -1
	for i, w := range matches {
		if holds(w) && (best < 0 || len(w.Path) > len(matches[best].Path)) {
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
		return core.Workspace{}, false, fmt.Errorf("%w: %s is a worktree of a repository registered for several projects (%s)", core.ErrAmbiguous, loc.Path, strings.Join(projects, ", "))
	}
	return matches[0], true, nil
}

// contains reports whether path is base or inside it.
func contains(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ForTask chooses the directory to run task t in, given the Location of a
// directory. If explicit is true, the directory was passed by the user and
// must belong to the task's project. Otherwise it is the current directory.
// If it belongs to the task's project, or is anywhere in a repository the
// project is registered in, the project's folder in that checkout is used
// (see root). Failing that, the project's only registered workspace is,
// unless it is a bare repository. With several workspaces, the choice is
// ambiguous; with none, the error says how to register one.
func ForTask(ctx context.Context, t core.Task, loc Location, explicit bool, all []core.Workspace) (string, error) {
	var own []core.Workspace
	for _, w := range all {
		if w.ProjectID == t.ProjectID {
			own = append(own, w)
		}
	}
	// Match against every project first, so a folder that belongs to
	// another project isn't claimed. In the same repository as the task's
	// project, though, the project's own workspaces settle it.
	w, ok, err := Resolve(loc, all)
	if err != nil && !errors.Is(err, core.ErrAmbiguous) {
		return "", err
	}
	if !(ok && w.ProjectID == t.ProjectID) && loc.GitCommonDir != "" {
		if w, ok, err = Resolve(loc, own); err != nil {
			return "", err
		}
	}
	if ok && w.ProjectID == t.ProjectID {
		return root(ctx, loc.Path, loc.GitCommonDir, w), nil
	}
	if explicit {
		return "", fmt.Errorf("%w: %s is not a workspace of %s (see: taskship workspace list -p %s)", core.ErrInvalid, loc.Path, t.Project, t.Project)
	}

	switch len(own) {
	case 0:
		return "", fmt.Errorf("%w: no workspace is registered for %s; register a checkout with: taskship workspace add %s --path PATH", core.ErrNotFound, t.Project, t.Project)
	case 1:
		if info, err := os.Stat(own[0].Path); err != nil || !info.IsDir() {
			return "", fmt.Errorf("%w: the workspace %s of %s no longer exists; remove it with: taskship workspace remove %s", core.ErrNotFound, own[0].Path, t.Project, own[0].Path)
		}
		if out, _ := gitOutput(ctx, own[0].Path, "rev-parse", "--is-bare-repository"); out == "true" {
			return "", fmt.Errorf("%w: %s is a bare repository, with no files to work on; run this from one of its worktrees, or pass --workspace PATH", core.ErrInvalid, own[0].Path)
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
	out, ok := gitOutput(ctx, dir, "rev-parse", "--show-toplevel")
	if !ok {
		return ""
	}
	d, err := filepath.EvalSymlinks(out)
	if err != nil {
		return ""
	}
	return d
}

// MainCheckout returns the symlink-free path of the main worktree of the Git
// repository containing dir: the checkout that linked worktrees were added
// from, or the repository itself if it is bare. It returns "" if dir is not
// in a Git repository.
func MainCheckout(ctx context.Context, dir string) (path string, bare bool) {
	out, ok := gitOutput(ctx, dir, "worktree", "list", "--porcelain")
	if !ok {
		return "", false
	}
	// The main worktree is listed first, as "worktree PATH" followed by its
	// attributes, one of which is "bare" for a bare repository.
	first, _, _ := strings.Cut(out, "\n\n")
	lines := strings.Split(first, "\n")
	p, found := strings.CutPrefix(lines[0], "worktree ")
	if !found {
		return "", false
	}
	path, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", false
	}
	return path, slices.Contains(lines[1:], "bare")
}

// RemoteName returns the repository name in the URL of the "origin" remote
// of the Git repository containing dir, or of its only remote if there is no
// origin (for example "myproject" for git@github.com:me/myproject.git). It
// returns "" if there is no such remote. Reading remotes runs no
// repository-configured commands.
func RemoteName(ctx context.Context, dir string) string {
	out, ok := gitOutput(ctx, dir, "remote")
	if !ok {
		return ""
	}
	remotes := strings.Fields(out)
	remote := "origin"
	if !slices.Contains(remotes, remote) {
		if len(remotes) != 1 {
			return ""
		}
		remote = remotes[0]
	}
	url, ok := gitOutput(ctx, dir, "remote", "get-url", remote)
	if !ok {
		return ""
	}
	return repoNameFromURL(url)
}

// repoNameFromURL returns the last path element of a Git remote URL, without
// a trailing slash or ".git".
func repoNameFromURL(url string) string {
	url = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git")
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	return url
}

// gitOutput runs a read-only git command in dir and returns its trimmed
// output; ok is false if git fails or is not installed.
func gitOutput(ctx context.Context, dir string, args ...string) (out string, ok bool) {
	b, err := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}
