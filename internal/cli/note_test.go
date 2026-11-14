package cli

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func TestNotes(t *testing.T) {
	seedCLIProjects(t)

	if out := mustRun(t, "note", "list", "-p", "myproject"); out != "No notes in myproject yet. Add one with \"taskship note add\".\n" {
		t.Errorf("empty list = %q", out)
	}
	if out := mustRun(t, "note", "add", "Try a compact board layout", "-p", "myproject"); out != "✓ Added note #1 to myproject\n" {
		t.Errorf("add = %q", out)
	}
	if r := runInput(t, "Handy query:\n\n```sql\nSELECT 1;\n```\n", "note", "add", "-", "-p", "myproject"); r.code != 0 || r.stdout != "✓ Added note #2 to myproject\n" {
		t.Errorf("add from stdin: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	if out := mustRun(t, "note", "add", "Elsewhere", "-p", "other"); out != "✓ Added note #1 to other\n" {
		t.Errorf("numbers are per project: %q", out)
	}

	// Without text, the editor opens, but only in a terminal.
	if r := run(t, "", "note", "add", "-p", "myproject"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "writing a note in the editor needs a terminal") {
		t.Errorf("editor without a terminal: exit %d, stderr %q", r.code, r.stderr)
	}
	tmp := fakeEditor(t, "\nFrom the editor.\n\n")
	if r := runInput(t, "", "note", "add", "-p", "myproject"); r.code != 0 || r.stdout != "✓ Added note #3 to myproject\n" {
		t.Errorf("add in editor: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	assertNoDrafts(t, tmp)
	fakeEditor(t, "  \n")
	if r := runInput(t, "", "note", "add", "-p", "myproject"); r.code != exitFailure || r.stdout != "The note is empty, so nothing changed.\n" {
		t.Errorf("empty editor note: exit %d, stdout %q", r.code, r.stdout)
	}
	for _, c := range [][]string{{"add", " ", "-p", "myproject"}, {"edit", "two", "x", "-p", "myproject"}} {
		if r := run(t, "", append([]string{"note"}, c...)...); r.code != exitInvalidInput {
			t.Errorf("%v: exit %d", c, r.code)
		}
	}

	// Newest first.
	heading := regexp.MustCompile(`(?m)^#(\d) · \d{4}-\d\d-\d\d \d\d:\d\d( \(edited\))?$`)
	list := mustRun(t, "note", "list", "-p", "myproject")
	if got := heading.ReplaceAllString(list, "#$1$2"); got != "#3\nFrom the editor.\n\n#2\nHandy query:\n\n```sql\nSELECT 1;\n```\n\n#1\nTry a compact board layout\n" {
		t.Errorf("list =\n%s", list)
	}

	if out := mustRun(t, "note", "edit", "#1", "Try a compact board", "-p", "myproject"); out != "✓ Updated note #1 in myproject\n" {
		t.Errorf("edit = %q", out)
	}
	if out := mustRun(t, "note", "edit", "1", "Try a compact board", "-p", "myproject"); out != "No changes to note #1 in myproject\n" {
		t.Errorf("unchanged edit = %q", out)
	}
	fakeEditor(t, "Edited in the editor.")
	if r := runInput(t, "", "note", "edit", "3", "-p", "myproject"); r.code != 0 {
		t.Errorf("edit in editor: exit %d, stderr %q", r.code, r.stderr)
	}
	var notes []core.Note
	if err := json.Unmarshal([]byte(mustRun(t, "note", "list", "-p", "myproject", "--json")), &notes); err != nil || len(notes) != 3 ||
		notes[0].Body != "Edited in the editor." || notes[2].Body != "Try a compact board" || !notes[2].Edited() || notes[1].Edited() {
		t.Errorf("list --json = %+v, %v", notes, err)
	}
	for _, c := range [][]string{{"edit", "9", "x", "-p", "myproject"}, {"delete", "9", "-y", "-p", "myproject"}, {"list", "-p", "nope"}} {
		if r := run(t, "", append([]string{"note"}, c...)...); r.code != exitNotFound {
			t.Errorf("%v: exit %d, stderr %q", c, r.code, r.stderr)
		}
	}

	// delete asks in a terminal and needs --yes without one.
	if r := run(t, "", "note", "delete", "1", "-p", "myproject"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "--yes") {
		t.Errorf("delete without a terminal: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := runInput(t, "n\n", "note", "delete", "1", "-p", "myproject"); r.code != exitFailure || !strings.Contains(r.stdout, "Nothing deleted.") {
		t.Errorf("declined delete: exit %d, stdout %q", r.code, r.stdout)
	}
	if r := runInput(t, "y\n", "note", "delete", "1", "-p", "myproject"); r.code != 0 || !strings.HasSuffix(r.stdout, "✓ Deleted note #1 from myproject\n") {
		t.Errorf("delete: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	mustRun(t, "note", "delete", "#3", "--yes", "-p", "myproject")
	if out := mustRun(t, "note", "add", "Fourth", "-p", "myproject"); out != "✓ Added note #4 to myproject\n" {
		t.Errorf("numbers aren't reused: %q", out)
	}

	// An archived project takes no notes, and says so before the editor opens.
	mustRun(t, "project", "archive", "other")
	if r := runInput(t, "", "note", "add", "-p", "other"); r.code != exitInvalidInput || !strings.Contains(r.stderr, "is archived") || r.stdout != "" {
		t.Errorf("archived project: exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}

	// Without --project, the current folder's project; none here.
	t.Chdir(t.TempDir())
	if r := run(t, "", "note", "list"); r.code != exitInvalidInput || !strings.HasSuffix(r.stderr, "to set it up, or pass --project NAME.\n") {
		t.Errorf("no project here: exit %d, stderr %q", r.code, r.stderr)
	}
	repo, _ := gitRepo(t)
	mustRun(t, "workspace", "add", "myproject", "--path", repo)
	t.Chdir(repo)
	if out := mustRun(t, "note", "add", "From the repo"); out != "✓ Added note #5 to myproject\n" {
		t.Errorf("add in the project's folder = %q", out)
	}
}
