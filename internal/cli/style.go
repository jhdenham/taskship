package cli

import (
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

// Styles for human-readable output. Print styled text with lipgloss.Fprint
// and friends, which drop the colors when the writer isn't a color terminal
// (a pipe, a file, a test buffer) or NO_COLOR is set.
var (
	accent   = lipgloss.Color("#7571F9")
	pink     = lipgloss.Color("#F780E2")
	green    = lipgloss.Color("#02BF87")
	red      = lipgloss.Color("#ED567A")
	subtle   = lipgloss.Color("243")
	heading  = lipgloss.NewStyle().Bold(true)
	faint    = lipgloss.NewStyle().Foreground(subtle)
	command  = lipgloss.NewStyle().Foreground(pink)
	value    = lipgloss.NewStyle().Foreground(accent)
	okMark   = lipgloss.NewStyle().Foreground(green).Bold(true).Render("✓")
	errMark  = lipgloss.NewStyle().Foreground(red).Bold(true).Render("✗")
	askMark  = lipgloss.NewStyle().Foreground(green).Bold(true).Render("?")
	errLabel = lipgloss.NewStyle().Foreground(red).Bold(true).Render("taskship:")
	errBadge = lipgloss.NewStyle().Background(red).Foreground(lipgloss.Color("#FFFDF5")).Bold(true).Padding(0, 1).Render("ERROR")
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
	width := 80
	if f, ok := w.(*os.File); ok {
		if cols, _, e := term.GetSize(f.Fd()); e == nil && cols > 0 {
			width = min(width, cols)
		}
	}
	indent := lipgloss.Width(errBadge) + 1
	lipgloss.Fprint(w, "\n"+errBadge+" "+wrap(err.Error(), width-indent, strings.Repeat(" ", indent))+"\n")
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
	return value.Render(displayPath(path))
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
