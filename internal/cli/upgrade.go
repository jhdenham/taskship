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

// releaseKey annotates the root command with the version set at release
// build time, which is empty for binaries built from source.
const releaseKey = "release"

// How to update a taskship that upgrade won't replace.
const (
	checkoutHint  = "To update it, pull the latest changes and rebuild it."
	goInstallHint = "Update it with:\ngo install github.com/jhdenham/taskship/cmd/taskship@latest"
)

// updateHint says how to update the running taskship: with upgrade for a
// release build, else the way it was built.
func updateHint(cmd *cobra.Command) string {
	switch {
	case cmd.Root().Annotations[releaseKey] != "":
		return fmt.Sprintf("Run %s to install the latest version.", cmdRef("taskship upgrade"))
	case builtFromCheckout():
		return checkoutHint
	default:
		return goInstallHint
	}
}

func newUpgradeCmd() *cobra.Command {
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
			current := cmd.Root().Annotations[releaseKey]
			if current == "" {
				how := "was installed with go install"
				if builtFromCheckout() {
					how = "was built from a source checkout"
				}
				return kindError{fmt.Sprintf("%s %s, so upgrade won't replace it\n\n%s", display.Path(target), how, updateHint(cmd)), core.ErrInvalid}
			}
			out := cmd.OutOrStdout()
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
		if err := updateSkill(cmd, bin, agent, want); err != nil {
			return fmt.Errorf("taskship was upgraded to %s, but updating the skill for %s failed (%v)\n\nTo update it, run: %s",
				version, core.AgentName(agent), err, command.Render("taskship skill install "+agent))
		}
	}
	return nil
}

// updateSkill has the taskship at bin update agent's skill to want if it's
// installed and outdated.
func updateSkill(cmd *cobra.Command, bin, agent, want string) error {
	dir, err := skill.Dir(agent)
	if err != nil {
		return err
	}
	state, _, err := skill.Status(dir, want)
	if err != nil || state != skill.Outdated {
		return err
	}
	c := exec.CommandContext(cmd.Context(), bin, "skill", "install", agent)
	c.Stdout, c.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
	return c.Run()
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
	n := len(sessions)
	fmt.Fprintf(w, "You have %s that %s to be restarted to use %s:\n",
		plural(n, "a running Taskship session", "running Taskship sessions"), plural(n, "needs", "need"), version)
	rows := make([][2]string, len(sessions))
	for i, s := range sessions {
		rows[i] = [2]string{s.Label, fmt.Sprintf("pid %d", s.PID)}
	}
	columns(w, 3, rows)
}
