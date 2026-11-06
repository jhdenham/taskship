package core

import (
	"strings"
	"testing"
)

func TestNameFromFolder(t *testing.T) {
	tests := []struct{ folder, want string }{
		{"myproject", "myproject"},
		{"my-project", "my-project"},
		{"MyCoolApp", "my-cool-app"},
		{"InvoicePDF", "invoice-pdf"},
		{"HTTPServer", "http-server"},
		{"my_project", "my-project"},
		{"My Project", "my-project"},
		{"api.v2", "api-v2"},
		{"--weird--name--", "weird-name"},
		{"2024-notes", "notes"},
		{".bare", "bare"},
		{"日本", ""},
		{strings.Repeat("abcd-", 10), "abcd-abcd-abcd-abcd-abcd-abcd-abcd-abcd"},
	}
	for _, tt := range tests {
		got := NameFromFolder(tt.folder)
		if got != tt.want {
			t.Errorf("NameFromFolder(%q) = %q, want %q", tt.folder, got, tt.want)
		}
		if got != "" {
			if err := ValidateProject(Project{Name: got, TaskPrefix: "P"}); err != nil {
				t.Errorf("NameFromFolder(%q) = %q is not a valid name: %v", tt.folder, got, err)
			}
		}
	}
}

func TestDefaultPrefix(t *testing.T) {
	none := func(string) bool { return false }
	tests := []struct{ name, want string }{
		// Several words: initials, capped at 4.
		{"task-ship", "TS"},
		{"my-cool-app", "MCA"},
		{"my-very-cool-web-app", "MVCW"},
		// Version-like words are dropped first.
		{"api-v2", "API"},
		{"web-2024", "WEB"},
		{"tool-v2-cli", "TC"},
		// One word of 4 letters or fewer: the whole word.
		{"web", "WEB"},
		{"web3", "WEB3"},
		{"x", "X"},
		// Longer words: consonant skeleton, doubled letters collapsed, y a
		// consonant, vowels refilled left to right when short.
		{"taskship", "TSKS"},
		{"myproject", "MYPR"},
		{"frontend", "FRNT"},
		{"dashboard", "DSHB"},
		{"billing", "BLNG"},
		{"habit", "HABT"},
		{"audio", "AUDI"},
		// A dropped word can't leave a prefix starting with a digit.
		{"v2-3d", "V3"},
	}
	for _, tt := range tests {
		got := DefaultPrefix(tt.name, none)
		if got != tt.want {
			t.Errorf("DefaultPrefix(%q) = %q, want %q", tt.name, got, tt.want)
		}
		if err := ValidateProject(Project{Name: tt.name, TaskPrefix: got}); err != nil {
			t.Errorf("DefaultPrefix(%q) = %q is not a valid prefix: %v", tt.name, got, err)
		}
	}
}

func TestDefaultPrefixSkipsTakenPrefixes(t *testing.T) {
	taken := map[string]bool{"TSKS": true, "TSKS2": true}
	if got := DefaultPrefix("taskship", func(p string) bool { return taken[p] }); got != "TSKS3" {
		t.Errorf("DefaultPrefix with TSKS and TSKS2 taken = %q, want TSKS3", got)
	}
}
