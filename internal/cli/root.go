// Package cli defines the taskship command tree.
package cli

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// NewRootCmd returns the root taskship command. An empty version falls back to
// the module version embedded by `go install`, then to "dev".
func NewRootCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:          "taskship",
		Short:        "From task to agent, in one command.",
		Version:      resolveVersion(version),
		SilenceUsage: true,
	}
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
