package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}
	cmd.AddCommand(
		newProjectAddCmd(),
		newProjectListCmd(),
		newProjectShowCmd(),
		newProjectEditCmd(),
		newProjectArchiveCmd(true),
		newProjectArchiveCmd(false),
	)
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var p core.Project
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "add NAME --prefix PREFIX",
		Short:   "Create a project",
		Example: "  taskship project add myproject --prefix PROJ",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p.Name = args[0]
			if p.TaskPrefix == "" {
				return usageError{fmt.Errorf("--prefix is required")}
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			created, err := s.CreateProject(cmd.Context(), p)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, created)
			}
			success(cmd.OutOrStdout(), "Created project %s (%s)", value.Render(created.Name), value.Render(created.TaskPrefix))
			return nil
		},
	}
	cmd.Flags().StringVar(&p.TaskPrefix, "prefix", "", "task reference prefix, e.g. PROJ for PROJ-1 (required)")
	cmd.Flags().StringVar(&p.Description, "description", "", "project description")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newProjectListCmd() *cobra.Command {
	var asJSON, archived bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			projects, err := s.ListProjects(cmd.Context(), archived)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, projects)
			}
			if len(projects) == 0 {
				msg := noProjectsYet
				if !archived {
					if all, err := s.ListProjects(cmd.Context(), true); err != nil {
						return err
					} else if len(all) > 0 {
						msg = "All projects are archived. Use --archived to list them."
					}
				}
				lipgloss.Fprintln(cmd.OutOrStdout(), msg)
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tPREFIX")
			for _, p := range projects {
				fmt.Fprintf(w, "%s\t%s%s\n", p.Name, p.TaskPrefix, archivedMark(p.ArchivedAt))
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&archived, "archived", false, "include archived projects")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newProjectShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show NAME",
		Short: "Show project details",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			p, err := s.GetProject(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, p)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "Name:\t%s\n", p.Name)
			fmt.Fprintf(w, "Prefix:\t%s\n", p.TaskPrefix)
			if p.Description != "" {
				fmt.Fprintf(w, "Description:\t%s\n", p.Description)
			}
			fmt.Fprintf(w, "Created:\t%s\n", p.CreatedAt.Local().Format(time.DateTime))
			if p.ArchivedAt != nil {
				fmt.Fprintf(w, "Archived:\t%s\n", p.ArchivedAt.Local().Format(time.DateTime))
			}
			fmt.Fprintf(w, "ID:\t%s\n", p.ID)
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newProjectEditCmd() *cobra.Command {
	var name, description string
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "edit NAME [--name NEW_NAME] [--description TEXT]",
		Short:   "Rename a project or change its description",
		Long:    "Rename a project or change its description. Tasks keep their references.\n\nThe task prefix cannot be changed yet, because existing task references would break.",
		Example: "  taskship project edit myproject --name my-project",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var patch store.ProjectPatch
			flags := cmd.Flags()
			if flags.Changed("name") {
				patch.Name = &name
			}
			if flags.Changed("description") {
				patch.Description = &description
			}
			if patch == (store.ProjectPatch{}) {
				return usageError{fmt.Errorf("nothing to change; pass --name or --description")}
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			p, err := s.UpdateProject(cmd.Context(), args[0], patch)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, p)
			}
			success(cmd.OutOrStdout(), "Updated project %s", value.Render(p.Name))
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "new name")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

// newProjectArchiveCmd returns "project archive" or, if archive is false, "project restore".
func newProjectArchiveCmd(archive bool) *cobra.Command {
	verb, short, done := "archive", "Archive a project and hide it and its tasks from default lists", "Archived"
	if !archive {
		verb, short, done = "restore", "Restore an archived project", "Restored"
	}
	var asJSON bool
	cmd := &cobra.Command{
		Use:   verb + " NAME",
		Short: short,
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			p, err := s.SetProjectArchived(cmd.Context(), args[0], archive)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, p)
			}
			success(cmd.OutOrStdout(), "%s project %s", done, value.Render(p.Name))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func archivedMark(at *time.Time) string {
	if at != nil {
		return " (archived)"
	}
	return ""
}
