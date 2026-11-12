package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderMarkdown(t *testing.T) {
	for _, tt := range []struct {
		name, src string
		width     int
		want      string
	}{
		{"hard-wrapped paragraph rewraps",
			"Add a new file called \"hello-world.md\" to the /docs directory. The file must read\n\"Hello World!\". On a separate line in the file, we need to also add \"Hello World!\"\nbackwards.",
			60,
			"Add a new file called \"hello-world.md\" to the /docs\n" +
				"directory. The file must read \"Hello World!\". On a separate\n" +
				"line in the file, we need to also add \"Hello World!\"\n" +
				"backwards."},
		{"paragraphs stay apart", "One\ntwo.\n\nThree.", 40, "One two.\n\nThree."},
		{"hard breaks", "Line one  \nline two\\\nline three", 40, "Line one\nline two\nline three"},
		{"list items and continuations",
			"- first item that\n  continues here\n- second\n1. numbered\n   item", 40,
			"• first item that continues here\n• second\n1. numbered item"},
		{"checklist under a heading",
			"## Acceptance criteria\n\n- [x] Weekly view shows each habit\n- [ ] Data persists across\n  restarts", 40,
			"Acceptance criteria   1 of 2 done\n[x] Weekly view shows each habit\n[ ] Data persists across restarts"},
		{"wrapped list item hangs", "- a b c d e f g h i j", 12, "• a b c d e\n  f g h i j"},
		{"quote", "> quoted\n> text", 40, "│ quoted text"},
		{"code keeps its lines", "Run:\n\n```\ngo test\n  ./...\n```", 40, "Run:\n\n  go test\n    ./..."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ansi.Strip(strings.Join(renderMarkdown(tt.src, tt.width, newTheme(true)), "\n"))
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
