package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jhdenham/taskship/internal/display"
)

func testPrompter(input string) (*prompter, *bytes.Buffer) {
	var out bytes.Buffer
	return &prompter{in: bufio.NewReader(strings.NewReader(input)), out: &out}, &out
}

func upper(s string) (string, error) { return strings.ToUpper(s), nil }

func TestAsk(t *testing.T) {
	tests := []struct {
		name, input, def, want, transcript string
		check                              func(string) (string, error)
	}{
		{"default", "\n", "myproject", "myproject", "? Project name (myproject): ", nil},
		{"typed", "other\n", "myproject", "other", "? Project name (myproject): ", nil},
		{"trimmed", "  other  \n", "", "other", "? Project name: ", nil},
		{"normalized", "proj\n", "MYPT", "PROJ", "? Project name (MYPT): ", upper},
		{"no default asks again", "\n\nx\n", "", "x", "? Project name: ? Project name: ? Project name: ", nil},
		{"last line without newline", "x", "", "x", "? Project name: ", nil},
		{
			"retry with new default", "My Project\n\n", "myproject", "my-project",
			"? Project name (myproject): " +
				"  ✗ Names use lowercase letters, digits, and hyphens.\n" +
				"? Project name (my-project): ",
			func(s string) (string, error) {
				if s == "My Project" {
					return "", retry{"Names use lowercase letters, digits, and hyphens.", "my-project"}
				}
				return s, nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, out := testPrompter(tt.input)
			got, err := p.ask("Project name", tt.def, tt.check)
			if err != nil || got != tt.want {
				t.Errorf("ask = %q, %v; want %q", got, err, tt.want)
			}
			if out.String() != tt.transcript {
				t.Errorf("transcript:\n%q\nwant:\n%q", out.String(), tt.transcript)
			}
		})
	}
}

func TestAskStopsOnOtherErrors(t *testing.T) {
	boom := errors.New("boom")
	p, _ := testPrompter("x\n")
	if _, err := p.ask("Q", "", func(string) (string, error) { return "", boom }); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}

func TestAskInputEnded(t *testing.T) {
	p, _ := testPrompter("")
	if _, err := p.ask("Q", "default", nil); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want ErrUnexpectedEOF", err)
	}
}

func TestConfirm(t *testing.T) {
	tests := []struct {
		input      string
		def, want  bool
		transcript string
	}{
		{"\n", true, true, "? Initialize Taskship now? (Y/n): "},
		{"\n", false, false, "? Initialize Taskship now? (y/N): "},
		{"n\n", true, false, "? Initialize Taskship now? (Y/n): "},
		{"YES\n", false, true, "? Initialize Taskship now? (y/N): "},
		{"maybe\ny\n", true, true, "? Initialize Taskship now? (Y/n):   ✗ Answer y or n.\n? Initialize Taskship now? (Y/n): "},
	}
	for _, tt := range tests {
		p, out := testPrompter(tt.input)
		got, err := p.confirm("Initialize Taskship now?", tt.def)
		if err != nil || got != tt.want {
			t.Errorf("confirm(%q) = %v, %v; want %v", tt.input, got, err, tt.want)
		}
		if out.String() != tt.transcript {
			t.Errorf("confirm(%q) transcript %q, want %q", tt.input, out.String(), tt.transcript)
		}
	}
}

func TestDisplayPath(t *testing.T) {
	t.Setenv("HOME", "/Users/me")
	for in, want := range map[string]string{
		"/Users/me":                "~",
		"/Users/me/code/myproject": "~/code/myproject",
		"/Users/meg/code":          "/Users/meg/code",
		"/tmp/x":                   "/tmp/x",
		"relative/dir":             "relative/dir",
	} {
		if got := display.Path(in); got != want {
			t.Errorf("display.Path(%q) = %q, want %q", in, got, want)
		}
	}
}
