package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
	"github.com/jhdenham/taskship/internal/workspace"
)

func newInitCmd() *cobra.Command {
	var r initRun
	cmd := &cobra.Command{
		Use:   "init [PATH] [--name NAME] [--prefix PREFIX] [-y]",
		Short: "Set up a Taskship project or link to an existing one",
		Long: `Set up a Taskship project for the repository or folder at PATH (default:
the current folder), or link it to an existing project of the same name.
Taskship doesn't write anything to the repository or folder.

In a Git repository, its main checkout is set up, and every worktree of it is
recognized. In a subfolder of a repository, init asks whether to set up the
whole repository or only that folder, and in a folder that is already part of
a project, whether to set up a separate project for it. In a folder that
contains repositories, it asks whether to set up the folder as one project
or which repository in it to set up.

init asks for the project name and task prefix, suggesting defaults from the
repository's remote or the folder's name. --name and --prefix answer those
questions, and -y accepts the defaults, so init can run without a terminal.`,
		Example: "  taskship init\n  taskship init ~/code/myproject -y\n  taskship init --name myproject --prefix PROJ",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
				return usageError{err}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, explicit := ".", len(args) == 1
			if explicit {
				dir = args[0]
			}
			r.cmd, r.p = cmd, newPrompter(cmd)
			r.interactive = inputIsTerminal(cmd.InOrStdin())
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			r.s = s
			return r.run(cmd.Context(), dir, explicit)
		},
	}
	cmd.Flags().StringVar(&r.name, "name", "", "project name (default: from the remote or folder name)")
	cmd.Flags().StringVar(&r.prefix, "prefix", "", "task prefix, e.g. PROJ (default: from the project name)")
	cmd.Flags().BoolVarP(&r.yes, "yes", "y", false, "accept the default answers instead of asking")
	return cmd
}

// initRun is one run of taskship init.
type initRun struct {
	cmd          *cobra.Command
	s            *store.Store
	p            *prompter
	name, prefix string // flags
	yes          bool
	interactive  bool // stdin is a terminal
	asked        bool // a question was asked
}

// initTarget is the folder init sets up.
type initTarget struct {
	path, gitCommonDir string
	subfolder          bool // a folder inside a repository, not its checkout
}

func (t initTarget) kind() string {
	if t.gitCommonDir != "" && !t.subfolder {
		return "repository"
	}
	return "folder"
}

var errNeedTerminal = usageError{errors.New("init needs a terminal to ask questions; pass -y to accept the defaults, or --name and --prefix")}

func (r *initRun) run(ctx context.Context, dir string, explicit bool) error {
	ws, err := r.s.ListWorkspaces(ctx, "")
	if err != nil {
		return err
	}
	t, ok, err := r.target(ctx, dir, explicit, ws)
	if err != nil || !ok {
		return err
	}

	out := r.cmd.OutOrStdout()
	fmt.Fprintf(out, "Setting up Taskship for %s\n", displayPath(t.path))
	home, err := homeDir()
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Taskship doesn't write anything to this %s. Everything lives in %s.\n\n", t.kind(), displayPath(home))

	def := defaultName(ctx, t)
	for {
		name, err := r.ask("Project name", r.name, def, "name", checkName)
		if err != nil {
			return err
		}
		p, err := r.s.GetProject(ctx, name)
		if errors.Is(err, core.ErrNotFound) {
			return r.create(ctx, t, name)
		} else if err != nil {
			return err
		}
		link, err := r.confirm(fmt.Sprintf("%s already exists (prefix %s). Link this folder to it?", p.Name, p.TaskPrefix), true)
		if err != nil {
			return err
		}
		if link {
			return r.link(ctx, t, p)
		}
		if r.name != "" {
			return fmt.Errorf("nothing changed; pass a different --name")
		}
		def = ""
	}
}

// target works out the folder to set up from dir: a repository's main
// checkout, or a folder. It asks when dir is a subfolder of a repository,
// already part of a project, or a folder of repositories. ok is false if
// there is nothing to set up, and init has said why.
func (r *initRun) target(ctx context.Context, dir string, explicit bool, ws []core.Workspace) (t initTarget, ok bool, err error) {
	loc, err := workspace.Inspect(ctx, dir)
	if err != nil {
		return initTarget{}, false, err
	}
	// pos is the folder's position in the main checkout. A bare repository
	// has no folders of its own, so its worktrees count as a whole.
	pos, main := loc.Path, ""
	if loc.MainPath != "" {
		pos = loc.MainPath
	}
	if loc.GitCommonDir != "" {
		var bare bool
		if main, bare = workspace.MainCheckout(ctx, loc.Path); main == "" {
			main = pos
		} else if bare {
			pos = main
		}
	}

	if w, set, err := workspace.Resolve(loc, ws); err != nil {
		return initTarget{}, false, err
	} else if set {
		if w.Path == pos {
			return initTarget{}, false, r.alreadySetUp(ctx, initTarget{w.Path, loc.GitCommonDir, w.Path != main}, w.Project)
		}
		return r.separate(ctx, loc, pos, w)
	}
	if loc.GitCommonDir != "" {
		if pos == main {
			return initTarget{path: main, gitCommonDir: loc.GitCommonDir}, true, checkInitPath(main)
		}
		return r.wholeOrFolder(loc, pos, main, explicit)
	}
	if err := checkInitPath(loc.Path); err != nil {
		return initTarget{}, false, err
	}
	if !explicit {
		if repos, total := childRepos(ctx, loc.Path, ws); total > 0 {
			return r.inFolderOfRepos(ctx, loc.Path, repos, total, ws)
		}
	}
	return initTarget{path: loc.Path}, true, nil
}

