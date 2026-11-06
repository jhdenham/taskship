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
		{"2024-notes", "2024-notes"},
		{"360 Security", "360-security"},
		{"7-Eleven", "7-eleven"},
		{"1Password", "1-password"},
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
		// Longer words: the first three and the last letter of the consonant
		// skeleton, doubled letters collapsed, y a consonant, vowels refilled
		// left to right when short.
		{"taskship", "TSKP"},
		{"myproject", "MYPT"},
		{"frontend", "FRND"},
		{"backend", "BCKD"},
		{"password", "PSWD"},
		{"dashboard", "DSHD"},
		{"security", "SCRY"},
		{"billing", "BLNG"},
		{"habit", "HABT"},
		{"audio", "AUDI"},
		// Leading numbers are dropped like versions; a prefix starts with a letter.
		{"2024-notes", "NOTS"},
		{"360-security", "SCRY"},
		{"7-eleven", "ELVN"},
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

// With no letter to start a prefix, there is no suggestion.
func TestDefaultPrefixNone(t *testing.T) {
	for _, name := range []string{"2024", "360-3d"} {
		if got := DefaultPrefix(name, func(string) bool { return false }); got != "" {
			t.Errorf("DefaultPrefix(%q) = %q, want none", name, got)
		}
	}
}

func TestDefaultPrefixSkipsTakenPrefixes(t *testing.T) {
	taken := map[string]bool{"TSKP": true, "TSKP2": true}
	if got := DefaultPrefix("taskship", func(p string) bool { return taken[p] }); got != "TSKP3" {
		t.Errorf("DefaultPrefix with TSKP and TSKP2 taken = %q, want TSKP3", got)
	}
}

// Suggestions on the blocklist are shortened until they're not. The inputs
// are innocent words whose default prefix would be blocked.
func TestDefaultPrefixAvoidsBlocklist(t *testing.T) {
	none := func(string) bool { return false }
	for name, want := range map[string]string{
		"batch":           "BTC",
		"spice":           "SPI",
		"chinook":         "CHN",
		"app-service-sdk": "AS",
	} {
		if got := DefaultPrefix(name, none); got != want {
			t.Errorf("DefaultPrefix(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestBlocklistEntries(t *testing.T) {
	if len(blockedPrefixes) < 50 {
		t.Fatalf("blocklist has %d entries; did decoding fail?", len(blockedPrefixes))
	}
	for p := range blockedPrefixes {
		if !prefixRE.MatchString(p) || len(p) < 2 || len(p) > 4 {
			t.Errorf("blocklist entry %q (ROT13) is not a 2-4 letter prefix", rot13(p))
		}
	}
	if rot13("TASKSHIP") != "GNFXFUVC" || rot13(rot13("Taskship")) != "Taskship" {
		t.Error("rot13 is not ROT13")
	}
}

func TestFreePrefix(t *testing.T) {
	taken := map[string]bool{"PROJ": true, "PROJ2": true}
	isTaken := func(p string) bool { return taken[p] }
	if got := FreePrefix("PROJ", isTaken); got != "PROJ3" {
		t.Errorf("FreePrefix(PROJ) = %q, want PROJ3", got)
	}
	if got := FreePrefix("OTHER", isTaken); got != "OTHER" {
		t.Errorf("FreePrefix(OTHER) = %q, want OTHER", got)
	}
}
