package tui

import (
	"context"
	"flag"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jhdenham/taskship/internal/core"
)

var capture = flag.String("capture", "", "write the website's TUI captures as HTML to this folder")

// TestCaptures writes colored screens for the website and the docs'
// screenshots (website/scripts/tui-screenshots.sh runs it). Without
// -capture it only checks that they render.
func TestCaptures(t *testing.T) {
	// The launch menu needs the agent on PATH and the project's folder.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	for _, c := range []struct {
		name string
		h    int
		keys []string
	}{
		{"tui-board", 32, []string{"l", "l", "j", "enter"}},
		{"tui-list", 24, []string{"2", "g", "j"}},
		{"tui-launch", 32, []string{"l", "l", "j", "enter", "r"}},
		{"tui-comment", 32, []string{"l", "l", "j", "enter", "c", "text:Persistence is unblocked; picking it up next."}},
	} {
		m, s := myproject(t, 140, c.h)
		if _, err := s.AddWorkspace(context.Background(), "myproject", core.Workspace{Path: t.TempDir()}); err != nil {
			t.Fatal(err)
		}
		press(m, c.keys...)
		out := toHTML(m.render())
		if *capture == "" {
			continue
		}
		head := fmt.Sprintf("<!-- The TUI at 140x%d, dark theme, after pressing %s on the seeded data of internal/tui/tui_test.go. Written by website/scripts/tui-screenshots.sh; don't edit. -->\n", c.h, strings.Join(c.keys, " "))
		if err := os.WriteFile(filepath.Join(*capture, c.name+".html"), []byte(head+out), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// toHTML turns the ANSI styles Lip Gloss writes (bold, 24-bit foreground and
// background, reset) into spans. Symbols other than box drawing are wrapped
// in class "ch" so the page can hold them to one column.
func toHTML(s string) string {
	var b strings.Builder
	var fg, bg string
	var bold, open bool
	flush := func() {
		if open {
			b.WriteString("</span>")
			open = false
		}
		var css []string
		if fg != "" {
			css = append(css, "color:"+fg)
		}
		if bg != "" {
			css = append(css, "background:"+bg)
		}
		if bold {
			css = append(css, "font-weight:700")
		}
		if len(css) > 0 {
			b.WriteString(`<span style="` + strings.Join(css, ";") + `">`)
			open = true
		}
	}
	for len(s) > 0 {
		if strings.HasPrefix(s, "\x1b[") {
			end := strings.IndexByte(s, 'm')
			p := strings.Split(s[2:end], ";")
			s = s[end+1:]
			for i := 0; i < len(p); i++ {
				switch p[i] {
				case "", "0":
					fg, bg, bold = "", "", false
				case "1":
					bold = true
				case "22":
					bold = false
				case "39":
					fg = ""
				case "49":
					bg = ""
				case "38", "48":
					rgb := hex(p[i+2 : i+5])
					if p[i] == "38" {
						fg = rgb
					} else {
						bg = rgb
					}
					i += 4
				}
			}
			flush()
			continue
		}
		r, size := utf8.DecodeRuneInString(s)
		text := html.EscapeString(s[:size])
		if r >= 0x2190 && (r < 0x2500 || r > 0x257f) {
			text = `<span class="ch">` + text + `</span>`
		}
		b.WriteString(text)
		s = s[size:]
	}
	if open {
		b.WriteString("</span>")
	}
	return b.String()
}

func hex(rgb []string) string {
	out := "#"
	for _, c := range rgb {
		n, _ := strconv.Atoi(c)
		out += fmt.Sprintf("%02x", n)
	}
	return out
}
