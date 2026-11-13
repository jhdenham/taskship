// Package guide holds Taskship's guide to writing tasks and loads the
// user's own conventions, which agents follow over the guide.
package guide

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jhdenham/taskship/internal/display"
	"github.com/jhdenham/taskship/internal/home"
)

// Tasks is the built-in guide to writing a task that's ready to work on.
//
//go:embed tasks.md
var Tasks string

// ConventionsFile is the user's conventions file in Taskship's home folder.
// Taskship reads it but never creates it.
const ConventionsFile = "conventions.md"

// Conventions returns the user's conventions, trimmed, or "" if there are
// none, and the file's path. A missing file isn't an error; an error reading
// it names the file.
func Conventions() (text, path string, err error) {
	dir, err := home.Dir()
	if err != nil {
		return "", "", err
	}
	path = filepath.Join(dir, ConventionsFile)
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", path, nil
	}
	if err != nil {
		return "", path, fmt.Errorf("couldn't read your conventions in %s: %w", display.Path(path), err)
	}
	return strings.TrimSpace(string(b)), path, nil
}

// Text returns the guide followed by the user's conventions from path, or a
// note on where the user could add some.
func Text(conventions, path string) string {
	if conventions == "" {
		return Tasks + "\n## The user's conventions\n\nNone yet. The user can add their own in " + display.Path(path) + ".\n"
	}
	return Tasks + "\n## The user's conventions\n\nFrom " + display.Path(path) +
		". Where they differ from the guide above, follow them.\n\n" + conventions + "\n"
}
