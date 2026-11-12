package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func TestComments(t *testing.T) {
	seedCLIProjects(t)
	mustRun(t, "add", "Fix flaky test", "-p", "myproject", "--description", "It fails on CI.")

	if out := mustRun(t, "comment", "list", "PROJ-1"); out != "No comments on PROJ-1 yet. Add one with \"taskship comment add PROJ-1\".\n" {
		t.Errorf("empty list = %q", out)
	}
	if out := mustRun(t, "comment", "add", "proj-1", "Tried bumping the timeout."); out != "✓ Added comment #1 to PROJ-1\n" {
		t.Errorf("add = %q", out)
	}
	if r := runInput(t, "Still flaky\non CI.\n", "comment", "add", "PROJ-1", "-"); r.code != 0 || r.stdout != "✓ Added comment #2 to PROJ-1\n" {
		t.Errorf("add from stdin: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}

	// Without text, the editor opens, but only in a terminal.
	if r := run(t, "", "comment", "add", "PROJ-1"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "needs a terminal") {
		t.Errorf("editor without a terminal: exit %d, stderr %q", r.code, r.stderr)
	}
	tmp := fakeEditor(t, "\nFrom the editor.\n\n")
	if r := runInput(t, "", "comment", "add", "PROJ-1"); r.code != 0 || r.stdout != "✓ Added comment #3 to PROJ-1\n" {
		t.Errorf("add in editor: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	assertNoDrafts(t, tmp)
	fakeEditor(t, "  \n")
	if r := runInput(t, "", "comment", "add", "PROJ-1"); r.code != exitFailure || !strings.Contains(r.stdout, "nothing changed") {
		t.Errorf("empty editor comment: exit %d, stdout %q", r.code, r.stdout)
	}
	if r := run(t, "", "comment", "add", "PROJ-1", " "); r.code != exitInvalidInput {
		t.Errorf("empty comment: exit %d", r.code)
	}

	heading := regexp.MustCompile(`(?m)^#(\d) · \d{4}-\d\d-\d\d \d\d:\d\d( \(edited\))?$`)
	list := mustRun(t, "comment", "list", "PROJ-1")
	if got := heading.ReplaceAllString(list, "#$1$2"); got != "#1\nTried bumping the timeout.\n\n#2\nStill flaky\non CI.\n\n#3\nFrom the editor.\n" {
		t.Errorf("list =\n%s", list)
	}

	// Edit with text, and in the editor starting from the current text.
	if out := mustRun(t, "comment", "edit", "PROJ-1", "#2", "Still flaky on CI."); out != "✓ Updated comment #2 on PROJ-1\n" {
		t.Errorf("edit = %q", out)
	}
	if out := mustRun(t, "comment", "edit", "PROJ-1", "2", "Still flaky on CI."); out != "No changes to comment #2 on PROJ-1\n" {
		t.Errorf("unchanged edit = %q", out)
	}
	fakeEditor(t, "Edited in the editor.")
	if r := runInput(t, "", "comment", "edit", "PROJ-1", "3"); r.code != 0 {
		t.Errorf("edit in editor: exit %d, stderr %q", r.code, r.stderr)
	}
	var comments []core.Comment
	if err := json.Unmarshal([]byte(mustRun(t, "comment", "list", "PROJ-1", "--json")), &comments); err != nil || len(comments) != 3 ||
		comments[1].Body != "Still flaky on CI." || !comments[1].Edited() || comments[2].Body != "Edited in the editor." || comments[0].Edited() {
		t.Errorf("list --json = %+v, %v", comments, err)
	}
	for _, c := range [][]string{{"edit", "PROJ-1", "9", "x"}, {"edit", "PROJ-9", "1", "x"}, {"delete", "PROJ-1", "9", "-y"}} {
		if r := run(t, "", append([]string{"comment"}, c...)...); r.code != exitNotFound {
			t.Errorf("%v: exit %d, stderr %q", c, r.code, r.stderr)
		}
	}
	if r := run(t, "", "comment", "edit", "PROJ-1", "two", "x"); r.code != exitInvalidInput {
		t.Errorf("bad number: exit %d", r.code)
	}

	// show includes comments; show --md puts them after the marker.
	if out := mustRun(t, "show", "PROJ-1"); !strings.Contains(out, "It fails on CI.\n\nComments (3)\n\n#1 · ") {
		t.Errorf("show =\n%s", out)
	}
	md := mustRun(t, "show", "PROJ-1", "--md")
	if !strings.Contains(md, "\nIt fails on CI.\n\n"+core.CommentsMarker+"\n## Comments\n\n### #1 · ") ||
		!strings.Contains(md, " (edited)\n\nStill flaky on CI.\n") {
		t.Errorf("show --md =\n%s", md)
	}
	var shown struct {
		Ref      string         `json:"ref"`
		Comments []core.Comment `json:"comments"`
	}
	if err := json.Unmarshal([]byte(mustRun(t, "show", "PROJ-1", "--json")), &shown); err != nil || shown.Ref != "PROJ-1" || len(shown.Comments) != 3 {
		t.Errorf("show --json = %+v, %v", shown, err)
	}

	// Applying the shown Markdown back changes nothing: comments aren't description.
	if r := runInput(t, md, "task", "edit", "PROJ-1", "--from", "-"); r.code != 0 || r.stdout != "No changes to PROJ-1\n" {
		t.Errorf("edit --from with comments: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}

	// export writes them too.
	dir := t.TempDir()
	mustRun(t, "export", dir, "-p", "myproject")
	if data, err := os.ReadFile(filepath.Join(dir, "PROJ-1.md")); err != nil || string(data) != md {
		t.Errorf("export =\n%s\n%v", data, err)
	}

	// delete asks in a terminal and needs --yes without one.
	if r := run(t, "", "comment", "delete", "PROJ-1", "1"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "--yes") {
		t.Errorf("delete without a terminal: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := runInput(t, "n\n", "comment", "delete", "PROJ-1", "1"); r.code != exitFailure || !strings.Contains(r.stdout, "Nothing deleted.") {
		t.Errorf("declined delete: exit %d, stdout %q", r.code, r.stdout)
	}
	if r := runInput(t, "y\n", "comment", "delete", "PROJ-1", "1"); r.code != 0 || !strings.HasSuffix(r.stdout, "✓ Deleted comment #1 from PROJ-1\n") {
		t.Errorf("delete: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	mustRun(t, "comment", "delete", "PROJ-1", "#3", "--yes")
	if out := mustRun(t, "comment", "add", "PROJ-1", "Fourth"); out != "✓ Added comment #4 to PROJ-1\n" {
		t.Errorf("numbers aren't reused: %q", out)
	}
}
