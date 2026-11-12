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
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/display"
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
contains repositories, it asks whether to set up the whole folder or a
single repository in it, and then which repository.

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
			_, err = r.run(cmd.Context(), dir, explicit)
			return err
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
	forAdd       bool // run by taskship add: no header or next steps
	headed       bool // the header was printed; a blank line ends it before what follows
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

// run sets up dir and returns the project it was set up for, or a zero
// Project if there was nothing to set up.
func (r *initRun) run(ctx context.Context, dir string, explicit bool) (core.Project, error) {
	ws, err := r.s.ListWorkspaces(ctx, "")
	if err != nil {
		return core.Project{}, err
	}
	pl, err := locate(ctx, dir)
	if err != nil {
		return core.Project{}, err
	}
	return r.runAt(ctx, pl, explicit, ws)
}

// runAt is run for a folder already located, given every workspace.
func (r *initRun) runAt(ctx context.Context, pl place, explicit bool, ws []core.Workspace) (core.Project, error) {
	t, ok, err := r.targetAt(ctx, pl, explicit, ws)
	if err != nil || !ok {
		return core.Project{}, err
	}

	if !r.forAdd {
		out := r.cmd.OutOrStdout()
		lipgloss.Fprintln(out, headingf("Setting up Taskship for %s", display.Path(t.path)))
		lipgloss.Fprintln(out, faint.Render(noTraceLine(t.kind())))
		r.headed = true
	}

	def := defaultName(ctx, t)
	for {
		name, err := r.ask("Project name", r.name, def, "name", checkName)
		if err != nil {
			return core.Project{}, err
		}
		p, err := r.s.GetProject(ctx, name)
		if errors.Is(err, core.ErrNotFound) {
			return r.create(ctx, t, name)
		} else if err != nil {
			return core.Project{}, err
		}
		if r.interactive && !r.yes {
			r.endHeader()
			lipgloss.Fprintf(r.p.out, "A project named %s already exists (prefix %s).\n", value.Render(p.Name), value.Render(p.TaskPrefix))
		}
		link, err := r.confirm("Link this folder to it?", true)
		if err != nil {
			return core.Project{}, err
		}
		if link {
			return p, r.link(ctx, t, p)
		}
		if r.name != "" {
			r.gap()
			lipgloss.Fprintln(r.cmd.OutOrStdout(), "Nothing changed. To set up a new project, pass a different --name.")
			return core.Project{}, nil
		}
		def = ""
	}
}

// noTraceLine says that Taskship leaves the repository or folder alone.
func noTraceLine(kind string) string {
	home, err := homeDir()
	if err != nil {
		home = "~/.taskship"
	}
	return fmt.Sprintf("Taskship doesn't write anything to this %s. Everything lives in %s.", kind, display.Path(home))
}

// target works out the folder to set up from dir: a repository's main
// checkout, or a folder. It asks when dir is a subfolder of a repository,
// already part of a project, or a folder of repositories. ok is false if
// there is nothing to set up, and init has said why.
func (r *initRun) target(ctx context.Context, dir string, explicit bool, ws []core.Workspace) (t initTarget, ok bool, err error) {
	pl, err := locate(ctx, dir)
	if err != nil {
		return initTarget{}, false, err
	}
	return r.targetAt(ctx, pl, explicit, ws)
}

// place is where a folder is: its Location, its position in the main
// checkout (pos), and the main checkout ("" outside a repository).
type place struct {
	loc       workspace.Location
	pos, main string
}

// whole reports whether the folder is a whole repository's main checkout.
func (pl place) whole() bool { return pl.loc.GitCommonDir != "" && pl.pos == pl.main }

func locate(ctx context.Context, dir string) (place, error) {
	if abs, err := filepath.Abs(dir); err == nil {
		if _, err := os.Stat(abs); errors.Is(err, fs.ErrNotExist) {
			return place{}, kindError{pathRef(abs) + " doesn't exist", core.ErrInvalid}
		}
	}
	loc, err := workspace.Inspect(ctx, dir)
	if err != nil {
		return place{}, err
	}
	return placeOf(ctx, loc), nil
}

