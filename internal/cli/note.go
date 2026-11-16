package cli

import (
	"errors"
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
)

func newNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Add, list, edit, and delete notes",
		Long: `Notes hold thoughts that aren't tasks yet: ideas, snippets, agent output.
Each belongs to a project and is numbered #1, #2, ... in the order it was
added. A note written from an agent run that Taskship launched records the
run. In the terminal UI, a note can become a task.`,
	}
	cmd.AddCommand(newNoteAddCmd(), newNoteListCmd(), newNoteEditCmd(), newNoteDeleteCmd())
	return cmd
}

// addProjectFlag adds --project to a note command.
func addProjectFlag(cmd *cobra.Command, project *string) {
	cmd.Flags().StringVarP(project, "project", "p", "", "project name (default: the project of the current folder)")
}

// errNoProjectHere is errNotSetUp for note commands that don't add.
var errNoProjectHere = usageError{errors.New("this folder isn't part of a Taskship project. Run " + cmdRef("taskship init") + " to set it up, or pass --project NAME.")}

// noteProject returns project, or the project of the current folder if it
// is empty; notSetUp is the error when the folder has none.
func noteProject(cmd *cobra.Command, s *store.Store, project string, notSetUp error) (string, error) {
	if project != "" {
		return project, nil
	}
	project, ok, err := projectForCurrentDir(cmd.Context(), s)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", notSetUp
	}
	return project, nil
}

func newNoteAddCmd() *cobra.Command {
	var project string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "add [TEXT] [--project NAME]",
		Short: "Add a note",
		Long: `Add a note to a project. The note is TEXT, or stdin if TEXT is "-".
Without TEXT, it opens $VISUAL or $EDITOR; an empty note adds nothing.
Without --project, the project is the one set up for the current folder.`,
		Example: "  taskship note add \"Try a compact board layout\"\n" +
			"  taskship note add\n" +
			"  pbpaste | taskship note add -",
		Args: rangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if project, err = noteProject(cmd, s, project, errNotSetUp); err != nil {
				return err
			}
			// Check before the editor opens, so nobody writes a note that
			// can't be saved.
			if p, err := s.GetProject(ctx, project); err != nil {
				return err
			} else if p.ArchivedAt != nil {
				return fmt.Errorf("%w: project %q is archived; restore it with: taskship project restore %s", core.ErrInvalid, p.Name, p.Name)
			}
			body, err := bodyText(cmd, args, "note", project+"-note-*.md", "")
			if err != nil {
				return err
			}
			n, err := s.AddNote(ctx, project, body, os.Getenv(runIDEnv))
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, n)
			}
			success(cmd.OutOrStdout(), "Added note #%d to %s", n.Number, project)
			return nil
		},
	}
	addProjectFlag(cmd, &project)
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newNoteListCmd() *cobra.Command {
	var project string
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list [--project NAME]",
		Short:   "List a project's notes, newest first",
		Example: "  taskship note list\n  taskship note list --project myproject --json",
		Args:    exactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if project, err = noteProject(cmd, s, project, errNoProjectHere); err != nil {
				return err
			}
			p, err := s.GetProject(ctx, project)
			if err != nil {
				return err
			}
			notes, err := s.ListNotes(ctx, p.ID)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, notes)
			}
			w := cmd.OutOrStdout()
			if len(notes) == 0 {
				lipgloss.Fprintf(w, "No notes in %s yet. Add one with %s.\n", p.Name, cmdRef("taskship note add"))
				return nil
			}
			for i, n := range notes {
				if i > 0 {
					fmt.Fprintln(w)
				}
				lipgloss.Fprintln(w, faint.Render(core.NoteHeading(n)))
				fmt.Fprintln(w, n.Body)
			}
			return nil
		},
	}
	addProjectFlag(cmd, &project)
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newNoteEditCmd() *cobra.Command {
	var project string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "edit NUMBER [TEXT] [--project NAME]",
		Short: "Change a note",
		Long: `Replace the text of a note. The new text is TEXT, or stdin if TEXT is "-".
Without TEXT, the note opens in $VISUAL or $EDITOR.`,
		Example: "  taskship note edit 3\n  taskship note edit 3 \"Rename sync settings to connections?\"",
		Args:    rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := core.ParseNoteNumber(args[0])
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if project, err = noteProject(cmd, s, project, errNoProjectHere); err != nil {
				return err
			}
			old, err := s.GetNote(ctx, project, n)
			if err != nil {
				return err
			}
			body, err := bodyText(cmd, args[1:], "note", fmt.Sprintf("%s-note-%d-*.md", project, n), old.Body+"\n")
			if err != nil {
				return err
			}
			note, err := s.UpdateNote(ctx, project, n, body)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, note)
			}
			if note.UpdatedAt.Equal(old.UpdatedAt) {
				fmt.Fprintf(cmd.OutOrStdout(), "No changes to note #%d in %s\n", n, project)
				return nil
			}
			success(cmd.OutOrStdout(), "Updated note #%d in %s", n, project)
			return nil
		},
	}
	addProjectFlag(cmd, &project)
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newNoteDeleteCmd() *cobra.Command {
	var project string
	var yes, asJSON bool
	cmd := &cobra.Command{
		Use:   "delete NUMBER [--yes] [--project NAME]",
		Short: "Delete a note permanently",
		Long: `Delete a note. This can't be undone, so in a terminal it asks first; --yes
skips the question, and is required without a terminal. The numbers of other
notes don't change, and the number isn't reused.`,
		Example: "  taskship note delete 3",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := core.ParseNoteNumber(args[0])
			if err != nil {
				return err
			}
			if !yes && !inputIsTerminal(cmd.InOrStdin()) {
				return usageError{errors.New("deleting a note needs a terminal to confirm; pass --yes to delete it without asking")}
			}
			ctx := cmd.Context()
			s, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if project, err = noteProject(cmd, s, project, errNoProjectHere); err != nil {
				return err
			}
			if _, err := s.GetNote(ctx, project, n); err != nil {
				return err
			}
			if !yes {
				ok, err := newPrompter(cmd).confirm(fmt.Sprintf("Delete note #%d in %s? This can't be undone.", n, project), false)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Nothing deleted.")
					return errDeclined
				}
			}
			note, err := s.DeleteNote(ctx, project, n)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, note)
			}
			success(cmd.OutOrStdout(), "Deleted note #%d from %s", n, project)
			return nil
		},
	}
	addProjectFlag(cmd, &project)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "delete without asking")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
