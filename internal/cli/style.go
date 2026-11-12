package cli

import (
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/jhdenham/taskship/internal/core"
	"github.com/jhdenham/taskship/internal/display"
)

// Styles for human-readable output. Print styled text with lipgloss.Fprint
// and friends, which drop the colors when the writer isn't a color terminal
// (a pipe, a file, a test buffer) or NO_COLOR is set.
var (
	accent    = lipgloss.Color("#7571F9")
	pink      = lipgloss.Color("#F780E2")
	green     = lipgloss.Color("#02BF87")
	red       = lipgloss.Color("#ED567A")
	subtle    = lipgloss.Color("243")
	heading   = lipgloss.NewStyle().Bold(true)
	faint     = lipgloss.NewStyle().Foreground(subtle)
	command   = lipgloss.NewStyle().Foreground(pink)
	doneStyle = lipgloss.NewStyle().Foreground(green)
	failStyle = lipgloss.NewStyle().Foreground(red)
	value     = lipgloss.NewStyle().Foreground(accent)
	okMark    = lipgloss.NewStyle().Foreground(green).Bold(true).Render("✓")
	errMark   = lipgloss.NewStyle().Foreground(red).Bold(true).Render("✗")
	askMark   = lipgloss.NewStyle().Foreground(green).Bold(true).Render("?")
	errLabel  = lipgloss.NewStyle().Foreground(red).Bold(true).Render("taskship:")
	errBadge  = lipgloss.NewStyle().Background(red).Foreground(lipgloss.Color("#FFFDF5")).Bold(true).Padding(0, 1).Render("ERROR")
)

// printError prints err to w. In a terminal, it is a blank line and an ERROR
// badge with the message beside it, wrapped to the terminal (at most 80
// columns). Anywhere else, it is one plain "taskship: message" line for
// scripts and logs.
func printError(w io.Writer, err error) {
	if !isTerminal(w) {
		lipgloss.Fprintln(w, errLabel, err)
		return
	}
	width := termWidth(w)
	indent := lipgloss.Width(errBadge) + 1
	lipgloss.Fprint(w, "\n"+errBadge+" "+wrap(err.Error(), width-indent, strings.Repeat(" ", indent))+"\n")
}

// termWidth is the width of the terminal w, at most 80 columns.
func termWidth(w io.Writer) int {
	width := 80
	if f, ok := w.(*os.File); ok {
		if cols, _, e := term.GetSize(f.Fd()); e == nil && cols > 0 {
			width = min(width, cols)
		}
	}
	return width
}

// banner prints title as a section heading. In a terminal, it is a muted
// rule across the terminal (at most 80 columns) with the title near the
// start: "── Comments (2) ─────". Anywhere else, it is just the title, bold.
func banner(w io.Writer, title string) {
	if !isTerminal(w) {
		lipgloss.Fprintln(w, heading.Render(title))
		return
	}
	lipgloss.Fprintln(w, faint.Render(display.Rule(title, termWidth(w))))
}

// cell is a table cell: its text and, if not nil, a style for it.
type cell struct {
	text  string
	style *lipgloss.Style
}

// table prints rows as columns two spaces apart, like text/tabwriter, but
// aligned by the cells' visible text, so styled cells line up. It returns
// the write error, if any.
func table(w io.Writer, rows [][]cell) error {
	var widths []int
	for _, row := range rows {
		for i, c := range row {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], lipgloss.Width(c.text))
		}
	}
	var b strings.Builder
	for _, row := range rows {
		for i, c := range row {
			text := c.text
			if c.style != nil && text != "" {
				text = c.style.Render(text)
			}
			if i == len(row)-1 {
				b.WriteString(text + "\n")
			} else {
				b.WriteString(text + strings.Repeat(" ", widths[i]-lipgloss.Width(c.text)+2))
			}
		}
	}
	_, err := lipgloss.Fprint(w, b.String())
	return err
}

// header is a table's header row: plain cells.
func header(names ...string) []cell {
	row := make([]cell, len(names))
	for i, n := range names {
		row[i] = cell{text: n}
	}
	return row
}

// statusStyle colors a task status: green when done, accent in progress,
// muted in the backlog or canceled, and nil (plain) for todo.
func statusStyle(st core.Status) *lipgloss.Style {
	switch st {
	case core.StatusDone:
		return &doneStyle
	case core.StatusInProgress:
		return &value
	case core.StatusBacklog, core.StatusCanceled:
		return &faint
	}
	return nil
}

// wrap breaks each line of s at spaces, at most width columns where the
// words allow, and starts each line after the first with indent.
func wrap(s string, width int, indent string) string {
	var b strings.Builder
	for l, line := range strings.Split(s, "\n") {
		if l > 0 {
			b.WriteString("\n" + indent)
		}
		n := 0
		for i, word := range strings.Fields(line) {
			w := lipgloss.Width(word)
			if i > 0 && n+1+w > width {
				b.WriteString("\n" + indent)
				n = 0
			} else if i > 0 {
				b.WriteString(" ")
				n++
			}
			b.WriteString(word)
			n += w
		}
	}
	return b.String()
}

// pathRef shortens a path like displayPath and colors it as a value.
func pathRef(path string) string {
	return value.Render(display.Path(path))
}

// headingf formats a heading in bold, with each %s argument in the value
// color. (Styling a whole formatted line would lose the bold after the
// first colored argument.)
func headingf(format string, args ...string) string {
	var b strings.Builder
	for i, part := range strings.Split(format, "%s") {
		b.WriteString(heading.Render(part))
		if i < len(args) {
			b.WriteString(heading.Foreground(accent).Render(args[i]))
		}
	}
	return b.String()
}

// cmdRef quotes a command in running text and colors it like the commands
// in a list of next steps.
func cmdRef(c string) string {
	return `"` + command.Render(c) + `"`
}

// columns prints rows as two aligned columns, indented by two spaces, with
// gap spaces after the widest first cell. Unlike text/tabwriter, it aligns
// styled text by its visible width.
func columns(w io.Writer, gap int, rows [][2]string) {
	width := 0
	for _, r := range rows {
		width = max(width, lipgloss.Width(r[0]))
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString("  " + r[0] + strings.Repeat(" ", width-lipgloss.Width(r[0])+gap) + r[1] + "\n")
	}
	lipgloss.Fprint(w, b.String())
}
