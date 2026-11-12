// Package editor opens text in the user's editor ($VISUAL, $EDITOR, or vi).
package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Draft writes text to a new temporary file named after pattern (see
// os.CreateTemp) and returns its path. The caller removes the file.
func Draft(pattern, text string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	path := f.Name()
	_, err = f.WriteString(text)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// Command returns the command that opens path in the user's editor:
// $VISUAL, else $EDITOR (either may include arguments), else vi.
func Command(path string) *exec.Cmd {
	editor := argv()
	return exec.Command(editor[0], append(editor[1:], path)...)
}

// Name is the name of the program Command runs, without its folder or
// arguments, for labels such as "open in nvim".
func Name() string {
	return filepath.Base(argv()[0])
}

func argv() []string {
	editor := strings.Fields(os.Getenv("VISUAL"))
	if len(editor) == 0 {
		editor = strings.Fields(os.Getenv("EDITOR"))
	}
	if len(editor) == 0 {
		editor = []string{"vi"}
	}
	return editor
}
