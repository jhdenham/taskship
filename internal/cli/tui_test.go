package cli

import (
	"testing"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/workspace"
)

// The TUI offers to set up a folder only when it isn't set up and init can
// run there.
func TestTUIOptions(t *testing.T) {
	ws := []core.Workspace{
		{Project: "myproject", Path: "/code/myproject", GitCommonDir: "/code/myproject/.git"},
		{Project: "mono-web", Path: "/code/mono", GitCommonDir: "/code/mono/.git"},
		{Project: "mono-api", Path: "/code/mono/api", GitCommonDir: "/code/mono/.git"},
	}
	for _, tt := range []struct {
		name, path, commonDir string
		project               string
		canSetUp              bool
	}{
		{"its project", "/code/myproject", "/code/myproject/.git", "myproject", true},
		{"not set up", "/code/website", "/code/website/.git", "", true},
		// A worktree of a repository registered for several projects is
		// already set up, for more than one project.
		{"several projects", "/worktrees/mono/feature", "/code/mono/.git", "", false},
		{"filesystem root", "/", "", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := tuiOptions(workspace.Location{Path: tt.path, GitCommonDir: tt.commonDir}, ws)
			if err != nil {
				t.Fatal(err)
			}
			if opts.Project != tt.project || opts.CanSetUp != tt.canSetUp {
				t.Errorf("project %q, can set up %v; want %q, %v", opts.Project, opts.CanSetUp, tt.project, tt.canSetUp)
			}
		})
	}
}
