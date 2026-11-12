// Package cli defines the taskship command tree.
package cli

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// NewRootCmd returns the root taskship command. An empty version falls back to
// the module version embedded by `go install`, then to "dev".
func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "taskship",
		Short:         "From task to agent, in one command.",
		Example:       "  taskship PROJ-42   Launch Claude on a task, plan-first (short for: taskship run PROJ-42)",
		Version:       resolveVersion(version),
		SilenceUsage:  true,
		SilenceErrors: true, // Execute prints errors and picks the exit code
		// Bare "taskship" opens the TUI in a terminal, and prints help
		// anywhere else.
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !hasTerminal(cmd) {
				return cmd.Help()
			}
			return runTUI(cmd)
		},
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError{err}
	})
	root.AddCommand(
		newProjectCmd(),
		newTaskCmd(),
		newTaskAddCmd("add"),
		newTaskListCmd("tasks"),
		newTaskShowCmd("show"),
		newCommentCmd(),
		newExportCmd(),
		newWorkspaceCmd(),
		newRunCmd(),
		newRunsCmd(),
		newInitCmd(),
		newTUICmd(),
	)
	return root
}

func resolveVersion(v string) string {
	if v != "" {
		return v
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