func placeOf(ctx context.Context, loc workspace.Location) place {
	// A bare repository has no folders of its own, so its worktrees count
	// as a whole.
	pl := place{loc: loc, pos: loc.Path}
	if loc.MainPath != "" {
		pl.pos = loc.MainPath
	}
	if loc.GitCommonDir != "" {
		var bare bool
		if pl.main, bare = workspace.MainCheckout(ctx, loc.Path); pl.main == "" {
			pl.main = pl.pos
		} else if bare {
			pl.pos = pl.main
		}
	}
	return pl
}

func (r *initRun) targetAt(ctx context.Context, pl place, explicit bool, ws []core.Workspace) (t initTarget, ok bool, err error) {
	loc, pos, main := pl.loc, pl.pos, pl.main
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
		if repos, setUp, total := childRepos(ctx, loc.Path, ws); total > 0 {
			return r.inFolderOfRepos(ctx, loc.Path, repos, setUp, total, ws)
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
	lipgloss.Fprintln(out, headingf("This folder is already part of a project (name: %s, prefix: %s).", p.Name, p.TaskPrefix)+"\n")
	yes, err := r.confirm("Set up a separate project for this folder?", false)
	if err != nil {
		return initTarget{}, false, err
	}
	r.gap()
	if !yes {
		lipgloss.Fprintln(out, "Nothing changed. Run "+cmdRef("taskship tasks")+" to see its tasks.")
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
	lipgloss.Fprintln(w, headingf(which+" (%s) is part of a repository (%s).", display.Path(loc.Path), display.Path(main))+"\n")
	fmt.Fprintln(w, "Where do you want to set up a Taskship project?")
	lipgloss.Fprintf(w, "  1) The whole repository (%s)\n", pathRef(main))
	lipgloss.Fprintf(w, "  2) Only this folder (%s)\n", pathRef(pos))
	choice, err := r.ask("Enter your choice", "", "1", "", numberIn(2))
	if err != nil {
		return initTarget{}, false, err
	}
	r.gap()
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
	return usageError{fmt.Errorf("run %s inside a project's folder, not %s, or pass a folder path, for example: %s", cmdRef("taskship init"), where, command.Render("taskship init ~/code/myproject"))}
}

// childRepo is a Git repository directly inside a folder.
type childRepo struct {
	path    string
	changed time.Time
	project string // the project it's set up for, if any
}

// childRepos returns the repositories directly inside dir that aren't set
// up yet, most recently changed first, those that are, by name, and how
// many repositories there are in all. A repository's last change is the modification time of its .git
// entry, which commits, checkouts and fetches update.
func childRepos(ctx context.Context, dir string, ws []core.Workspace) (repos, setUp []childRepo, total int) {
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
		w, ok, err := workspace.Resolve(loc, ws)
		if err != nil {
			continue
		}
		if ok {
			setUp = append(setUp, childRepo{loc.Path, info.ModTime(), w.Project})
			continue
		}
		repos = append(repos, childRepo{loc.Path, info.ModTime(), ""})
	}
	slices.SortStableFunc(repos, func(a, b childRepo) int { return b.changed.Compare(a.changed) })
	return repos, setUp, total
}

// maxChoices is how many repositories are listed to choose from.
const maxChoices = 8

// inFolderOfRepos asks whether to set up dir, a folder of repositories, as
// a whole or a single repository in it, and then which repository.
func (r *initRun) inFolderOfRepos(ctx context.Context, dir string, repos, setUp []childRepo, total int, ws []core.Workspace) (initTarget, bool, error) {
	if r.yes || !r.interactive {
		return initTarget{}, false, usageError{fmt.Errorf("%s contains Git repositories; pass the one to set up, for example: %s", pathRef(dir), command.Render("taskship init "+display.Path(filepath.Join(dir, "myproject"))))}
	}
	w := r.p.out
	contains, one := "a repository", "The repository in it"
	if only := slices.Concat(repos, setUp); total == 1 && len(only) == 1 {
		one = fmt.Sprintf("The repository (%s)", pathRef(only[0].path))
	} else if total > 1 {
		contains, one = fmt.Sprintf("%d repositories", total), "A single repository"
	}
	lipgloss.Fprintln(w, headingf("The current folder (%s) contains "+contains+".", display.Path(dir))+"\n")
	fmt.Fprintln(w, "Where do you want to set up a Taskship project?")
	lipgloss.Fprintf(w, "  1) The whole folder (%s)\n", pathRef(dir))
	lipgloss.Fprintf(w, "  2) %s\n", one)
	r.asked = true
	where, err := r.p.ask("Enter your choice", "2", numberIn(2))
	if err != nil {
		return initTarget{}, false, err
	}
	r.gap()
	if where == "1" {
		return initTarget{path: dir}, true, nil
	}
	if len(repos) == 0 {
		return initTarget{}, false, r.allSetUp(ctx, dir, setUp)
	}

	if len(repos) > maxChoices {
		fmt.Fprintf(w, "Which repository do you want to set up? The %d most recently changed that aren't set up yet:\n", maxChoices)
		repos = repos[:maxChoices]
	} else {
		fmt.Fprintln(w, "Which repository do you want to set up?")
	}
	n := len(repos) + 1
	width := len(strconv.Itoa(n))
	for i, repo := range repos {
		fmt.Fprintf(w, "  %*d) %s\n", width, i+1, filepath.Base(repo.path))
	}
	fmt.Fprintf(w, "  %*d) Another folder...\n", width, n)

	choice, err := r.ask("Enter your choice (1-"+strconv.Itoa(n)+")", "", "", "", numberIn(n))
	if err != nil {
		return initTarget{}, false, err
	}
	if i, _ := strconv.Atoi(choice); i < n {
		r.gap()
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
	r.gap()
	return r.target(ctx, path, true, ws)
}

// allSetUp reports that every repository in dir is already set up, and for
// which projects.
func (r *initRun) allSetUp(ctx context.Context, dir string, setUp []childRepo) error {
	head, where := fmt.Sprintf("All %d repositories in %s are already set up as Taskship projects:", len(setUp), pathRef(dir)), "one of them"
	if len(setUp) == 1 {
		head, where = fmt.Sprintf("The repository in %s is already set up as a Taskship project:", pathRef(dir)), "it"
	}
	var b strings.Builder
	b.WriteString(head + "\n")
	for _, repo := range setUp {
		p, err := r.s.GetProject(ctx, repo.project)
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "  %s (name: %s, prefix: %s)\n", filepath.Base(repo.path), value.Render(p.Name), value.Render(p.TaskPrefix))
	}
	fmt.Fprintf(&b, "\nRun %s in %s to see its tasks.\n", cmdRef("taskship tasks"), where)
	lipgloss.Fprint(r.cmd.OutOrStdout(), b.String())
	return nil
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
			src := "--" + what
			if flag == "" {
				src = "default " + what // from -y
			}
			return "", usageError{fmt.Errorf("invalid %s %q: %s", src, answer, strings.ToLower(rt.msg[:1])+rt.msg[1:])}
		}
		return v, err
	}
	if !r.interactive {
		return "", errNeedTerminal
	}
	r.endHeader()
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
	r.endHeader()
	r.asked = true
	return r.p.confirm(label, def)
}