// separate offers a separate project for a folder inside project w's folder.
func (r *initRun) separate(ctx context.Context, loc workspace.Location, pos string, w core.Workspace) (initTarget, bool, error) {
	p, err := r.s.GetProject(ctx, w.Project)
	if err != nil {
		return initTarget{}, false, err
	}
	out := r.cmd.OutOrStdout()
	fmt.Fprintf(out, "This folder is already part of a project (name: %s, prefix: %s).\n", p.Name, p.TaskPrefix)
	yes, err := r.confirm("Set up a separate project for this folder?", false)
	if err != nil {
		return initTarget{}, false, err
	}
	if !yes {
		fmt.Fprintln(out, `Run "taskship tasks" to see its tasks.`)
		return initTarget{}, false, nil
	}
	return initTarget{pos, loc.GitCommonDir, loc.GitCommonDir != ""}, true, checkInitPath(pos)
}

// wholeOrFolder asks whether to set up a whole repository or only the
// subfolder of it at pos.
func (r *initRun) wholeOrFolder(loc workspace.Location, pos, main string, explicit bool) (initTarget, bool, error) {
	whole := initTarget{path: main, gitCommonDir: loc.GitCommonDir}
	if r.yes {
		return whole, true, checkInitPath(main)
	}
	if !r.interactive {
		return initTarget{}, false, errNeedTerminal
	}
	w := r.p.out
	which := "The current folder"
	if explicit {
		which = "The folder"
	}
	fmt.Fprintf(w, "%s (%s) is part of a repository (%s).\n", which, displayPath(loc.Path), displayPath(main))
	fmt.Fprintln(w, "Where do you want to set up a Taskship project?")
	fmt.Fprintf(w, "  1) The whole repository (%s)\n", displayPath(main))
	fmt.Fprintf(w, "  2) Only this folder (%s)\n", displayPath(pos))
	choice, err := r.ask("Enter your choice", "", "1", "", numberIn(2))
	if err != nil {
		return initTarget{}, false, err
	}
	if choice == "1" {
		return whole, true, checkInitPath(main)
	}
	return initTarget{pos, loc.GitCommonDir, true}, true, checkInitPath(pos)
}

// checkInitPath refuses the home folder and the filesystem root, which would
// claim everything below them.
func checkInitPath(path string) error {
	home, _ := os.UserHomeDir()
	where := ""
	switch path {
	case home:
		where = "your home folder"
	case filepath.Dir(path):
		where = "the filesystem root"
	default:
		return nil
	}
	return usageError{fmt.Errorf(`run "taskship init" inside a project's folder, not %s, or pass one: taskship init ~/code/myproject`, where)}
}

// childRepo is a Git repository directly inside a folder.
type childRepo struct {
	path    string
	changed time.Time
}

// childRepos returns the repositories directly inside dir that aren't set
// up yet, most recently changed first, and how many repositories there are
// in all. A repository's last change is the modification time of its .git
// entry, which commits, checkouts and fetches update.
func childRepos(ctx context.Context, dir string, ws []core.Workspace) (repos []childRepo, total int) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info, err := os.Stat(filepath.Join(path, ".git"))
		if err != nil {
			continue
		}
		total++
		loc, err := workspace.Inspect(ctx, path)
		if err != nil || loc.GitCommonDir == "" {
			continue
		}
		if _, ok, err := workspace.Resolve(loc, ws); ok || err != nil {
			continue
		}
		repos = append(repos, childRepo{loc.Path, info.ModTime()})
	}
	slices.SortStableFunc(repos, func(a, b childRepo) int { return b.changed.Compare(a.changed) })
	return repos, total
}

// maxChoices is how many repositories are listed to choose from.
const maxChoices = 8

