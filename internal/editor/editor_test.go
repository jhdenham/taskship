package editor

import "testing"

func TestName(t *testing.T) {
	for _, tt := range []struct{ visual, editor, want string }{
		{"", "", "vi"},
		{"", "nano", "nano"},
		{"", "/opt/homebrew/bin/nvim", "nvim"},
		{"code --wait", "nano", "code"},
	} {
		t.Setenv("VISUAL", tt.visual)
		t.Setenv("EDITOR", tt.editor)
		if got := Name(); got != tt.want {
			t.Errorf("VISUAL=%q EDITOR=%q: Name() = %q, want %q", tt.visual, tt.editor, got, tt.want)
		}
	}
}
