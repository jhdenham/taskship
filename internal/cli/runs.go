package cli

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/display"
)

func newRunsCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "runs TASK_REF [--json]",
		Short: "List a task's agent runs",
		Long: `List the agent runs launched for a task on this machine, newest first.

Runs are numbered per task (#1, #2, ...). A run is "running" while its agent
is open, "ended" with the agent's exit code once it exits, and "failed" if the
agent couldn't be started. COMPLETED marks the run whose agent marked the task
done. Resume a
Claude session with "claude --resume SESSION" from the run's folder.`,
		Example: "  taskship runs PROJ-42",
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
			runs, err := s.ListRuns(ctx, t.ID)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, runs)
			}
			out := cmd.OutOrStdout()
			if len(runs) == 0 {
				lipgloss.Fprintf(out, "No runs for %s yet. Run %s to launch Claude.\n", t.Ref, cmdRef("taskship "+t.Ref))
				return nil
			}
			rows := [][]cell{header("RUN", "STARTED", "AGENT", "STATE", "EXIT", "COMPLETED", "FOLDER", "SESSION")}
			for _, r := range runs {
				exit := "-"
				if r.ExitCode != nil {
					exit = fmt.Sprint(*r.ExitCode)
				}
				completed := ""
				if r.ID == t.CompletedRunID {
					completed = "✓"
				}
				rows = append(rows, []cell{{text: fmt.Sprintf("#%d", r.Number)}, {text: r.StartedAt.Local().Format("2006-01-02 15:04")},
					{text: r.Agent}, {string(r.State), stateStyle(r)}, {text: exit}, {text: completed},
					{display.Path(r.WorkspacePath), &faint}, {r.SessionID, &faint}})
			}
			if err := table(out, rows); err != nil {
				return err
			}
			if isTerminal(out) {
				lipgloss.Fprintf(out, "\nTo resume a session, run %s in its folder.\n", cmdRef("claude --resume SESSION"))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

// stateStyle colors a run's state: accent while it is starting or running,
// green if the agent exited 0, red if it failed or exited non-zero.
func stateStyle(r core.Run) *lipgloss.Style {
	switch {
	case r.State == core.RunFailed, r.State == core.RunEnded && (r.ExitCode == nil || *r.ExitCode != 0):
		return &failStyle
	case r.State == core.RunEnded:
		return &doneStyle
	default:
		return &value
	}
}
