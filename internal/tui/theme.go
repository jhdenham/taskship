package tui

import (
	"hash/fnv"
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/jhdenham/taskship/internal/core"
)

// theme holds the TUI's styles for a dark or light terminal. The colors are
// the terminal roles of the shared palette (website/src/styles/global.css).
// Bubble Tea drops colors the terminal can't show, and all of them under
// NO_COLOR, so nothing here may be the only sign of a state: selection and
// focus also use markers, bold text and heavy borders.
type theme struct {
	plain    lipgloss.Style
	key      lipgloss.Style // keys in footers and actions; the accent
	keyBold  lipgloss.Style
	faint    lipgloss.Style
	value    lipgloss.Style // names, paths, mentioned refs
	ok       lipgloss.Style
	err      lipgloss.Style
	bold     lipgloss.Style
	sel      lipgloss.Style // background of the selected card or row
	border   lipgloss.Style
	focus    lipgloss.Style // border of the focused column or pane
	labels   []lipgloss.Style
	heading  lipgloss.Style // Markdown headings and section titles
	selected color.Color
}

func newTheme(dark bool) theme {
	ld := lipgloss.LightDark(dark)
	accent := ld(lipgloss.Color("#1f6b45"), lipgloss.Color("#9cf2a6"))
	success := ld(lipgloss.Color("#237a45"), lipgloss.Color("#5fd38a"))
	value := ld(lipgloss.Color("#8f5b00"), lipgloss.Color("#f0c674"))
	errc := ld(lipgloss.Color("#c2412d"), lipgloss.Color("#ff8a7a"))
	faint := ld(lipgloss.Color("#5f7068"), lipgloss.Color("#8fa197"))
	line := ld(lipgloss.Color("#c5d0c8"), lipgloss.Color("#2c473b"))
	tint := ld(lipgloss.Color("#e1efe4"), lipgloss.Color("#173026"))
	s := lipgloss.NewStyle
	t := theme{
		key:      s().Foreground(accent),
		keyBold:  s().Foreground(accent).Bold(true),
		faint:    s().Foreground(faint),
		value:    s().Foreground(value),
		ok:       s().Foreground(success),
		err:      s().Foreground(errc),
		bold:     s().Bold(true),
		sel:      s().Background(tint),
		border:   s().Foreground(line),
		focus:    s().Foreground(accent),
		heading:  s().Foreground(accent).Bold(true),
		selected: tint,
	}
	for _, c := range [][2]string{
		{"#8f5b00", "#f0c674"}, // yellow
		{"#6a4fb5", "#b8a4f0"}, // purple
		{"#1f7a6e", "#7fd4c8"}, // teal
		{"#a8336b", "#f2a7c8"}, // pink
		{"#2f5f9e", "#8fb8f0"}, // blue
	} {
		t.labels = append(t.labels, s().Foreground(ld(lipgloss.Color(c[0]), lipgloss.Color(c[1]))))
	}
	return t
}

// label renders "#name" in a color picked by a hash of the name, so a label
// keeps its color everywhere.
func (t theme) label(name string) string {
	h := fnv.New32a()
	h.Write([]byte(name))
	return t.labels[h.Sum32()%uint32(len(t.labels))].Render("#" + name)
}

// labels renders a task's labels separated by spaces.
func (t theme) labelList(labels []string) string {
	out := ""
	for i, l := range labels {
		if i > 0 {
			out += t.plain.Render(" ")
		}
		out += t.label(l)
	}
	return out
}

// status renders a status name ("in progress", not "in_progress").
func (t theme) status(st core.Status) string {
	name := statusName(st)
	switch st {
	case core.StatusInProgress:
		return t.value.Bold(true).Render(name)
	case core.StatusDone:
		return t.ok.Render(name)
	case core.StatusBacklog, core.StatusCanceled:
		return t.faint.Render(name)
	}
	return t.bold.Render(name)
}

// priority renders a priority, "—" for none.
func (t theme) priority(p core.Priority) string {
	switch p {
	case core.PriorityUrgent:
		return t.err.Bold(true).Render("urgent")
	case core.PriorityHigh:
		return t.value.Render("high")
	case core.PriorityLow:
		return t.faint.Render("low")
	case core.PriorityNone:
		return t.faint.Render("—")
	}
	return t.plain.Render(string(p))
}

// onSelection returns the theme with every style drawn on the selection's
// background, for the text of a selected card or row.
func (t theme) onSelection() theme {
	bg := func(s *lipgloss.Style) { *s = s.Background(t.selected) }
	for _, s := range []*lipgloss.Style{&t.plain, &t.key, &t.keyBold, &t.faint, &t.value, &t.ok, &t.err, &t.bold, &t.border, &t.focus, &t.heading} {
		bg(s)
	}
	t.labels = append([]lipgloss.Style(nil), t.labels...)
	for i := range t.labels {
		bg(&t.labels[i])
	}
	return t
}

// keys renders footer-style key hints: pairs of key and description.
func (t theme) keys(pairs ...string) string {
	return t.keysIn(t.keyBold, pairs...)
}

// questionKeys renders the answer keys of a question, in the error color
// like the question.
func (t theme) questionKeys(pairs ...string) string {
	return t.keysIn(t.err.Bold(true), pairs...)
}

func (t theme) keysIn(key lipgloss.Style, pairs ...string) string {
	out := ""
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			out += "  "
		}
		out += key.Render(pairs[i])
		if pairs[i+1] != "" {
			out += " " + t.faint.Render(pairs[i+1])
		}
	}
	return out
}

func statusName(st core.Status) string {
	if st == core.StatusInProgress {
		return "in progress"
	}
	return string(st)
}

// statusGlyph marks a status in the list view.
func statusGlyph(st core.Status) string {
	switch st {
	case core.StatusBacklog:
		return "◇"
	case core.StatusTodo:
		return "○"
	case core.StatusInProgress:
		return "◐"
	case core.StatusDone:
		return "●"
	}
	return "×"
}
