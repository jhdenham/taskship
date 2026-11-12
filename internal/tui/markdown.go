package tui

import (
	"fmt"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

// Markdown rendering for the detail view. It covers what task descriptions
// and comments use: headings, paragraphs, lists, checklists, quotes, fenced
// code, and inline code, bold and task refs. As in Markdown, a single line
// break inside a paragraph, list item or quote is a space, so text
// hard-wrapped in an editor rewraps to the pane; a line ending in two spaces
// or a backslash keeps its break.

var (
	headingRE   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	checkRE     = regexp.MustCompile(`^(\s*)[-*+]\s+\[([ xX])\]\s+(.*)$`)
	bulletRE    = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	numberedRE  = regexp.MustCompile(`^(\s*)(\d+[.)])\s+(.*)$`)
	inlineRE    = regexp.MustCompile("\\*\\*([^*]+)\\*\\*|`([^`]+)`|\\b([A-Z][A-Z0-9]{0,9}-[1-9][0-9]*)\\b")
	fenceStarts = []string{"```", "~~~"}
)

// mdBlock is a paragraph, list item or quote being collected: its first
// line's prefix (a bullet, a checkbox), the prefix of its other lines, and
// its text, split at hard line breaks.
type mdBlock struct {
	prefix, cont string
	parts        []string
	broke        bool // the last line ended with a hard break
}

// add appends a source line to the block's text.
func (b *mdBlock) add(line string) {
	hard := strings.HasSuffix(line, "  ") || strings.HasSuffix(strings.TrimSpace(line), "\\")
	text := strings.TrimSuffix(strings.TrimSpace(line), "\\")
	if len(b.parts) == 0 || b.broke {
		b.parts = append(b.parts, text)
	} else {
		b.parts[len(b.parts)-1] += " " + text
	}
	b.broke = hard
}

// renderMarkdown renders src as lines at most width cells wide.
func renderMarkdown(src string, width int, th theme) []string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(strings.Trim(src, "\n"), "\n")
	var out []string
	var block *mdBlock
	flush := func() {
		if block == nil {
			return
		}
		for i, part := range block.parts {
			prefix := block.prefix
			if i > 0 {
				prefix = block.cont
			}
			out = append(out, hang(prefix, block.cont, part, width, th)...)
		}
		block = nil
	}
	start := func(prefix, cont, text string) {
		flush()
		block = &mdBlock{prefix: prefix, cont: cont}
		block.add(text)
	}
	inFence, afterHeading := false, false
	for i, line := range lines {
		wasHeading := afterHeading
		afterHeading = false
		trimmed := strings.TrimSpace(line)
		if isFence(trimmed) {
			flush()
			inFence = !inFence
			continue
		}
		if inFence {
			out = append(out, th.value.Render(truncate("  "+line, width)))
			continue
		}
		if trimmed == "" {
			flush()
			if len(out) > 0 && out[len(out)-1] != "" && !wasHeading {
				out = append(out, "")
			}
			afterHeading = wasHeading
			continue
		}
		if g := headingRE.FindStringSubmatch(line); g != nil {
			flush()
			h := th.heading.Render(g[2])
			if done, total := checklist(lines[i+1:]); total > 0 {
				h += th.faint.Render(fmt.Sprintf("   %d of %d done", done, total))
			}
			out = append(out, truncate(h, width))
			afterHeading = true
			continue
		}
		if g := checkRE.FindStringSubmatch(line); g != nil {
			box := th.faint.Render("[ ] ")
			if g[2] != " " {
				box = th.ok.Render("[x] ")
			}
			start(g[1]+box, strings.Repeat(" ", len(g[1])+4), g[3])
			continue
		}
		if g := bulletRE.FindStringSubmatch(line); g != nil {
			start(g[1]+th.faint.Render("• "), strings.Repeat(" ", len(g[1])+2), g[2])
			continue
		}
		if g := numberedRE.FindStringSubmatch(line); g != nil {
			start(g[1]+th.faint.Render(g[2]+" "), strings.Repeat(" ", len(g[1])+len(g[2])+1), g[3])
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, ">"); ok {
			if block == nil || block.cont != th.faint.Render("│ ") {
				bar := th.faint.Render("│ ")
				start(bar, bar, rest)
			} else {
				block.add(rest)
			}
			continue
		}
		if block != nil {
			block.add(line) // a continuation line
			continue
		}
		start("", "", line)
	}
	flush()
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func isFence(s string) bool {
	for _, f := range fenceStarts {
		if strings.HasPrefix(s, f) {
			return true
		}
	}
	return false
}

// checklist counts the checked and all checklist items before the next
// heading.
func checklist(lines []string) (done, total int) {
	for _, l := range lines {
		if headingRE.MatchString(l) {
			break
		}
		if g := checkRE.FindStringSubmatch(l); g != nil {
			total++
			if g[2] != " " {
				done++
			}
		}
	}
	return done, total
}

// hang wraps text after prefix, starting the following lines with cont,
// which is as wide as prefix.
func hang(prefix, cont, text string, width int, th theme) []string {
	indent := lipgloss.Width(prefix)
	wrapped := wrapWords(text, max(10, width-indent))
	out := make([]string, len(wrapped))
	for i, w := range wrapped {
		lead := cont
		if i == 0 {
			lead = prefix
		}
		out[i] = lead + inline(w, th)
	}
	return out
}

// inline styles **bold**, `code` and task refs in one line.
func inline(s string, th theme) string {
	var b strings.Builder
	last := 0
	for _, g := range inlineRE.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(th.plain.Render(s[last:g[0]]))
		switch {
		case g[2] >= 0:
			b.WriteString(th.bold.Render(s[g[2]:g[3]]))
		case g[4] >= 0:
			b.WriteString(th.value.Render(s[g[4]:g[5]]))
		default:
			b.WriteString(th.value.Render(s[g[6]:g[7]]))
		}
		last = g[1]
	}
	b.WriteString(th.plain.Render(s[last:]))
	return b.String()
}

// wrapWords breaks s at spaces into lines at most width cells wide,
// breaking words longer than a line.
func wrapWords(s string, width int) []string {
	var lines []string
	cur := ""
	for _, word := range strings.Fields(s) {
		for lipgloss.Width(word) > width {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			cut := []rune(word)
			n := 0
			for n < len(cut) && lipgloss.Width(string(cut[:n+1])) <= width {
				n++
			}
			lines = append(lines, string(cut[:n]))
			word = string(cut[n:])
		}
		switch {
		case cur == "":
			cur = word
		case lipgloss.Width(cur)+1+lipgloss.Width(word) <= width:
			cur += " " + word
		default:
			lines = append(lines, cur)
			cur = word
		}
	}
	if cur != "" || len(lines) == 0 {
		lines = append(lines, cur)
	}
	return lines
}
