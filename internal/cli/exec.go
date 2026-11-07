package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
)

// Exit codes. Codes 5 (backend/auth unavailable) and 6 (agent launch
// failed) are reserved for later commands.
const (
	exitOK           = 0
	exitFailure      = 1
	exitInvalidInput = 2
	exitNotFound     = 3
	exitAmbiguous    = 4
)

// Execute runs the root command cmd with args (without the program name),
// prints any error to stderr, and returns the exit code.
func Execute(cmd *cobra.Command, args []string) int {
	cmd.SetArgs(shortcut(cmd, args))
	err := cmd.Execute()
	if err == nil {
		return exitOK
	}
	if errors.Is(err, errDeclined) {
		return exitFailure
	}
	printError(cmd.ErrOrStderr(), err)
	var usage usageError
	switch {
	case errors.As(err, &usage), errors.Is(err, core.ErrInvalid):
		return exitInvalidInput
	case errors.Is(err, core.ErrNotFound):
		return exitNotFound
	case errors.Is(err, core.ErrAmbiguous):
		return exitAmbiguous
	default:
		return exitFailure
	}
}

// shortcut rewrites "taskship PROJ-42 ..." to "taskship run PROJ-42 ...".
// Only a valid task ref that isn't a command is rewritten, so a mistyped
// command is still an unknown-command error.
func shortcut(root *cobra.Command, args []string) []string {
	if len(args) == 0 {
		return args
	}
	if _, _, err := core.ParseTaskRef(args[0]); err != nil {
		return args
	}
	if cmd, _, err := root.Find(args[:1]); err == nil && cmd != root {
		return args
	}
	return append([]string{"run"}, args...)
}

// kindError is an error with its own message that is classified, for the
// exit code, like the error it wraps.
type kindError struct {
	msg string
	err error
}

func (e kindError) Error() string { return e.msg }
func (e kindError) Unwrap() error { return e.err }

// errDeclined ends a command the user declined to go on with. The command
// has already said what happened, so it isn't printed, but it exits 1.
var errDeclined = errors.New("declined")

// usageError marks bad command-line usage (wrong args, unknown flags).
type usageError struct{ error }

func (e usageError) Unwrap() error { return e.error }

func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(n)(cmd, args); err != nil {
			return usageError{err}
		}
		return nil
	}
}

// homeDir is $TASKSHIP_HOME, or ~/.taskship by default.
func homeDir() (string, error) {
	if h := os.Getenv("TASKSHIP_HOME"); h != "" {
		return h, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".taskship"), nil
}

// openStore opens the local store, creating it on first use. Only call it
// from commands that need stored data, so metadata commands stay side-effect free.
func openStore(ctx context.Context) (*store.Store, error) {
	dir, err := homeDir()
	if err != nil {
		return nil, err
	}
	return store.Open(ctx, dir)
}

func writeJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
