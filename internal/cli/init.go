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
		Short: "Set up Taskship for a repository or folder",
		Long: `Set up Taskship for the repository or folder at PATH (default: the current
directory): create a project for it, or link it to an existing project of the
same name. Taskship doesn't write anything to the repository or folder.

In a Git repository, its main checkout is set up, and every worktree of it is
recognized. Run in a folder that contains repositories, init asks which one
to set up.

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
}

func (t initTarget) kind() string {
	if t.gitCommonDir != "" {
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
	t, err := r.target(ctx, dir, explicit, ws)
	if err != nil {
		return err
	}
	if w, ok, err := workspace.Resolve(workspace.Location{Path: t.path, GitCommonDir: t.gitCommonDir}, ws); err != nil {
		return err
	} else if ok {
		return r.alreadySetUp(ctx, t, w.Project)
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
		link, err := r.confirm(fmt.Sprintf("%s already exists (prefix %s). Link this folder to it?", p.Name, p.TaskPrefix))
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

// target works out the folder to set up from dir: the main checkout of a Git
// repository, a repository chosen from those in dir, or dir itself.
func (r *initRun) target(ctx context.Context, dir string, explicit bool, ws []core.Workspace) (initTarget, error) {
	loc, err := workspace.Inspect(ctx, dir)
	if err != nil {
		return initTarget{}, err
	}
	path, commonDir := loc.Path, loc.GitCommonDir
	if commonDir != "" {
		if main, _ := workspace.MainCheckout(ctx, path); main != "" {
			path = main
		}
		return initTarget{path, commonDir}, checkInitPath(path)
	}
	if err := checkInitPath(path); err != nil {
		return initTarget{}, err
	}
	if _, ok, _ := workspace.Resolve(loc, ws); !ok && !explicit {
		repos, total := childRepos(ctx, path, ws)
		if total > 0 {
			return r.choose(ctx, path, repos, total, ws)
		}
	}
	return initTarget{path: path}, nil
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

// maxChoices is how many repositories the chooser lists.
const maxChoices = 8

// choose asks which repository in dir to set up, offering another folder or
// dir itself too.
func (r *initRun) choose(ctx context.Context, dir string, repos []childRepo, total int, ws []core.Workspace) (initTarget, error) {
	if r.yes || !r.interactive {
		return initTarget{}, usageError{fmt.Errorf("%s contains Git repositories; pass the one to set up, for example: taskship init %s", displayPath(dir), displayPath(filepath.Join(dir, "myproject")))}
	}
	w := r.p.out
	switch {
	case len(repos) == 0:
		fmt.Fprintf(w, "All %d repositories in %s are already set up.\n", total, displayPath(dir))
	case len(repos) > maxChoices:
		fmt.Fprintf(w, "%s isn't a Git repository, but it contains %d that aren't set up yet. The %d most recently changed:\n", displayPath(dir), len(repos), maxChoices)
		repos = repos[:maxChoices]
	default:
		fmt.Fprintf(w, "%s isn't a Git repository, but it contains %d that aren't set up yet:\n", displayPath(dir), len(repos))
	}
	n := len(repos) + 2
	width := len(strconv.Itoa(n))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	now := time.Now()
	for i, repo := range repos {
		fmt.Fprintf(tw, "  %*d) %s\t%s\n", width, i+1, filepath.Base(repo.path), changedAgo(repo.changed, now))
	}
	tw.Flush()
	fmt.Fprintf(w, "  %*d) Another folder...\n", width, n-1)
	fmt.Fprintf(w, "  %*d) %s itself\n", width, n, displayPath(dir))

	choice, err := r.ask("Which one do you want to set up? (1-"+strconv.Itoa(n)+")", "", "", "", func(s string) (string, error) {
		if i, err := strconv.Atoi(s); err != nil || i < 1 || i > n {
			return "", retry{fmt.Sprintf("Choose a number from 1 to %d.", n), ""}
		}
		return s, nil
	})
	if err != nil {
		return initTarget{}, err
	}
	switch i, _ := strconv.Atoi(choice); {
	case i <= len(repos):
		return r.target(ctx, repos[i-1].path, true, ws)
	case i == n:
		return initTarget{path: dir}, nil
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
		return initTarget{}, err
	}
	return r.target(ctx, path, true, ws)
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

// confirm asks a yes/no question whose default is yes; -y answers it.
func (r *initRun) confirm(label string) (bool, error) {
	if r.yes {
		return true, nil
	}
	if !r.interactive {
		return false, errNeedTerminal
	}
	r.asked = true
	return r.p.confirm(label, true)
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

// defaultName suggests a project name: the repository name in its remote's
// URL, or else the folder's name. For a bare repository in a folder named
// .bare or .git, the folder above it names the project.
func defaultName(ctx context.Context, t initTarget) string {
	if t.gitCommonDir != "" {
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