// inFolderOfRepos asks whether to set up dir, a folder of repositories, as
// one project, and if not, which repository in it to set up.
func (r *initRun) inFolderOfRepos(ctx context.Context, dir string, repos []childRepo, total int, ws []core.Workspace) (initTarget, bool, error) {
	if r.yes || !r.interactive {
		return initTarget{}, false, usageError{fmt.Errorf("%s contains Git repositories; pass the one to set up, for example: taskship init %s", displayPath(dir), displayPath(filepath.Join(dir, "myproject")))}
	}
	w := r.p.out
	if total == 1 {
		fmt.Fprintf(w, "Looks like %s contains a project repository.\n", displayPath(dir))
	} else {
		fmt.Fprintf(w, "Looks like %s contains multiple project repositories.\n", displayPath(dir))
	}
	r.asked = true
	itself, err := r.p.confirm(fmt.Sprintf("Set up %s as one project?", displayPath(dir)), false)
	if err != nil || itself {
		return initTarget{path: dir}, itself, err
	}
	if len(repos) == 0 {
		if total == 1 {
			fmt.Fprintf(r.cmd.OutOrStdout(), "The repository in %s is already set up.\n", displayPath(dir))
		} else {
			fmt.Fprintf(r.cmd.OutOrStdout(), "All %d repositories in %s are already set up.\n", total, displayPath(dir))
		}
		return initTarget{}, false, nil
	}

	if len(repos) > maxChoices {
		fmt.Fprintf(w, "Which repository do you want to set up? The %d most recently changed that aren't set up yet:\n", maxChoices)
		repos = repos[:maxChoices]
	} else {
		fmt.Fprintln(w, "Which repository do you want to set up?")
	}
	n := len(repos) + 1
	width := len(strconv.Itoa(n))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	now := time.Now()
	for i, repo := range repos {
		fmt.Fprintf(tw, "  %*d) %s\t%s\n", width, i+1, filepath.Base(repo.path), changedAgo(repo.changed, now))
	}
	tw.Flush()
	fmt.Fprintf(w, "  %*d) Another folder...\n", width, n)

	choice, err := r.ask("Enter your choice (1-"+strconv.Itoa(n)+")", "", "", "", numberIn(n))
	if err != nil {
		return initTarget{}, false, err
	}
	if i, _ := strconv.Atoi(choice); i < n {
		return r.target(ctx, repos[i-1].path, true, ws)
	}
	path, err := r.ask("Folder path", "", "", "", func(s string) (string, error) {
		path, err := filepath.Abs(expandHome(s))
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			return "", retry{fmt.Sprintf("No such folder: %s.", s), ""}
		}
		if err := checkInitPath(path); err != nil {
			return "", retry{err.Error(), ""}
		}
		return path, nil
	})
	if err != nil {
		return initTarget{}, false, err
	}
	return r.target(ctx, path, true, ws)
}

// numberIn accepts a whole number from 1 to n.
func numberIn(n int) func(string) (string, error) {
	return func(s string) (string, error) {
		if i, err := strconv.Atoi(s); err != nil || i < 1 || i > n {
			return "", retry{fmt.Sprintf("Choose a number from 1 to %d.", n), ""}
		}
		return s, nil
	}
}

// ask asks a question, unless flag already answers it or -y accepts def.
// what names the flag that answers it, for errors.
func (r *initRun) ask(label, flag, def, what string, check func(string) (string, error)) (string, error) {
	answer := flag
	if answer == "" && r.yes {
		if def == "" {
			return "", usageError{fmt.Errorf("can't suggest a %s here; pass --%s", what, what)}
		}
		answer = def
	}
	if answer != "" {
		v, err := check(answer)
		var rt retry
		if errors.As(err, &rt) {
			return "", usageError{fmt.Errorf("%s: %s", answer, rt.msg)}
		}
		return v, err
	}
	if !r.interactive {
		return "", errNeedTerminal
	}
	r.asked = true
	return r.p.ask(label, def, check)
}

// confirm asks a yes/no question; -y takes the default.
func (r *initRun) confirm(label string, def bool) (bool, error) {
	if r.yes {
		return def, nil
	}
	if !r.interactive {
		return false, errNeedTerminal
	}
	r.asked = true
	return r.p.confirm(label, def)
}

func (r *initRun) create(ctx context.Context, t initTarget, name string) error {
	projects, err := r.s.ListProjects(ctx, true)
	if err != nil {
		return err
	}
	owners := map[string]string{}
	for _, p := range projects {
		owners[p.TaskPrefix] = p.Name
	}
	taken := func(prefix string) bool { _, ok := owners[prefix]; return ok }
	prefix, err := r.ask("Task prefix", r.prefix, core.DefaultPrefix(name, taken), "prefix", func(s string) (string, error) {
		s = strings.ToUpper(s)
		if core.ValidatePrefix(s) != nil {
			return "", retry{"Prefixes are up to 10 letters and digits, starting with a letter.", ""}
		}
		if owner, ok := owners[s]; ok {
			return "", retry{fmt.Sprintf("%s is already used by project %s.", s, owner), core.FreePrefix(s, taken)}
		}
		return s, nil
	})
	if err != nil {
		return err
	}
	p, _, err := r.s.CreateProjectWithWorkspace(ctx, core.Project{Name: name, TaskPrefix: prefix}, core.Workspace{Path: t.path, GitCommonDir: t.gitCommonDir})
	if err != nil {
		return err
	}
	r.done(t, p, "Created project %s. Tasks will be numbered %s-1, %s-2, and so on.", p.Name, p.TaskPrefix, p.TaskPrefix)
	return nil
}

