package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/display"
	"github.com/jhdenham/taskship/internal/tui"
	"github.com/jhdenham/taskship/internal/workspace"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the terminal UI (also: taskship with no arguments)",
		Long: `Open a full-screen view of a project's tasks: a board and a list, a detail
view, and everything you can do to a task, with the keyboard or the mouse.
It opens on the current folder's project, or else lists your projects.
Press ? inside for the keys.

"taskship" with no arguments opens it too, in a terminal; without one, it
prints this help instead.`,
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !hasTerminal(cmd) {
				return usageError{errors.New("the terminal UI needs a terminal")}
			}
			return runTUI(cmd)
		},
	}
}

// hasTerminal reports whether cmd reads from and writes to a terminal, as
// the TUI and launching an agent need.
func hasTerminal(cmd *cobra.Command) bool {
	return inputIsTerminal(cmd.InOrStdin()) && isTerminal(cmd.OutOrStdout())
}

// runTUI opens the TUI, and after it closes does what the user chose: set up
// the current folder with init and reopen, or launch an agent. The caller
// has checked for a terminal.
func runTUI(cmd *cobra.Command) error {
	ctx := cmd.Context()
	s, err := openStore(ctx)
	if err != nil {
		return err
	}
	defer s.Close()
	for {
		loc, ws, err := currentDir(ctx, s)
		if err != nil {
			return err
		}
		opts, err := tuiOptions(loc, ws)
		if err != nil {
			return err
		}
		out, err := tui.Run(ctx, s, opts)
		switch {
		case err != nil:
			return err
		case out.Launch != nil:
			return launch(cmd, s, *out.Launch)
		case out.SetUp:
			r := initRun{cmd: cmd, s: s, p: newPrompter(cmd), interactive: true}
			if _, err := r.run(ctx, ".", false); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

// tuiOptions says where the TUI starts in the folder at loc: on its project,
// or in the picker. A folder claimed by several projects opens the picker,
// without offering to set it up, since it already is.
func tuiOptions(loc workspace.Location, ws []core.Workspace) (tui.Options, error) {
	project, _, err := projectAt(loc, ws)
	ambiguous := errors.Is(err, core.ErrAmbiguous)
	if err != nil && !ambiguous {
		return tui.Options{}, err
	}
	return tui.Options{
		Project:  project,
		Folder:   display.Path(loc.Path),
		CanSetUp: !ambiguous && checkInitPath(loc.Path) == nil,
	}, nil
}
