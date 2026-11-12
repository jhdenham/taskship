package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTaskMarkdownRoundTrip(t *testing.T) {
	task := Task{
		Ref: "PROJ-1", Project: "myproject", Status: StatusTodo, Priority: PriorityHigh, Labels: []string{"cli", "infra"},
		Title:       "Add weekly habit tracker",
		Description: "Track habits per week.\n\n## Acceptance criteria\n\n- [ ] Weekly view\n- [x] Persists",
	}
	md := TaskMarkdown(task)
	want := "---\nref: PROJ-1\nproject: myproject\nstatus: todo\npriority: high\nlabels: [cli, infra]\n---\n\n# Add weekly habit tracker\n\n" + task.Description + "\n"
	if md != want {
		t.Fatalf("TaskMarkdown =\n%s\nwant\n%s", md, want)
	}
	doc, err := ParseTaskMarkdown(md)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc, TaskDoc{Ref: "PROJ-1", Project: "myproject", Status: StatusTodo, Priority: PriorityHigh,
		Labels: []string{"cli", "infra"}, Title: task.Title, Description: task.Description}) {
		t.Errorf("round trip = %+v", doc)
	}

	task.Description, task.Labels = "", []string{}
	if !strings.Contains(TaskMarkdown(task), "\nlabels: []\n") {
		t.Errorf("no labels:\n%s", TaskMarkdown(task))
	}
	if doc, err := ParseTaskMarkdown(TaskMarkdown(task)); err != nil || doc.Description != "" || doc.Title != task.Title ||
		doc.Labels == nil || len(doc.Labels) != 0 {
		t.Errorf("empty description round trip = %+v, %v", doc, err)
	}
}

func TestParseTaskMarkdown(t *testing.T) {
	// Front matter is optional, CRLF is accepted, and surrounding blank lines are dropped.
	doc, err := ParseTaskMarkdown("\r\n# Title only  \r\n\r\nBody\r\n  indented\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc, TaskDoc{Title: "Title only", Description: "Body\n  indented"}) {
		t.Errorf("doc = %+v", doc)
	}

	// Front matter keys are optional too.
	doc, err = ParseTaskMarkdown("---\nstatus: done\n---\n\n# T\n")
	if err != nil || doc.Status != StatusDone || doc.Priority != "" || doc.Ref != "" || doc.Labels != nil {
		t.Errorf("partial front matter = %+v, %v", doc, err)
	}

	doc, err = ParseTaskMarkdown("---\nlabels: [ Infra,bug , infra ]\n---\n# T\n")
	if err != nil || !reflect.DeepEqual(doc.Labels, []string{"bug", "infra"}) {
		t.Errorf("labels = %q, %v", doc.Labels, err)
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
		"---\nlabels: infra\n---\n# T\n",
		"---\nlabels: [infra\n---\n# T\n",
		"---\nlabels: [two words]\n---\n# T\n",
		"---\nlabels: [a, , b]\n---\n# T\n",
	} {
		if _, err := ParseTaskMarkdown(src); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseTaskMarkdown(%q) err = %v, want ErrInvalid", src, err)
		}
	}
}

func TestCommentsMarkdown(t *testing.T) {
	if got := CommentsMarkdown(nil); got != "" {
		t.Errorf("no comments = %q", got)
	}
	at := time.Date(2026, 11, 12, 14, 3, 0, 0, time.Local)
	comments := []Comment{
		{Number: 1, Body: "Tried bumping the timeout.", CreatedAt: at, UpdatedAt: at},
		{Number: 3, Body: "Still flaky.\n\n# Not a title", CreatedAt: at, UpdatedAt: at.Add(time.Hour)},
	}
	want := "\n" + CommentsMarker + "\n## Comments\n\n### #1 · 2026-11-12 14:03\n\nTried bumping the timeout.\n" +
		"\n### #3 · 2026-11-12 14:03 (edited)\n\nStill flaky.\n\n# Not a title\n"
	md := CommentsMarkdown(comments)
	if md != want {
		t.Errorf("CommentsMarkdown =\n%s\nwant\n%s", md, want)
	}

	// Parsing a document with comments drops them from the description.
	task := Task{Ref: "PROJ-1", Project: "myproject", Status: StatusTodo, Priority: PriorityNone, Title: "T", Description: "Body."}
	doc, err := ParseTaskMarkdown(TaskMarkdown(task) + md)
	if err != nil || doc.Description != "Body." || doc.Title != "T" {
		t.Errorf("parse with comments = %+v, %v", doc, err)
	}
}
