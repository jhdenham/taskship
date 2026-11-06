package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
	"github.com/jhdenham/taskship/internal/workspace"
)

func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Manage local checkouts of projects",
		Long: `Manage the local checkouts (workspaces) of projects.

Workspaces are stored only on this machine, so the same project can live at
different paths on different machines. Register a project's main checkout once:
every Git worktree of the same repository is recognized automatically from Git
metadata, so worktrees never need to be added or cleaned up.

Inside a registered checkout, one of its worktrees, or a folder below either,
"taskship add" uses that project without --project.`,
	}
	cmd.AddCommand(newWorkspaceAddCmd(), newWorkspaceListCmd(), newWorkspaceRemoveCmd())
	return cmd
}

func newWorkspaceAddCmd() *cobra.Command {
	var path string
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "add PROJECT [--path PATH]",
		Short:   "Register a local checkout or folder for a project",
		Example: "  taskship workspace add myproject --path ~/code/myproject",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			loc, err := workspace.Inspect(ctx, path)
			if err != nil {
				return err
			}
			s, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			w, err := s.AddWorkspace(ctx, args[0], core.Workspace{Path: loc.Path, GitCommonDir: loc.GitCommonDir})
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, w)
			}
			out := cmd.OutOrStdout()
			success(out, "Added workspace %s for %s", displayPath(w.Path), w.Project)
			if w.GitCommonDir != "" {
				fmt.Fprintln(out, "Worktrees of this repository are recognized automatically.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", ".", "directory to register")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newWorkspaceListCmd() *cobra.Command {
	var project string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list [--project NAME]",
		Short: "List registered workspaces",
		Long:  "List registered workspaces. Paths that no longer exist are marked (missing); remove them with \"taskship workspace remove PATH\".",
		Args:  exactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			ws, err := s.ListWorkspaces(cmd.Context(), project)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, ws)
			}
			if len(ws) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No workspaces. Register one with: taskship workspace add PROJECT --path PATH")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PROJECT\tPATH")
			for _, ws := range ws {
				mark := ""
				if _, err := os.Stat(ws.Path); err != nil {
					mark = " (missing)"
				}
				fmt.Fprintf(w, "%s\t%s%s\n", ws.Project, displayPath(ws.Path), mark)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "only workspaces of this project")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newWorkspaceRemoveCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "remove PATH",
		Short: "Unregister a workspace (files are not touched)",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// The path may no longer exist, so resolve symlinks only if possible.
			path, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if real, err := filepath.EvalSymlinks(path); err == nil {
				path = real
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			w, err := s.RemoveWorkspace(cmd.Context(), path)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, w)
			}
			success(cmd.OutOrStdout(), "Removed workspace %s from %s", displayPath(w.Path), w.Project)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

// projectForCurrentDir returns the name of the project whose registered
// workspace contains the current directory, directly or through a Git
// worktree of the same repository. ok is false if no project matches.
func projectForCurrentDir(ctx context.Context, s *store.Store) (name string, ok bool, err error) {
	loc, err := workspace.Inspect(ctx, ".")
	if err != nil {
		return "", false, err
	}
	ws, err := s.ListWorkspaces(ctx, "")
	if err != nil {
		return "", false, err
	}
	w, ok, err := workspace.Resolve(loc, ws)
	if errors.Is(err, core.ErrAmbiguous) {
		return "", false, fmt.Errorf("%w; pass --project", err)
	}
	return w.Project, ok, err
}
