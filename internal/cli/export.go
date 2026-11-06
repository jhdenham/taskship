package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
)

func newExportCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "export DIR [--project NAME]",
		Short: "Write tasks to a folder as Markdown files",
		Long: `Write each unarchived task to DIR/REF.md (for example DIR/PROJ-1.md) in the
format shown by "taskship show --md". DIR is created if needed, and existing
files with the same names are overwritten. The export is a snapshot: edits to
the files do not change the tasks.`,
		Example: "  taskship export ~/taskship-export --project myproject",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			tasks, err := s.ListTasks(cmd.Context(), store.TaskFilter{Project: project})
			if err != nil {
				return err
			}
			dir := args[0]
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			for _, t := range tasks {
				if err := os.WriteFile(filepath.Join(dir, t.Ref+".md"), []byte(core.TaskMarkdown(t)), 0o644); err != nil {
					return err
				}
			}
			noun := "tasks"
			if len(tasks) == 1 {
				noun = "task"
			}
			success(cmd.OutOrStdout(), "Exported %d %s to %s", len(tasks), noun, displayPath(dir))
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "only tasks in this project")
	return cmd
}
