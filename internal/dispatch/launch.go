package dispatch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// maxPromptBytes keeps the prompt within Linux's limit on a single argument
// (128 KiB including the terminating NUL).
const maxPromptBytes = 128<<10 - 1

// ClaudeArgs returns the arguments that start Claude Code interactively in
// plan mode with prompt as the first message. Plan mode makes Claude ask for
// approval of its plan before it edits files. "--" keeps a prompt that starts
// with "-" from being read as an option.
func ClaudeArgs(sessionID, prompt string) ([]string, error) {
	if len(prompt) > maxPromptBytes {
		return nil, fmt.Errorf("the prompt is %d KB, more than the %d KB an agent can be given; shorten the task's description, its comments, or your conventions",
			len(prompt)>>10, (maxPromptBytes+1)>>10)
	}
	return []string{"--permission-mode", "plan", "--session-id", sessionID, "--", prompt}, nil
}

// Launch runs the agent executable at path with args and environment env in
// dir, attached to this process's terminal, and returns its exit code once it exits (128 plus
// the signal number if a signal ended it). started is called once the
// process is running. The error is non-nil only if the agent couldn't be
// started or waited for.
//
// While the agent runs, this process doesn't exit on signals, so it can
// record how the run ended. Ctrl-C and Ctrl-\ are left to the agent: the
// terminal sends them to it directly. A hang-up (the terminal closed) or a
// terminate request is passed on to the agent, which then exits.
func Launch(path string, args, env []string, dir string, started func()) (int, error) {
	cmd := exec.Command(path, args...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	// Notify, not Ignore: ignored signals would stay ignored in the agent.
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, os.Interrupt, syscall.SIGQUIT, syscall.SIGHUP, syscall.SIGTERM)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case sig := <-sigs:
				if sig == syscall.SIGHUP || sig == syscall.SIGTERM {
					cmd.Process.Signal(sig)
				}
			case <-done:
				return
			}
		}
	}()
	started()
	err := cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()), nil
		}
		return exitErr.ExitCode(), nil
	}
	return 0, err
}
