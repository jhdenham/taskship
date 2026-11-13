package dispatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/google/uuid"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/guide"
	"github.com/jhdenham/taskship/internal/store"
	"github.com/jhdenham/taskship/internal/workspace"
)

// RunIDEnv names the environment variable that tells taskship commands run by
// an agent which run they belong to. The store records it only for a running
// run of the same task, so a stale or unrelated value is ignored.
const RunIDEnv = "TASKSHIP_RUN_ID"

// ErrAgentMissing reports that the agent's executable isn't on PATH.
var ErrAgentMissing = errors.New("agent not installed")

// StartError reports that the agent's process couldn't be started.
type StartError struct{ Err error }

func (e *StartError) Error() string { return e.Err.Error() }
func (e *StartError) Unwrap() error { return e.Err }

// Plan is a prepared agent run: the task, what the run is for, the folder
// the agent runs in, and its prompt. Ready fills in the executable and its
// arguments.
type Plan struct {
	Task   core.Task
	Mode   core.RunMode
	Agent  string
	Dir    string
	Prompt string
	// Allowed are the Bash commands the agent may run without asking (see
	// AllowedCommands).
	Allowed []string

	path, sessionID string
	args            []string
}

// Prepare chooses the folder to run task t in (see workspace.ForTask; dir is
// the folder to start from, explicit if the user gave it) and builds the
// prompt for mode from the task, its comments, and the user's conventions.
func Prepare(ctx context.Context, s *store.Store, t core.Task, mode core.RunMode, dir string, explicit bool) (Plan, error) {
	loc, err := workspace.Inspect(ctx, dir)
	if err != nil {
		return Plan{}, err
	}
	ws, err := s.ListWorkspaces(ctx, "")
	if err != nil {
		return Plan{}, err
	}
	if dir, err = workspace.ForTask(ctx, t, loc, explicit, ws); err != nil {
		return Plan{}, err
	}
	comments, err := s.ListComments(ctx, t.ID)
	if err != nil {
		return Plan{}, err
	}
	conventions, _, err := guide.Conventions()
	if err != nil {
		return Plan{}, err
	}
	build := BuildPrompt
	if mode == core.RunModeRefine {
		build = BuildRefinePrompt
	}
	taskship := TaskshipCommand()
	return Plan{Task: t, Mode: mode, Agent: "claude", Dir: dir, Prompt: build(t, comments, taskship, conventions),
		Allowed: AllowedCommands(mode, t.Ref, taskship)}, nil
}

// Ready checks that the agent can be launched: its executable is on PATH
// (else ErrAgentMissing) and the prompt fits in an argument.
func (p *Plan) Ready() error {
	path, err := exec.LookPath("claude")
	if err != nil {
		return ErrAgentMissing
	}
	sessionID := uuid.NewString()
	args, err := ClaudeArgs(sessionID, p.Prompt, p.Allowed)
	if err != nil {
		return fmt.Errorf("%w: %w", core.ErrInvalid, err)
	}
	p.path, p.sessionID, p.args = path, sessionID, args
	return nil
}

// Execute runs a Ready plan in this terminal and returns the agent's exit
// code. To work on a task (not to refine it), a task in backlog or todo
// moves to in_progress first, and back if the agent can't be started (a
// *StartError). The run is recorded from
// before the start to the end. starting is called just before the launch,
// with whether the task moved; warn receives problems that don't stop the
// run, such as a run that couldn't be recorded.
func Execute(ctx context.Context, s *store.Store, p Plan, starting func(moved bool), warn func(error)) (int, error) {
	t := p.Task
	moved := p.Mode == core.RunModeRun && (t.Status == core.StatusBacklog || t.Status == core.StatusTodo)
	setStatus := func(st core.Status) error {
		_, err := s.UpdateTask(ctx, t.Ref, store.TaskPatch{Status: &st})
		return err
	}
	restoreStatus := func() {
		if moved {
			if err := setStatus(t.Status); err != nil {
				warn(fmt.Errorf("couldn't move %s back to %s: %w", t.Ref, t.Status, err))
			}
		}
	}
	if moved {
		if err := setStatus(core.StatusInProgress); err != nil {
			return 0, err
		}
	}
	machine, _ := os.Hostname()
	r, err := s.CreateRun(ctx, core.Run{TaskID: t.ID, TaskRef: t.Ref, Agent: p.Agent, Mode: p.Mode,
		Machine: machine, WorkspacePath: p.Dir, SessionID: p.sessionID})
	if err != nil {
		restoreStatus()
		return 0, err
	}
	starting(moved)
	var recordErr error
	env := append(os.Environ(), RunIDEnv+"="+r.ID)
	code, err := Launch(p.path, p.args, env, p.Dir, func() { recordErr = s.StartRun(ctx, r.ID) })
	if err != nil {
		if ferr := s.FinishRun(ctx, r.ID, nil); ferr != nil {
			warn(fmt.Errorf("couldn't record the failed run: %w", ferr))
		}
		restoreStatus()
		return 0, &StartError{err}
	}
	if err := s.FinishRun(ctx, r.ID, &code); err != nil && recordErr == nil {
		recordErr = err
	}
	if recordErr != nil {
		warn(fmt.Errorf("Claude ran, but the run wasn't recorded: %w", recordErr))
	}
	return code, nil
}