// link adds t to existing project p, replacing p's workspaces that no
// longer exist, such as an old clone that was moved or deleted.
func (r *initRun) link(ctx context.Context, t initTarget, p core.Project) error {
	if r.prefix != "" && strings.ToUpper(r.prefix) != p.TaskPrefix {
		return usageError{fmt.Errorf("%s already exists with prefix %s; --prefix can't change it", p.Name, p.TaskPrefix)}
	}
	ws, err := r.s.ListWorkspaces(ctx, p.Name)
	if err != nil {
		return err
	}
	for _, w := range ws {
		if _, err := os.Stat(w.Path); errors.Is(err, fs.ErrNotExist) {
			if _, err := r.s.RemoveWorkspace(ctx, w.Path); err != nil {
				return err
			}
		}
	}
	if _, err := r.s.AddWorkspace(ctx, p.Name, core.Workspace{Path: t.path, GitCommonDir: t.gitCommonDir}); err != nil {
		return err
	}
	r.done(t, p, "Linked %s to %s.", displayPath(t.path), p.Name)
	return nil
}

// done reports success and prints the next steps.
func (r *initRun) done(t initTarget, p core.Project, format string, a ...any) {
	out := r.cmd.OutOrStdout()
	if r.asked {
		fmt.Fprintln(out)
	}
	success(out, format, a...)
	fmt.Fprintln(out, "\nNext steps:")
	ref := p.TaskPrefix + "-1"
	tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintf(tw, "  taskship add \"Fix the export bug\"\tAdd a task from anywhere in this %s\n", t.kind())
	fmt.Fprintf(tw, "  taskship tasks\tList this project's tasks\n")
	fmt.Fprintf(tw, "  taskship show %s\tShow a task\n", ref)
	fmt.Fprintf(tw, "  taskship %s --dry-run\tPreview what an agent would be given\n", ref)
	tw.Flush()
}

func (r *initRun) alreadySetUp(ctx context.Context, t initTarget, project string) error {
	p, err := r.s.GetProject(ctx, project)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.cmd.OutOrStdout(), "Taskship is already set up for this %s.\n  Project: %s\n  Prefix:  %s\nRun \"taskship tasks\" to see its tasks.\n", t.kind(), p.Name, p.TaskPrefix)
	return nil
}

// checkName accepts a valid project name. For an invalid one, it suggests
// the name made from it as the new default.
func checkName(s string) (string, error) {
	if core.ValidateName(s) == nil {
		return s, nil
	}
	return "", retry{"Names use lowercase letters, digits, and hyphens.", core.NameFromFolder(s)}
}

// defaultName suggests a project name: for a whole repository, the
// repository name in its remote's URL; otherwise, or without a remote, the
// folder's name. For a bare repository in a folder named
// .bare or .git, the folder above it names the project.
func defaultName(ctx context.Context, t initTarget) string {
	if t.gitCommonDir != "" && !t.subfolder {
		if name := core.NameFromFolder(workspace.RemoteName(ctx, t.path)); name != "" {
			return name
		}
	}
	base := filepath.Base(t.path)
	if base == ".bare" || base == ".git" {
		base = filepath.Base(filepath.Dir(t.path))
	}
	return core.NameFromFolder(strings.TrimSuffix(base, ".git"))
}

// expandHome expands a leading ~ in a path typed at a prompt.
func expandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return path
}

// changedAgo describes when something changed, e.g. "changed 2 days ago".
func changedAgo(t, now time.Time) string {
	d := now.Sub(t)
	plural := func(n int, unit string) string {
		if n == 1 {
			return fmt.Sprintf("changed 1 %s ago", unit)
		}
		return fmt.Sprintf("changed %d %ss ago", n, unit)
	}
	switch {
	case d < time.Minute:
		return "changed just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour")
	case d < 48*time.Hour:
		return "changed yesterday"
	case d < 7*24*time.Hour:
		return plural(int(d.Hours()/24), "day")
	case t.Year() == now.Year():
		return "changed in " + t.Month().String()
	default:
		return fmt.Sprintf("changed in %s %d", t.Month(), t.Year())
	}
}
