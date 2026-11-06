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
		Example:       "  taskship PROJ-42 --dry-run   Preview an agent run for a task (short for: taskship run PROJ-42 --dry-run)",
		Version:       resolveVersion(version),
		SilenceUsage:  true,
		SilenceErrors: true, // Execute prints errors and picks the exit code
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
		newExportCmd(),
		newWorkspaceCmd(),
		newRunCmd(),
		newInitCmd(),
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
