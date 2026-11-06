package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// success prints a line reporting that something was done, marked with ✓.
func success(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "✓ "+format+"\n", a...)
}

// displayPath shortens an absolute path in the home directory to ~/..., for
// human-readable output. JSON output keeps absolute paths.
func displayPath(path string) string {
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
