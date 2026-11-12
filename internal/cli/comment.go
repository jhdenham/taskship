package cli

import (
	"errors"
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
)

func newCommentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Add, list, edit, and delete task comments",
		Long: `Comments are notes on a task, numbered #1, #2, ... in the order they were
added. They are stored separately from the description: "show --md" and
"export" print them after it, read-only, and the agent's prompt includes them.`,
	}
	cmd.AddCommand(newCommentAddCmd(), newCommentListCmd(), newCommentEditCmd(), newCommentDeleteCmd())
	return cmd
}

func newCommentAddCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "add TASK_REF [TEXT]",
		Short: "Add a comment to a task",
		Long: `Add a comment to a task. The comment is TEXT, or stdin if TEXT is "-".
Without TEXT, it opens $VISUAL or $EDITOR; an empty comment adds nothing.`,
		Example: "  taskship comment add PROJ-1 \"Tried bumping the timeout; still flaky.\"\n" +
			"  taskship comment add PROJ-1\n" +
			"  git log -1 --format=%B | taskship comment add PROJ-1 -",
		Args: rangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			t, err := s.GetTask(ctx, args[0])
			if err != nil {
				return err
			}
			body, err := commentText(cmd, args[1:], t.Ref+"-comment-*.md", "")
			if err != nil {
				return err
			}
			c, err := s.AddComment(ctx, t.Ref, body)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, c)
			}
			success(cmd.OutOrStdout(), "Added comment #%d to %s", c.Number, t.Ref)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newCommentListCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list TASK_REF",
		Short:   "List a task's comments, oldest first",
		Example: "  taskship comment list PROJ-1",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			t, err := s.GetTask(ctx, args[0])
			if err != nil {
				return err
			}
			comments, err := s.ListComments(ctx, t.ID)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, comments)
			}
			if len(comments) == 0 {
				lipgloss.Fprintf(cmd.OutOrStdout(), "No comments on %s yet. Add one with %s.\n", t.Ref, cmdRef("taskship comment add "+t.Ref))
				return nil
			}
			printComments(cmd.OutOrStdout(), comments)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newCommentEditCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "edit TASK_REF NUMBER [TEXT]",
		Short: "Change a comment",
		Long: `Replace the text of a task's comment. The new text is TEXT, or stdin if TEXT
is "-". Without TEXT, the comment opens in $VISUAL or $EDITOR.`,
		Example: "  taskship comment edit PROJ-1 2\n  taskship comment edit PROJ-1 2 \"Fixed in PROJ-4.\"",
		Args:    rangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := core.ParseCommentNumber(args[1])
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			s, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			old, err := s.GetComment(ctx, args[0], n)
			if err != nil {
				return err
			}
			t, err := s.GetTask(ctx, args[0])
			if err != nil {
				return err
			}
			body, err := commentText(cmd, args[2:], fmt.Sprintf("%s-comment-%d-*.md", t.Ref, n), old.Body+"\n")
			if err != nil {
				return err
			}
			c, err := s.UpdateComment(ctx, t.Ref, n, body)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, c)
			}
			if c.UpdatedAt.Equal(old.UpdatedAt) {
				fmt.Fprintf(cmd.OutOrStdout(), "No changes to comment #%d on %s\n", n, t.Ref)
				return nil
			}
			success(cmd.OutOrStdout(), "Updated comment #%d on %s", n, t.Ref)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newCommentDeleteCmd() *cobra.Command {
	var yes, asJSON bool
	cmd := &cobra.Command{
		Use:   "delete TASK_REF NUMBER [--yes]",
		Short: "Delete a comment permanently",
		Long: `Delete a task's comment. This can't be undone, so in a terminal it asks
first; --yes skips the question, and is required without a terminal. The
numbers of other comments don't change, and the number isn't reused.`,
		Example: "  taskship comment delete PROJ-1 2",
		Args:    exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := core.ParseCommentNumber(args[1])
			if err != nil {
				return err
			}
			if !yes && !inputIsTerminal(cmd.InOrStdin()) {
				return usageError{errors.New("deleting a comment needs a terminal to confirm; pass --yes to delete it without asking")}
			}
			ctx := cmd.Context()
			s, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			c, err := s.GetComment(ctx, args[0], n)
			if err != nil {
				return err
			}
			t, err := s.GetTask(ctx, args[0])
			if err != nil {
				return err
			}
			if !yes {
				ok, err := newPrompter(cmd).confirm(fmt.Sprintf("Delete comment #%d on %s? This can't be undone.", n, t.Ref), false)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Nothing deleted.")
					return errDeclined
				}
			}
			if c, err = s.DeleteComment(ctx, t.Ref, n); err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, c)
			}
			success(cmd.OutOrStdout(), "Deleted comment #%d from %s", n, t.Ref)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "delete without asking")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

// commentText returns a comment's text: args[0] if given, stdin if it is
// "-", or else what the user writes in their editor, starting from initial.
// An empty comment from the editor ends the command with errDeclined.
func commentText(cmd *cobra.Command, args []string, pattern, initial string) (string, error) {
	if len(args) == 1 && args[0] == "-" {
		b, err := io.ReadAll(cmd.InOrStdin())
		return string(b), err
	}
	if len(args) == 1 {
		return args[0], nil
	}
	if !inputIsTerminal(cmd.InOrStdin()) {
		return "", usageError{errors.New(`writing a comment in the editor needs a terminal; give the text as an argument, or "-" to read stdin`)}
	}
	draft, src, err := editText(cmd, pattern, initial)
	if err != nil {
		return "", err
	}
	removeDraft(draft)
	if body, err := core.CleanCommentBody(string(src)); err == nil {
		return body, nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), "The comment is empty, so nothing changed.")
	return "", errDeclined
}

// printComments prints comments as a heading line ("#1 · 2026-11-12 14:03")
// followed by the body, with a blank line between comments.
func printComments(w io.Writer, comments []core.Comment) {
	for i, c := range comments {
		if i > 0 {
			fmt.Fprintln(w)
		}
		lipgloss.Fprintln(w, heading.Render(core.CommentHeading(c)))
		fmt.Fprintln(w, c.Body)
	}
}
