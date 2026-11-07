package cli

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestPrintError(t *testing.T) {
	orig := isTerminal
	t.Cleanup(func() { isTerminal = orig })
	err := errors.New("no project here")

	isTerminal = func(io.Writer) bool { return false }
	var plain bytes.Buffer
	printError(&plain, err)
	if got, want := plain.String(), "taskship: no project here\n"; got != want {
		t.Errorf("piped: got %q, want %q", got, want)
	}

	isTerminal = func(io.Writer) bool { return true }
	var banner bytes.Buffer
	printError(&banner, err)
	if got, want := banner.String(), "\n ERROR  no project here\n"; got != want {
		t.Errorf("terminal: got %q, want %q", got, want)
	}
}

func TestWrap(t *testing.T) {
	got := wrap("pass -y to accept the defaults, or --name and --prefix", 20, "  ")
	want := "pass -y to accept\n  the defaults, or\n  --name and --prefix"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
