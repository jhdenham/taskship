package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
	}
	cmd.AddCommand(
		newTaskAddCmd("add"),
		newTaskListCmd("list"),
		newTaskShowCmd("show"),
		newTaskEditCmd(),
		newTaskArchiveCmd(true),
		newTaskArchiveCmd(false),
	)
	return cmd
}

// The constructors take the command name so the root shortcuts
// (taskship add, tasks, show) are the same commands as task add, list, show.

func newTaskAddCmd(name string) *cobra.Command {
	var project, description, status, priority string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   name + " TITLE [--project NAME]",
		Short: "Create a task",
		Long: `Create a task. Without --project, the project is the one set up for the
current folder, its repository, or any worktree of it (see "taskship init").
If no project exists yet, add offers to set one up first.`,
		Example: "  taskship " + name + " \"Fix export\"\n  taskship " + name + " \"Fix export\" --project myproject",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := core.ParseStatus(status)
			if err != nil {
				return err
			}
			pr, err := core.ParsePriority(priority)
			if err != nil {
				return err
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			setUp := false
			if project == "" {
				loc, ws, err := currentDir(cmd.Context(), s)
				if err != nil {
					return err
				}
				var ok bool
				if project, ok, err = projectAt(loc, ws); err != nil {
					return err
				} else if !ok {
					if asJSON {
						return errNotSetUp
					}
					if project, err = offerSetup(cmd, s, loc, ws); err != nil {
						return err
					}
					setUp = true
				}
			}
			t, err := s.CreateTask(cmd.Context(), project, core.Task{
				Title: args[0], Description: description, Status: st, Priority: pr,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, t)
			}
			success(cmd.OutOrStdout(), "Created %s: %s", t.Ref, t.Title)
			if setUp {
				fmt.Fprintln(cmd.OutOrStdout(), `Run "taskship tasks" to list this project's tasks.`)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name (default: the project of the current folder)")
	cmd.Flags().StringVar(&description, "description", "", "task description")
	cmd.Flags().StringVar(&status, "status", string(core.StatusTodo), "initial status")
	cmd.Flags().StringVar(&priority, "priority", string(core.PriorityNone), "priority: none, low, medium, high, urgent")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newTaskListCmd(name string) *cobra.Command {
	var project string
	var statuses []string
	var asJSON, archived, allProjects bool
	cmd := &cobra.Command{
		Use:   name + " [--project NAME | --all-projects]",
		Short: "List tasks",
		Long: `List tasks.

By default, tasks are listed for the project of the current folder (see
"taskship init"), or across all projects when the folder belongs to no
project. --project and --all-projects choose explicitly. In a terminal, a
first line says what is shown and how to change it.`,
		Example: "  taskship " + name + "\n" +
			"  taskship " + name + " --all-projects\n" +
			"  taskship " + name + " --project myproject --status todo,in_progress",
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project != "" && allProjects {
				return usageError{fmt.Errorf("--project and --all-projects cannot be used together")}
			}
			filter := store.TaskFilter{Project: project, IncludeArchived: archived}
			for _, s := range statuses {
				st, err := core.ParseStatus(s)
				if err != nil {
					return err
				}
				filter.Statuses = append(filter.Statuses, st)
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			scope := listScope{project: project}
			if project == "" && !allProjects {
				if filter.Project, scope.inferred, err = projectForCurrentDir(cmd.Context(), s); err != nil {
					return err
				}
				scope.project = filter.Project
				scope.noProjectHere = !scope.inferred
			}
			tasks, err := s.ListTasks(cmd.Context(), filter)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, tasks)
			}
			out := cmd.OutOrStdout()
			if len(tasks) == 0 {
				if scope.project == "" {
					if projects, err := s.ListProjects(cmd.Context(), true); err != nil {
						return err
					} else if len(projects) == 0 {
						fmt.Fprintln(out, noProjectsYet)
						return nil
					}
				}
				fmt.Fprintln(out, scope.summary(filter, 0))
				return nil
			}
			if isTerminal(out) {
				fmt.Fprintf(out, "%s\n\n", scope.summary(filter, len(tasks)))
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "REF\tSTATUS\tPRIORITY\tTITLE")
			for _, t := range tasks {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s%s\n", t.Ref, t.Status, t.Priority, t.Title, archivedMark(t.ArchivedAt))
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "only tasks in this project (default: the project of the current folder)")
	cmd.Flags().BoolVar(&allProjects, "all-projects", false, "tasks in all projects, wherever you are")
	cmd.Flags().StringSliceVar(&statuses, "status", nil, "only tasks with these statuses (comma-separated or repeated)")
	cmd.Flags().BoolVar(&archived, "archived", false, "include archived tasks and tasks in archived projects")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

// listScope records how a task list's project was chosen, to explain it.
type listScope struct {
	project       string // "" for all projects
	inferred      bool   // project came from the current folder
	noProjectHere bool   // all projects because the folder has no project
}

// summary describes what a task list shows and how to change it, e.g.
// "Showing 3 tasks in myproject (this folder). Use --all-projects to see every project."
func (sc listScope) summary(f store.TaskFilter, n int) string {
	what := "tasks"
	if n == 1 {
		what = "task"
	}
	shown := fmt.Sprintf("Showing %d %s", n, what)
	if n == 0 {
		shown = "No tasks"
	}
	where, hint := " across all projects", ""
	switch {
	case sc.project != "" && sc.inferred:
		where, hint = " in "+sc.project+" (this folder)", " Use --all-projects to see every project."
	case sc.project != "":
		where, hint = " in "+sc.project, " Use --all-projects to see every project."
	case sc.noProjectHere && n > 0:
		hint = " Use --project NAME for one project."
	}
	var filters []string
	if len(f.Statuses) > 0 {
		filters = append(filters, "with status "+strings.Join(statusNames(f.Statuses), ", "))
	}
	if f.IncludeArchived {
		filters = append(filters, "including archived")
	}
	if len(filters) > 0 {
		where += ", " + strings.Join(filters, ", ")
	}
	return shown + where + "." + hint
}

func statusNames(sts []core.Status) []string {
	out := make([]string, len(sts))
	for i, st := range sts {
		out[i] = string(st)
	}
	return out
}

// isTerminal reports whether w is a terminal. The task list summary line is
// shown only in terminals, so piped output stays plain rows. Tests override it.
var isTerminal = func(w io.Writer) bool { return isCharDevice(w) }

// inputIsTerminal reports whether r is a terminal someone can answer
// questions on. Tests override it.
var inputIsTerminal = func(r io.Reader) bool { return isCharDevice(r) }

func isCharDevice(v any) bool {
	f, ok := v.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func newTaskShowCmd(name string) *cobra.Command {
	var asJSON, asMarkdown bool
	cmd := &cobra.Command{
		Use:     name + " TASK_REF",
		Short:   "Show task details",
		Example: "  taskship " + name + " PROJ-1\n  taskship " + name + " PROJ-1 --md",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if asJSON && asMarkdown {
				return usageError{fmt.Errorf("--json and --md cannot be used together")}
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			t, err := s.GetTask(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, t)
			}
			if asMarkdown {
				_, err := fmt.Fprint(cmd.OutOrStdout(), core.TaskMarkdown(t))
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  %s\n\n", t.Ref, t.Title)
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "Project:\t%s\n", t.Project)
			fmt.Fprintf(w, "Status:\t%s\n", t.Status)
			fmt.Fprintf(w, "Priority:\t%s\n", t.Priority)
			fmt.Fprintf(w, "Created:\t%s\n", t.CreatedAt.Local().Format(time.DateTime))
			fmt.Fprintf(w, "Updated:\t%s\n", t.UpdatedAt.Local().Format(time.DateTime))
			if t.ArchivedAt != nil {
				fmt.Fprintf(w, "Archived:\t%s\n", t.ArchivedAt.Local().Format(time.DateTime))
			}
			fmt.Fprintf(w, "ID:\t%s\n", t.ID)
			if err := w.Flush(); err != nil {
				return err
			}
			if t.Description != "" {
				fmt.Fprintf(out, "\n%s\n", t.Description)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().BoolVar(&asMarkdown, "md", false, "output Markdown, the format used by edit and export")
	return cmd
}

func newTaskEditCmd() *cobra.Command {
	var title, description, status, priority, from string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "edit TASK_REF [--title TITLE] [--description TEXT] [--status STATUS] [--priority PRIORITY] [--from FILE]",
		Short: "Change a task's title, description, status, or priority",
		Long: `Change a task's title, description, status, or priority.

With no flags, the task opens as Markdown in $VISUAL or $EDITOR (default vi).
With --from, the Markdown is read from a file, or from stdin if FILE is "-".
Only fields that differ from the current task are changed. See "taskship show --md"
for the format.`,
		Example: "  taskship task edit PROJ-1 --status in_progress --priority high\n" +
			"  taskship task edit PROJ-1\n" +
			"  taskship show PROJ-1 --md | sed 's/^status: .*/status: done/' | taskship task edit PROJ-1 --from -",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var patch store.TaskPatch
			flags := cmd.Flags()
			if flags.Changed("title") {
				patch.Title = &title
			}
			if flags.Changed("description") {
				patch.Description = &description
			}
			if flags.Changed("status") {
				st, err := core.ParseStatus(status)
				if err != nil {
					return err
				}
				patch.Status = &st
			}
			if flags.Changed("priority") {
				pr, err := core.ParsePriority(priority)
				if err != nil {
					return err
				}
				patch.Priority = &pr
			}
			fromFile := flags.Changed("from")
			if fromFile && patch != (store.TaskPatch{}) {
				return usageError{fmt.Errorf("--from cannot be combined with --title, --description, --status, or --priority")}
			}
			ctx := cmd.Context()
			s, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			// draft is the editor's temporary file. It is kept if the edits
			// cannot be applied, so they are not lost.
			var draft string
			keepDraft := func(err error) error {
				if draft == "" {
					return err
				}
				return fmt.Errorf("%w\nYour edits are saved in %s. Fix them and run: taskship task edit %s --from %s", err, draft, args[0], draft)
			}
			if patch == (store.TaskPatch{}) {
				t, err := s.GetTask(ctx, args[0])
				if err != nil {
					return err
				}
				var src []byte
				if fromFile {
					src, err = readFrom(cmd, from)
				} else {
					draft, src, err = editMarkdown(cmd, t)
				}
				if err != nil {
					return err
				}
				if patch, err = markdownPatch(t, string(src)); err != nil {
					return keepDraft(err)
				}
				if patch == (store.TaskPatch{}) {
					removeDraft(draft)
					if asJSON {
						return writeJSON(cmd, t)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "No changes to %s\n", t.Ref)
					return nil
				}
			}
			t, err := s.UpdateTask(ctx, args[0], patch)
			if err != nil {
				return keepDraft(err)
			}
			removeDraft(draft)
			if asJSON {
				return writeJSON(cmd, t)
			}
			success(cmd.OutOrStdout(), "Updated %s: %s", t.Ref, t.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&status, "status", "", "new status: backlog, todo, in_progress, done, canceled")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority: none, low, medium, high, urgent")
	cmd.Flags().StringVar(&from, "from", "", `read the task as Markdown from FILE ("-" for stdin)`)
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

// markdownPatch compares a Markdown task document with the current task and
// returns a patch of the fields that differ.
func markdownPatch(t core.Task, src string) (store.TaskPatch, error) {
	var patch store.TaskPatch
	doc, err := core.ParseTaskMarkdown(src)
	if err != nil {
		return patch, err
	}
	if doc.Ref != "" && !strings.EqualFold(doc.Ref, t.Ref) {
		return patch, fmt.Errorf("%w: ref is %s and cannot be changed (found %q)", core.ErrInvalid, t.Ref, doc.Ref)
	}
	if doc.Project != "" && doc.Project != t.Project {
		return patch, fmt.Errorf("%w: project is %s and cannot be changed here (found %q)", core.ErrInvalid, t.Project, doc.Project)
	}
	if doc.Title != t.Title {
		patch.Title = &doc.Title
	}
	if doc.Description != strings.Trim(t.Description, "\n") {
		patch.Description = &doc.Description
	}
	if doc.Status != "" && doc.Status != t.Status {
		patch.Status = &doc.Status
	}
	if doc.Priority != "" && doc.Priority != t.Priority {
		patch.Priority = &doc.Priority
	}
	return patch, nil
}

// editMarkdown writes the task to a temporary Markdown file, opens it in the
// user's editor, and returns the file's path and edited contents.
func editMarkdown(cmd *cobra.Command, t core.Task) (string, []byte, error) {
	f, err := os.CreateTemp("", t.Ref+"-*.md")
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	_, err = f.WriteString(core.TaskMarkdown(t))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return "", nil, err
	}

	editor := strings.Fields(os.Getenv("VISUAL"))
	if len(editor) == 0 {
		editor = strings.Fields(os.Getenv("EDITOR"))
	}
	if len(editor) == 0 {
		editor = []string{"vi"}
	}
	c := exec.CommandContext(cmd.Context(), editor[0], append(editor[1:], path)...)
	c.Stdin, c.Stdout, c.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		os.Remove(path)
		return "", nil, fmt.Errorf("editor %s: %w", editor[0], err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		os.Remove(path)
		return "", nil, err
	}
	return path, src, nil
}

func removeDraft(path string) {
	if path != "" {
		os.Remove(path)
	}
}

func readFrom(cmd *cobra.Command, path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(cmd.InOrStdin())
	}
	return os.ReadFile(path)
}

// newTaskArchiveCmd returns "task archive" or, if archive is false, "task restore".
func newTaskArchiveCmd(archive bool) *cobra.Command {
	verb, short, done := "archive", "Archive a task and hide it from default lists", "Archived"
	if !archive {
		verb, short, done = "restore", "Restore an archived task", "Restored"
	}
	var asJSON bool
	cmd := &cobra.Command{
		Use:   verb + " TASK_REF",
		Short: short,
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			t, err := s.SetTaskArchived(cmd.Context(), args[0], archive)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, t)
			}
			success(cmd.OutOrStdout(), "%s %s: %s", done, t.Ref, t.Title)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
