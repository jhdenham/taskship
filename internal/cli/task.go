package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/store"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
	}
	cmd.AddCommand(newTaskAddCmd("add"), newTaskListCmd("list"), newTaskShowCmd("show"))
	return cmd
}

// The constructors take the command name so the root shortcuts
// (taskship add, tasks, show) are the same commands as task add, list, show.

func newTaskAddCmd(name string) *cobra.Command {
	var project, description, status, priority string
	var asJSON bool
	cmd := &cobra.Command{
		Use:     name + " TITLE --project SLUG",
		Short:   "Create a task",
		Example: "  taskship " + name + " \"Fix export\" --project myproject",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" {
				return usageError{fmt.Errorf("--project is required")}
			}
			st, err := core.ParseStatus(status)
			if err != nil {
				return err
			}
			pr, err := core.ParsePriority(priority)
			if err != nil {
				return err
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			t, err := s.CreateTask(cmd.Context(), project, core.Task{
				Title: args[0], Description: description, Status: st, Priority: pr,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, t)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s: %s\n", t.Ref, t.Title)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project slug (required)")
	cmd.Flags().StringVar(&description, "description", "", "task description")
	cmd.Flags().StringVar(&status, "status", string(core.StatusTodo), "initial status")
	cmd.Flags().StringVar(&priority, "priority", string(core.PriorityNone), "priority: none, low, medium, high, urgent")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newTaskListCmd(name string) *cobra.Command {
	var project string
	var statuses []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:     name,
		Short:   "List tasks",
		Example: "  taskship " + name + " --project myproject --status todo,in_progress",
		Args:    exactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := store.TaskFilter{ProjectSlug: project}
			for _, s := range statuses {
				st, err := core.ParseStatus(s)
				if err != nil {
					return err
				}
				filter.Statuses = append(filter.Statuses, st)
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			tasks, err := s.ListTasks(cmd.Context(), filter)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, tasks)
			}
			if len(tasks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tasks.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "REF\tSTATUS\tPRIORITY\tTITLE")
			for _, t := range tasks {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Ref, t.Status, t.Priority, t.Title)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "only tasks in this project")
	cmd.Flags().StringSliceVar(&statuses, "status", nil, "only tasks with these statuses (comma-separated or repeated)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func newTaskShowCmd(name string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   name + " TASK_REF",
		Short: "Show task details",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			t, err := s.GetTask(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd, t)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s  %s\n\n", t.Ref, t.Title)
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "Project:\t%s\n", t.Project)
			fmt.Fprintf(w, "Status:\t%s\n", t.Status)
			fmt.Fprintf(w, "Priority:\t%s\n", t.Priority)
			fmt.Fprintf(w, "Created:\t%s\n", t.CreatedAt.Local().Format(time.DateTime))
			fmt.Fprintf(w, "Updated:\t%s\n", t.UpdatedAt.Local().Format(time.DateTime))
			fmt.Fprintf(w, "ID:\t%s\n", t.ID)
			if err := w.Flush(); err != nil {
				return err
			}
			if t.Description != "" {
				fmt.Fprintf(out, "\n%s\n", t.Description)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}
