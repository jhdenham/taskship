package cli

import (
	"fmt"
	"text/tabwriter"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
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
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "RUN\tSTARTED\tAGENT\tSTATE\tEXIT\tCOMPLETED\tFOLDER\tSESSION")
			for _, r := range runs {
				exit := "-"
				if r.ExitCode != nil {
					exit = fmt.Sprint(*r.ExitCode)
				}
				completed := ""
				if r.ID == t.CompletedRunID {
					completed = "✓"
				}
				fmt.Fprintf(w, "#%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Number, r.StartedAt.Local().Format("2006-01-02 15:04"),
					r.Agent, r.State, exit, completed, displayPath(r.WorkspacePath), r.SessionID)
			}
			if err := w.Flush(); err != nil {
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