// endHeader prints the blank line after the header, once, so an error
// right after the header doesn't get two (it prints its own).
func (r *initRun) endHeader() {
	if r.headed {
		fmt.Fprintln(r.cmd.OutOrStdout())
	}
	r.headed = false
}

func (r *initRun) create(ctx context.Context, t initTarget, name string) (core.Project, error) {
	projects, err := r.s.ListProjects(ctx, true)
	if err != nil {
		return core.Project{}, err
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
		return core.Project{}, err
	}
	p, _, err := r.s.CreateProjectWithWorkspace(ctx, core.Project{Name: name, TaskPrefix: prefix}, core.Workspace{Path: t.path, GitCommonDir: t.gitCommonDir})
	if err != nil {
		return core.Project{}, err
	}
	if r.forAdd {
		r.done(t, p, "Created project %s.", value.Render(p.Name))
	} else {
		r.done(t, p, "Created project %s. Tasks will be numbered %s-1, %s-2, and so on.", value.Render(p.Name), p.TaskPrefix, p.TaskPrefix)
	}
	return p, nil
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
	r.done(t, p, "Linked %s to %s.", pathRef(t.path), value.Render(p.Name))
	return nil
}

// gap prints a blank line after a question, before what follows it, if a
// question was asked.
func (r *initRun) gap() {
	if r.asked {
		fmt.Fprintln(r.p.out)
	}
}

