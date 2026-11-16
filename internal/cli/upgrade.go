package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/skill"
	"github.com/jhdenham/taskship/internal/upgrade"
)

// executable returns the running binary's path; tests replace it.
var executable = os.Executable

// newUpgradeCmd returns the upgrade command. release is the version set at
// release build time, which is empty for binaries built from source.
func newUpgradeCmd(release string) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Install the latest release of taskship",
		Long: `Check for a newer release of taskship and, after asking, install it in
place of this one. The download is checked against the release's checksums
and run once before it replaces anything. Then the new taskship updates the
Taskship skill for each agent where it's installed and outdated.

upgrade asks before changing anything, and needs --yes without a terminal.
A taskship built from source isn't replaced: update it with go install.`,
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if release == "" {
				return kindError{"This taskship was built from source, so upgrade won't replace it.\n\nUpdate it with:\ngo install github.com/jhdenham/taskship/cmd/taskship@latest", core.ErrInvalid}
			}
			out, current := cmd.OutOrStdout(), cmd.Root().Version
			latest, err := upgrade.Latest(cmd.Context())
			if err != nil {
				return err
			}
			if !core.VersionNewer(latest, current) {
				fmt.Fprintf(out, "taskship %s is up to date.\n", current)
				return nil
			}
			fmt.Fprintf(out, "Current: %s\nLatest:  %s\n", current, latest)
			if !yes {
				if !inputIsTerminal(cmd.InOrStdin()) {
					return usageError{errors.New("pass --yes to upgrade without a terminal")}
				}
				ok, err := newPrompter(cmd).confirm("Upgrade?", false)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(out, "Nothing changed.")
					return nil
				}
			}
			exe, err := executable()
			if err != nil {
				return err
			}
			target, err := filepath.EvalSymlinks(exe)
			if err != nil {
				return err
			}
			if _, err := upgrade.Install(cmd.Context(), latest, target); err != nil {
				return fmt.Errorf("%w; taskship %s is still installed", err, current)
			}
			success(out, "Upgraded taskship %s → %s", current, latest)
			return updateSkills(cmd, target, latest)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "upgrade without asking")
	return cmd
}

// updateSkills has the new taskship at bin update the skill for each agent
// where it's installed and older than version. This taskship can't do it:
// it would write its own, older skill.
func updateSkills(cmd *cobra.Command, bin, version string) error {
	want := skill.Render(version)
	for _, agent := range skill.Agents {
		dir, err := skill.Dir(agent)
		if err != nil {
			return err
		}
		state, _, err := skill.Status(dir, want)
		if err != nil {
			return err
		}
		if state != skill.Outdated {
			continue
		}
		c := exec.CommandContext(cmd.Context(), bin, "skill", "install", agent)
		c.Stdout, c.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
		if err := c.Run(); err != nil {
			return fmt.Errorf("taskship was upgraded, but updating the skill for %s failed (%v). Run: %s",
				core.AgentName(agent), err, cmdRef("taskship skill install "+agent))
		}
	}
	return nil
}
