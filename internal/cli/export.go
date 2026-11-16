package cli

import (
	"fmt"
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
		Short: "Write tasks and notes to a folder as Markdown files",
		Long: `Write each unarchived task to DIR/REF.md (for example DIR/PROJ-1.md) in the
format shown by "taskship show --md", including comments, and the notes of
each unarchived project, newest first, to DIR/notes/PROJECT.md. DIR is created if needed, and existing
files with the same names are overwritten. The export is a snapshot: edits to
the files do not change the tasks or notes.`,
		Example: "  taskship export ~/taskship-export --project myproject",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd)
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
				comments, err := s.ListComments(cmd.Context(), t.ID)
				if err != nil {
					return err
				}
				md := core.TaskMarkdown(t) + core.CommentsMarkdown(comments)
				if err := os.WriteFile(filepath.Join(dir, t.Ref+".md"), []byte(md), 0o644); err != nil {
					return err
				}
			}
			notes, err := exportNotes(cmd, s, project, filepath.Join(dir, "notes"))
			if err != nil {
				return err
			}
			what := fmt.Sprintf("%d %s", len(tasks), plural(len(tasks), "task", "tasks"))
			if notes > 0 {
				what += fmt.Sprintf(" and %d %s", notes, plural(notes, "note", "notes"))
			}
			success(cmd.OutOrStdout(), "Exported %s to %s", what, pathRef(dir))
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "only this project")
	return cmd
}

// exportNotes writes the notes of project, or of every project if it is
// empty, to dir/PROJECT.md, and returns how many there were. Like the
// tasks, archived projects are left out, and projects without notes get no
// file.
func exportNotes(cmd *cobra.Command, s *store.Store, project, dir string) (int, error) {
	ctx := cmd.Context()
	var projects []core.Project
	if project != "" {
		p, err := s.GetProject(ctx, project)
		if err != nil {
			return 0, err
		}
		projects = []core.Project{p}
	} else {
		var err error
		if projects, err = s.ListProjects(ctx, false); err != nil {
			return 0, err
		}
	}
	total := 0
	for _, p := range projects {
		if p.ArchivedAt != nil {
			continue
		}
		notes, err := s.ListNotes(ctx, p.ID)
		if err != nil {
			return 0, err
		}
		if len(notes) == 0 {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
		if err := os.WriteFile(filepath.Join(dir, p.Name+".md"), []byte(core.NotesMarkdown(p.Name, notes)), 0o644); err != nil {
			return 0, err
		}
		total += len(notes)
	}
	return total, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
