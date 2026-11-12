package cli

import (
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
)

// success prints a line reporting that something was done, marked with ✓.
func success(w io.Writer, format string, a ...any) {
	lipgloss.Fprintln(w, okMark, fmt.Sprintf(format, a...))
}
