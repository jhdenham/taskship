// Package display formats text for people, shared by the CLI and the TUI.
package display

import (
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
)

// Path shortens an absolute path in the home folder to ~/..., for
// human-readable output. JSON output keeps absolute paths.
func Path(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}
	return path
}

// Rule is a section heading across width cells: "── Comments (2) ─────",
// with at least three dashes after the title.
func Rule(title string, width int) string {
	return "── " + title + " " + strings.Repeat("─", max(width-lipgloss.Width(title)-4, 3))
}
