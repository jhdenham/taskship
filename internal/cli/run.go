package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/dispatch"
	"github.com/jhdenham/taskship/internal/store"
	"github.com/jhdenham/taskship/internal/workspace"
)

func newRunCmd() *cobra.Command {
	var agent, workspacePath string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "run TASK_REF [--agent claude] [--workspace PATH] [--dry-run]",
		Short: "Launch an agent on a task, plan-first",
		Long: `Launch Claude Code on a task in this terminal. "taskship PROJ-42" is short
for "taskship run PROJ-42".

A task in backlog or todo moves to in_progress. Claude starts in plan mode
with the task as its first message: it inspects the repository, assesses the
task, and proposes a plan, and it asks you to approve the plan before it edits
files. Each launch is recorded as a run
("taskship runs PROJ-42"). When Claude exits, taskship exits with its exit
code.

--dry-run prints the chosen workspace and the exact prompt the agent would
receive, and launches nothing.

The agent runs at the root of a workspace, never in a subfolder. The workspace
is the one containing --workspace if given (it must belong to the task's
project), otherwise the one containing the current folder if that belongs
to the task's project, otherwise the project's only registered workspace. In a
Git worktree of a registered checkout, the agent runs at the worktree's
corresponding folder, usually its top level.`,
		Example: "  taskship PROJ-42\n  taskship PROJ-42 --dry-run\n  taskship PROJ-42 --workspace ~/worktrees/myproject/feature",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if agent != "claude" {
				return fmt.Errorf("%w: unknown agent %q (valid: claude)", core.ErrInvalid, agent)
			}
			if !dryRun && (!inputIsTerminal(cmd.InOrStdin()) || !isTerminal(cmd.OutOrStdout())) {
				return usageError{errors.New("launching an agent needs a terminal; use --dry-run to preview the run")}
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
				if !errors.As(err, &nf) {
					return err
				}
				if nf.Missing != "" {
					return kindError{fmt.Sprintf("%s's folder %s no longer exists. If it moved, run %s in the new location to relink it.", value.Render(nf.Project), pathRef(nf.Missing), cmdRef("taskship init")), err}
				}
				return kindError{fmt.Sprintf("%s isn't linked to a folder on this machine. Run %s in its repository to link it.", value.Render(nf.Project), cmdRef("taskship init")), err}
			}
			comments, err := s.ListComments(ctx, t.ID)
			if err != nil {
				return err
			}
			prompt := dispatch.BuildPrompt(t, comments, dispatch.TaskshipCommand())
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: nothing was launched.\nAgent:     %s\nWorkspace: %s\n\n%s", agent, displayPath(dir), prompt)
				return nil
			}

			path, err := exec.LookPath("claude")
			if err != nil {
				return kindError{"Claude Code isn't installed, or isn't on your PATH. Install it from https://claude.com/claude-code and try again.", errLaunch}
			}
			sessionID := uuid.NewString()
			agentArgs, err := dispatch.ClaudeArgs(sessionID, prompt)
			if err != nil {
				return fmt.Errorf("%w: %w", core.ErrInvalid, err)
			}
			// Launching starts work on a task that hasn't started yet. If the
			// agent can't be started, the status goes back.
			moved := t.Status == core.StatusBacklog || t.Status == core.StatusTodo
			setStatus := func(st core.Status) error {
				_, err := s.UpdateTask(ctx, t.Ref, store.TaskPatch{Status: &st})
				return err
			}
			restoreStatus := func() {
				if moved {
					if err := setStatus(t.Status); err != nil {
						printError(cmd.ErrOrStderr(), fmt.Errorf("couldn't move %s back to %s: %w", t.Ref, t.Status, err))
					}
				}
			}
			if moved {
				if err := setStatus(core.StatusInProgress); err != nil {
					return err
				}
			}
			machine, _ := os.Hostname()
			r, err := s.CreateRun(ctx, core.Run{TaskID: t.ID, TaskRef: t.Ref, Agent: agent,
				Machine: machine, WorkspacePath: dir, SessionID: sessionID})
			if err != nil {
				restoreStatus()
				return err
			}
			if moved {
				success(cmd.OutOrStdout(), "Moved %s from %s to %s", t.Ref, t.Status, core.StatusInProgress)
			}
			success(cmd.OutOrStdout(), "Launching Claude for %s in %s", t.Ref, pathRef(dir))
			var recordErr error
			code, err := dispatch.Launch(path, agentArgs, dir, func() { recordErr = s.StartRun(ctx, r.ID) })
			if err != nil {
				if ferr := s.FinishRun(ctx, r.ID, nil); ferr != nil {
					printError(cmd.ErrOrStderr(), fmt.Errorf("couldn't record the failed run: %w", ferr))
				}
				restoreStatus()
				return kindError{fmt.Sprintf("Couldn't start Claude: %v", err), errLaunch}
			}
			if err := s.FinishRun(ctx, r.ID, &code); err != nil && recordErr == nil {
				recordErr = err
			}
			if recordErr != nil {
				printError(cmd.ErrOrStderr(), fmt.Errorf("Claude ran, but the run wasn't recorded: %w", recordErr))
			}
			if code != 0 {
				return agentExitError(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the workspace and prompt without launching the agent")
	cmd.Flags().StringVarP(&agent, "agent", "a", "claude", "agent to run")
	cmd.Flags().StringVar(&workspacePath, "workspace", "", "checkout or worktree to run in (default: chosen as described above)")
	return cmd
}
