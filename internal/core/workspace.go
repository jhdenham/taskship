package core

import "time"

// Workspace is a machine-local checkout registered for a project. Path is an
// absolute, symlink-free directory. GitCommonDir identifies its Git
// repository (empty for a non-Git directory), so linked worktrees of the same
// repository are recognized without being registered.
type Workspace struct {
	ID           string    `json:"id"`
	ProjectID    string    `json:"project_id"`
	Project      string    `json:"project"` // project slug, for display
	Path         string    `json:"path"`
	GitCommonDir string    `json:"git_common_dir,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
