package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/display"
	"github.com/jhdenham/taskship/internal/guide"
	"github.com/jhdenham/taskship/internal/skill"
)

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Teach coding agents to use Taskship",
		Long: `The Taskship skill teaches a coding agent to read, write, refine, and report
on tasks in any session, not only the ones Taskship launches. It's optional:
"taskship run" and "taskship refine" give the agent everything it needs.

The skill is installed in the agent's own folder for skills, never in a
repository: ~/.claude/skills/taskship for Claude ($CLAUDE_CONFIG_DIR/skills
if set) and ~/.agents/skills/taskship for Codex.`,
	}
	cmd.AddCommand(newSkillInstallCmd(), newSkillListCmd(), newSkillUninstallCmd(), newSkillGuideCmd())
	return cmd
}

func newSkillInstallCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "install AGENT|all",
		Short: "Install or update the Taskship skill for an agent",
		Long: `Install the Taskship skill for an agent (claude or codex), or update it to
this version of taskship. "all" installs it for every supported agent on
your PATH. Updating replaces any edits made to the skill; put your own
conventions in ~/.taskship/conventions.md ($TASKSHIP_HOME/conventions.md if
set) instead (see "taskship skill guide"). A skill installed by a newer
taskship is left alone.

The skill runs "taskship", so taskship must be on your PATH.

If the agent already has a skill named taskship that Taskship didn't
install, install asks before replacing it, and needs --yes without a
terminal.`,
		Example: "  taskship skill install claude\n  taskship skill install all",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("taskship"); err != nil {
				return kindError{"taskship isn't on your PATH. The skill runs it as \"taskship\", so add it to your PATH first.", core.ErrNotFound}
			}
			agents := args
			if args[0] == "all" {
				agents = nil
				for _, agent := range skill.Agents {
					if _, err := exec.LookPath(agent); err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "Skipped %s: %s isn't on your PATH.\n", core.AgentName(agent), agent)
						continue
					}
					agents = append(agents, agent)
				}
				if len(agents) == 0 {
					return kindError{fmt.Sprintf("No supported agents found on your PATH (%s). Install one, or name it: %s",
						strings.Join(skill.Agents, ", "), cmdRef("taskship skill install claude")), core.ErrNotFound}
				}
			}
			want := skill.Render(cmd.Root().Version)
			var errs []error
			fresh := false
			for _, agent := range agents {
				installed, err := installSkill(cmd, agent, want, yes)
				fresh = fresh || installed
				errs = append(errs, err)
			}
			if fresh {
				fmt.Fprintln(cmd.OutOrStdout(), "If an open session doesn't see it, restart it.")
			}
			return errors.Join(errs...)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "replace a skill Taskship didn't install without asking")
	return cmd
}

// installSkill installs or updates the skill want for agent, asking before
// replacing a skill Taskship didn't install unless yes is set. It reports
// whether the skill is newly installed. Declining isn't an error.
func installSkill(cmd *cobra.Command, agent, want string, yes bool) (bool, error) {
	dir, err := skill.Dir(agent)
	if err != nil {
		return false, err
	}
	state, version, err := skill.Status(dir, want)
	if err != nil {
		return false, err
	}
	name, out := core.AgentName(agent), cmd.OutOrStdout()
	switch state {
	case skill.Installed:
		fmt.Fprintf(out, "The Taskship skill for %s is up to date.\n", name)
		return false, nil
	case skill.Newer:
		fmt.Fprintf(out, "The Taskship skill for %s is from a newer taskship (%s), so it was left alone.\n", name, version)
		return false, nil
	}
	if state == skill.NotOurs && !yes {
		if !inputIsTerminal(cmd.InOrStdin()) {
			return false, usageError{fmt.Errorf("%s has a skill Taskship didn't install; pass --yes to replace it", display.Path(dir))}
		}
		ok, err := newPrompter(cmd).confirm(fmt.Sprintf("%s has a skill Taskship didn't install. Replace it?", pathRef(dir)), false)
		if err != nil {
			return false, err
		}
		if !ok {
			fmt.Fprintf(out, "Nothing changed for %s.\n", name)
			return false, nil
		}
	}
	if err := skill.Install(dir, want, state); err != nil {
		return false, err
	}
	switch state {
	case skill.Outdated:
		success(out, "Updated the Taskship skill for %s (%s → %s)", name, version, cmd.Root().Version)
	case skill.Modified:
		success(out, "Updated the Taskship skill for %s, replacing its edits", name)
	default:
		success(out, "Installed the Taskship skill for %s in %s", name, pathRef(dir))
		return true, nil
	}
	return false, nil
}

func newSkillListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show where the Taskship skill is installed",
		Long: `Show the Taskship skill's state for each agent: installed, outdated (from
an older taskship), newer (from a newer taskship), modified (edited since it
was installed), not installed, or not ours (a skill named taskship that
Taskship didn't install).`,
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			want := skill.Render(cmd.Root().Version)
			rows := [][]cell{header("AGENT", "SKILL", "VERSION", "FOLDER")}
			for _, agent := range skill.Agents {
				dir, err := skill.Dir(agent)
				if err != nil {
					return err
				}
				state, version, err := skill.Status(dir, want)
				if err != nil {
					return err
				}
				rows = append(rows, []cell{{text: agent}, {text: string(state)}, {text: version}, {display.Path(dir), &faint}})
			}
			return table(cmd.OutOrStdout(), rows)
		},
	}
}

func newSkillUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall AGENT|all",
		Short: "Remove the Taskship skill from an agent",
		Long: `Remove the Taskship skill from an agent (claude or codex), or with "all"
from every agent it's installed for. Only the file Taskship installed is
removed; a skill named taskship that Taskship didn't install is left alone.`,
		Example: "  taskship skill uninstall claude\n  taskship skill uninstall all",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if args[0] != "all" {
				dir, err := skill.Dir(args[0])
				if err != nil {
					return err
				}
				removed, err := skill.Uninstall(dir)
				switch {
				case errors.Is(err, skill.ErrNotOurs):
					return fmt.Errorf("%s has a skill Taskship didn't install, so it was left alone", display.Path(dir))
				case err != nil:
					return err
				case !removed:
					fmt.Fprintf(out, "The Taskship skill isn't installed for %s.\n", core.AgentName(args[0]))
				default:
					success(out, "Removed the Taskship skill for %s from %s", core.AgentName(args[0]), pathRef(dir))
				}
				return nil
			}
			said := false
			for _, agent := range skill.Agents {
				dir, err := skill.Dir(agent)
				if err != nil {
					return err
				}
				removed, err := skill.Uninstall(dir)
				switch {
				case errors.Is(err, skill.ErrNotOurs):
					lipgloss.Fprintf(out, "Left %s alone: Taskship didn't install the skill there.\n", pathRef(dir))
				case err != nil:
					return err
				case removed:
					success(out, "Removed the Taskship skill for %s from %s", core.AgentName(agent), pathRef(dir))
				default:
					continue
				}
				said = true
			}
			if !said {
				fmt.Fprintln(out, "The Taskship skill isn't installed for any agent.")
			}
			return nil
		},
	}
}

func newSkillGuideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guide",
		Short: "Print the guide agents follow when writing tasks",
		Long: `Print Taskship's guide to writing tasks, followed by your own conventions
from ~/.taskship/conventions.md ($TASKSHIP_HOME/conventions.md if set), if
that file exists. The skill tells agents
to read this before they write or refine a task. Your conventions take
precedence over the guide, and they are also added to the prompt of every
agent Taskship launches.`,
		Args: exactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			conventions, path, err := guide.Conventions()
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), guide.Text(conventions, path))
			return nil
		},
	}
}
