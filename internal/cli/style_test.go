package cli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/jhdenham/taskship/internal/core"
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

func TestStateStyle(t *testing.T) {
	zero, three := 0, 3
	for _, tc := range []struct {
		run  core.Run
		want lipgloss.Style
	}{
		{core.Run{State: core.RunRunning}, value},
		{core.Run{State: core.RunStarting}, value},
		{core.Run{State: core.RunEnded, ExitCode: &zero}, lipgloss.NewStyle().Foreground(green)},
		{core.Run{State: core.RunEnded, ExitCode: &three}, lipgloss.NewStyle().Foreground(red)},
		{core.Run{State: core.RunFailed}, lipgloss.NewStyle().Foreground(red)},
	} {
		if got, want := stateStyle(tc.run).Render("x"), tc.want.Render("x"); got != want {
			t.Errorf("%s exit %v: got %q, want %q", tc.run.State, tc.run.ExitCode, got, want)
		}
	}
}

func TestBanner(t *testing.T) {
	orig := isTerminal
	t.Cleanup(func() { isTerminal = orig })

	isTerminal = func(io.Writer) bool { return false }
	var plain bytes.Buffer
	banner(&plain, "Comments (2)")
	if got, want := plain.String(), "Comments (2)\n"; got != want {
		t.Errorf("piped: got %q, want %q", got, want)
	}

	isTerminal = func(io.Writer) bool { return true }
	var rule bytes.Buffer
	banner(&rule, "Comments (2)")
	if got, want := rule.String(), "── Comments (2) "+strings.Repeat("─", 64)+"\n"; got != want {
		t.Errorf("terminal: got %q, want %q", got, want)
	}
}

func TestStatusStyle(t *testing.T) {
	for st, want := range map[core.Status]*lipgloss.Style{
		core.StatusDone:       &doneStyle,
		core.StatusInProgress: &value,
		core.StatusBacklog:    &faint,
		core.StatusCanceled:   &faint,
		core.StatusTodo:       nil,
	} {
		if got := statusStyle(st); got != want {
			t.Errorf("%s: got %v, want %v", st, got, want)
		}
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// Commands that print tables report a failed write, as they did with
// text/tabwriter.
func TestTableReportsWriteErrors(t *testing.T) {
	if err := table(failWriter{}, [][]cell{{{text: "a"}}}); err == nil || err.Error() != "disk full" {
		t.Errorf("table error = %v, want disk full", err)
	}
}
