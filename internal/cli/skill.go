package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/guide"
)

func newSkillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Teach coding agents to use Taskship",
		Long: `The Taskship skill teaches a coding agent to read, write, refine, and report
on tasks in any session, not only the ones Taskship launches.`,
	}
	cmd.AddCommand(newSkillGuideCmd())
	return cmd
}

func newSkillGuideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guide",
		Short: "Print the guide agents follow when writing tasks",
		Long: `Print Taskship's guide to writing tasks, followed by your own conventions
from ~/.taskship/conventions.md, if that file exists. The skill tells agents
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
