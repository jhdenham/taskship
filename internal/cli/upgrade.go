package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/display"
	"github.com/jhdenham/taskship/internal/home"
	"github.com/jhdenham/taskship/internal/session"
	"github.com/jhdenham/taskship/internal/skill"
	"github.com/jhdenham/taskship/internal/upgrade"
)

// executable returns the running binary's path; tests replace it.
var executable = os.Executable

// builtFromCheckout reports whether this binary was built in a source
// checkout, rather than installed with "go install ...@version", which
// records no commit. Tests replace it.
var builtFromCheckout = func() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return true
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return true
		}
	}
	return info.Main.Version == "" || info.Main.Version == "(devel)"
}

// newUpgradeCmd returns the upgrade command. release is the version set at
// release build time, which is empty for binaries built from source.
func newUpgradeCmd(release string) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Install the latest release of taskship",
		Long: `Check for a newer release of taskship and, after asking, install it in
place of the running one. The download is checked against the release's
checksums and run once before it replaces anything. Then the new version
updates the Taskship skill for each agent where it's installed and
outdated, and lists any running Taskship sessions to restart.

upgrade asks before changing anything, and needs --yes without a terminal.
A taskship built from source isn't replaced: update it with go install, or
rebuild it from your checkout.`,
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			exe, err := executable()
			if err != nil {
				return err
			}
			target, err := filepath.EvalSymlinks(exe)
			if err != nil {
				return err
			}
			if release == "" {
				if builtFromCheckout() {
					return kindError{fmt.Sprintf("%s was built from a source checkout, so upgrade won't replace it\n\nTo update it, pull the latest changes and rebuild it.", display.Path(target)), core.ErrInvalid}
				}
				return kindError{fmt.Sprintf("%s was installed with go install, so upgrade won't replace it\n\nUpdate it with:\ngo install github.com/jhdenham/taskship/cmd/taskship@latest", display.Path(target)), core.ErrInvalid}
			}
			out, current := cmd.OutOrStdout(), cmd.Root().Version
			latest, notes, err := upgrade.Latest(cmd.Context())
			if err != nil {
				return err
			}
			if !core.VersionNewer(latest, current) {
				fmt.Fprintf(out, "taskship %s is up to date.\n", current)
				return nil
			}
			fmt.Fprintf(out, "Current: %s\nLatest:  %s\nRelease notes: %s\n", current, latest, notes)
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

			fmt.Fprintf(out, "Downloading taskship %s…\n", latest)
			// From here, Ctrl+C stops the download and cleans up instead of
			// leaving a partial file next to the binary.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			err = upgrade.Install(ctx, latest, target)
			stop()
			if err != nil {
				unchanged := fmt.Sprintf("Nothing changed: taskship %s is still installed.", current)
				var notWritable *upgrade.NotWritableError
				switch {
				case errors.Is(err, context.Canceled):
					return fmt.Errorf("upgrade interrupted\n\n%s", unchanged)
				case errors.As(err, &notWritable):
					return fmt.Errorf("can't write to %s, where taskship is installed\n\n%s Upgrade it the way you installed it there, or reinstall in a folder you own with:\ncurl -fsSL https://taskship.dev/install.sh | sh",
						display.Path(notWritable.Dir), unchanged)
				}
				return fmt.Errorf("%w\n\n%s", err, unchanged)
			}
			success(out, "Upgraded taskship %s → %s", current, latest)
			err = updateSkills(cmd, target, latest)
			printSessions(out, latest)
			return err
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "upgrade without asking")
	return cmd
}

// updateSkills has the new taskship at bin update the skill for each agent
// where it's installed and older than version. The running taskship can't
// do it: it would write its own, older skill.
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
			return fmt.Errorf("taskship was upgraded to %s, but updating the skill for %s failed (%v)\n\nTo update it, run: %s",
				version, core.AgentName(agent), err, command.Render("taskship skill install "+agent))
		}
	}
	return nil
}

// startSession records cmd as a running session, labelled with its command
// and ref, so that upgrade can list it. The caller defers the returned
// function. A session that can't be recorded, as in an agent's sandbox,
// runs anyway.
func startSession(cmd *cobra.Command, ref string) func() {
	label := cmd.CommandPath()
	if ref != "" {
		label += " " + ref
	}
	if dir, err := home.Dir(); err == nil {
		if stop, err := session.Start(dir, label); err == nil {
			return stop
		}
	}
	return func() {}
}

// printSessions lists the running sessions that need a restart to use
// version, if there are any.
func printSessions(w io.Writer, version string) {
	dir, err := home.Dir()
	if err != nil {
		return
	}
	sessions, err := session.Open(dir)
	if err != nil || len(sessions) == 0 {
		return
	}
	if len(sessions) == 1 {
		fmt.Fprintf(w, "You have a running Taskship session that needs to be restarted to use %s:\n", version)
	} else {
		fmt.Fprintf(w, "You have running Taskship sessions that need to be restarted to use %s:\n", version)
	}
	rows := make([][2]string, len(sessions))
	for i, s := range sessions {
		rows[i] = [2]string{s.Label, fmt.Sprintf("pid %d", s.PID)}
	}
	columns(w, 3, rows)
}
