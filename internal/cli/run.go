package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/dispatch"
	"github.com/jhdenham/taskship/internal/workspace"
)

func newRunCmd() *cobra.Command {
	var agent, workspacePath string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "run TASK_REF --dry-run [--agent claude] [--workspace PATH]",
		Short: "Preview an agent run for a task",
		Long: `Prepare a plan-first agent run for a task. "taskship PROJ-42" is short
for "taskship run PROJ-42".

Launching agents is not implemented yet, so --dry-run is required: it prints
the chosen workspace and the exact prompt the agent would receive, and
launches nothing.

The agent runs at the root of a workspace, never in a subfolder. The workspace
is the one containing --workspace if given (it must belong to the task's
project), otherwise the one containing the current folder if that belongs
to the task's project, otherwise the project's only registered workspace. In a
Git worktree of a registered checkout, the agent runs at the worktree's
corresponding folder, usually its top level.`,
		Example: "  taskship PROJ-42 --dry-run\n  taskship PROJ-42 --dry-run --workspace ~/worktrees/myproject/feature",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if agent != "claude" {
				return fmt.Errorf("%w: unknown agent %q (valid: claude)", core.ErrInvalid, agent)
			}
			if !dryRun {
				return errors.New("launching agents is not implemented yet; use --dry-run to preview the run")
			}
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
			dir := workspacePath
			if dir == "" {
				dir = "."
			}
			loc, err := workspace.Inspect(ctx, dir)
			if err != nil {
				return err
			}
			ws, err := s.ListWorkspaces(ctx, "")
			if err != nil {
				return err
			}
			if dir, err = workspace.ForTask(ctx, t, loc, workspacePath != "", ws); err != nil {
				var nf *workspace.NoFolderError
				switch {
				case errors.As(err, &nf) && nf.Missing != "":
					return kindError{fmt.Sprintf(`%s's folder %s no longer exists. If it moved, run "taskship init" in the new location to relink it.`, nf.Project, displayPath(nf.Missing)), err}
				case errors.As(err, &nf):
					return kindError{fmt.Sprintf(`%s isn't linked to a folder on this machine. Run "taskship init" in its repository to link it.`, nf.Project), err}
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Dry run: nothing was launched.\nAgent:     %s\nWorkspace: %s\n\n%s", agent, displayPath(dir), dispatch.BuildPrompt(t))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the workspace and prompt without launching the agent")
	cmd.Flags().StringVarP(&agent, "agent", "a", "claude", "agent to run")
	cmd.Flags().StringVar(&workspacePath, "workspace", "", "checkout or worktree to run in (default: chosen as described above)")
	return cmd
}