// done reports success and prints the next steps.
func (r *initRun) done(t initTarget, p core.Project, format string, a ...any) {
	out := r.cmd.OutOrStdout()
	if r.asked || r.headed {
		fmt.Fprintln(out)
	}
	success(out, format, a...)
	if r.forAdd {
		return
	}
	lipgloss.Fprintln(out, "\n"+heading.Render("Next steps:"))
	ref := p.TaskPrefix + "-1"
	columns(out, 3, [][2]string{
		{command.Render(`taskship add "Fix the export bug"`), faint.Render("Add a task from anywhere in this " + t.kind())},
		{command.Render("taskship tasks"), faint.Render("List this project's tasks")},
		{command.Render("taskship show " + ref), faint.Render("Show a task")},
		{command.Render("taskship " + ref + " --dry-run"), faint.Render("Preview what an agent would be given")},
	})
}

func (r *initRun) alreadySetUp(ctx context.Context, t initTarget, project string) error {
	p, err := r.s.GetProject(ctx, project)
	if err != nil {
		return err
	}
	lipgloss.Fprintf(r.cmd.OutOrStdout(), "%s\n  Project: %s\n  Prefix:  %s\n\nRun %s to see its tasks.\n", heading.Render("Taskship is already set up for this "+t.kind()+"."), value.Render(p.Name), value.Render(p.TaskPrefix), cmdRef("taskship tasks"))
	return nil
}

// checkName accepts a valid project name. For an invalid one, it suggests
// the name made from it as the new default.
func checkName(s string) (string, error) {
	if core.ValidateName(s) == nil {
		return s, nil
	}
	return "", retry{"Names use lowercase letters, digits, and hyphens (no spaces).", core.NameFromFolder(s)}
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

var noProjectsYet = "No projects yet. Run " + cmdRef("taskship init") + " in a repository to set one up."

var errNotSetUp = usageError{errors.New("this folder isn't part of a Taskship project. Run " + cmdRef("taskship init") + ` to set it up, or pass --project NAME to add to an existing project.`)}

// offerSetup is called by taskship add in a folder with no project, given
// the folder's Location and every workspace. If no project exists yet and
// there's a terminal, it offers to set one up here, as init would, and
// returns its name.
func offerSetup(cmd *cobra.Command, s *store.Store, loc workspace.Location, ws []core.Workspace) (string, error) {
	ctx := cmd.Context()
	projects, err := s.ListProjects(ctx, true)
	if err != nil {
		return "", err
	}
	if len(projects) > 0 || !inputIsTerminal(cmd.InOrStdin()) {
		return "", errNotSetUp
	}
	pl := placeOf(ctx, loc)
	if err := checkInitPath(pl.pos); err != nil {
		return "", err
	}
	kind := "folder"
	if pl.whole() {
		kind = "repository"
	}
	out := cmd.OutOrStdout()
	lipgloss.Fprintf(out, "%s\n%s\n\n", heading.Render(fmt.Sprintf("This %s isn't set up with Taskship yet.", kind)), faint.Render(noTraceLine(kind)))
	r := initRun{cmd: cmd, s: s, p: newPrompter(cmd), interactive: true, forAdd: true}
	if yes, err := r.confirm("Set up Taskship now?", true); err != nil {
		return "", err
	} else if !yes {
		r.gap()
		lipgloss.Fprintf(out, "No task added. Run %s to set up this %s, or pass --project NAME.\n", cmdRef("taskship init"), kind)
		return "", errDeclined
	}
	p, err := r.runAt(ctx, pl, false, ws)
	if err != nil {
		return "", err
	}
	if p.Name == "" {
		return "", errNotSetUp
	}
	return p.Name, nil
}
