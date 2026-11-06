package core

import (
	"errors"
	"testing"
)

func TestTaskMarkdownRoundTrip(t *testing.T) {
	task := Task{
		Ref: "PROJ-1", Project: "myproject", Status: StatusTodo, Priority: PriorityHigh,
		Title:       "Add weekly habit tracker",
		Description: "Track habits per week.\n\n## Acceptance criteria\n\n- [ ] Weekly view\n- [x] Persists",
	}
	md := TaskMarkdown(task)
	want := "---\nref: PROJ-1\nproject: myproject\nstatus: todo\npriority: high\n---\n# Add weekly habit tracker\n\n" + task.Description + "\n"
	if md != want {
		t.Fatalf("TaskMarkdown =\n%s\nwant\n%s", md, want)
	}
	doc, err := ParseTaskMarkdown(md)
	if err != nil {
		t.Fatal(err)
	}
	if doc != (TaskDoc{Ref: "PROJ-1", Project: "myproject", Status: StatusTodo, Priority: PriorityHigh, Title: task.Title, Description: task.Description}) {
		t.Errorf("round trip = %+v", doc)
	}

	task.Description = ""
	if doc, err := ParseTaskMarkdown(TaskMarkdown(task)); err != nil || doc.Description != "" || doc.Title != task.Title {
		t.Errorf("empty description round trip = %+v, %v", doc, err)
	}
}

func TestParseTaskMarkdown(t *testing.T) {
	// Front matter is optional, CRLF is accepted, and surrounding blank lines are dropped.
	doc, err := ParseTaskMarkdown("\r\n# Title only  \r\n\r\nBody\r\n  indented\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if doc != (TaskDoc{Title: "Title only", Description: "Body\n  indented"}) {
		t.Errorf("doc = %+v", doc)
	}

	// Front matter keys are optional too.
	doc, err = ParseTaskMarkdown("---\nstatus: done\n---\n\n# T\n")
	if err != nil || doc.Status != StatusDone || doc.Priority != "" || doc.Ref != "" {
		t.Errorf("partial front matter = %+v, %v", doc, err)
	}

	for _, src := range []string{
		"",
		"no heading",
		"## Not a title",
		"---\nstatus: todo\n# T\n",
		"---\nstatus: open\n---\n# T\n",
		"---\npriority: p1\n---\n# T\n",
		"---\ntitle: T\n---\n# T\n",
		"---\njust text\n---\n# T\n",
	} {
		if _, err := ParseTaskMarkdown(src); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseTaskMarkdown(%q) err = %v, want ErrInvalid", src, err)
		}
	}
}
