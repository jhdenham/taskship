package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
)

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage projects",
	}
	cmd.AddCommand(newProjectAddCmd(), newProjectListCmd(), newProjectShowCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var p core.Project
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "add SLUG --prefix PREFIX",
		Short:   "Create a project",
		Example: "  taskship project add myproject --prefix PROJ --name \"My Project\"",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p.Slug = args[0]
			if p.TaskPrefix == "" {
				return usageError{fmt.Errorf("--prefix is required")}
			}
			if p.Name == "" {
				p.Name = p.Slug
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
			fmt.Fprintf(cmd.OutOrStdout(), "Created project %s (%s)\n", created.Slug, created.TaskPrefix)
			return nil
		},
	}
	cmd.Flags().StringVar(&p.TaskPrefix, "prefix", "", "task reference prefix, e.g. PROJ for PROJ-1 (required)")
	cmd.Flags().StringVar(&p.Name, "name", "", "display name (default: the slug)")
	cmd.Flags().StringVar(&p.Description, "description", "", "project description")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newProjectListCmd() *cobra.Command {
	var asJSON bool
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
			projects, err := s.ListProjects(cmd.Context())
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, projects)
			}
			if len(projects) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No projects. Create one with: taskship project add SLUG --prefix PREFIX")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SLUG\tPREFIX\tNAME")
			for _, p := range projects {
				fmt.Fprintf(w, "%s\t%s\t%s\n", p.Slug, p.TaskPrefix, p.Name)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newProjectShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show SLUG",
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
			fmt.Fprintf(w, "Slug:\t%s\n", p.Slug)
			fmt.Fprintf(w, "Name:\t%s\n", p.Name)
			fmt.Fprintf(w, "Prefix:\t%s\n", p.TaskPrefix)
			if p.Description != "" {
				fmt.Fprintf(w, "Description:\t%s\n", p.Description)
			}
			fmt.Fprintf(w, "Created:\t%s\n", p.CreatedAt.Local().Format(time.DateTime))
			fmt.Fprintf(w, "ID:\t%s\n", p.ID)
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
