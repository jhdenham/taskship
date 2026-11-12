package display

import (
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, tt := range []struct{ in, want string }{
		{home, "~"},
		{filepath.Join(home, "code", "myproject"), "~/code/myproject"},
		{home + "-2/x", home + "-2/x"},
		{"/tmp/x", "/tmp/x"},
	} {
		if got := Path(tt.in); got != tt.want {
			t.Errorf("Path(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRule(t *testing.T) {
	if got := Rule("Comments (2)", 24); got != "── Comments (2) ────────" {
		t.Errorf("Rule = %q", got)
	}
	if got := Rule("A long title here", 10); got != "── A long title here ───" {
		t.Errorf("narrow Rule = %q", got)
	}
}
