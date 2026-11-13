// Package skill installs the Taskship skill into coding agents' user-level
// skill folders. It touches only the skill's own folder, and only files
// Taskship wrote there.
package skill

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/jhdenham/taskship/internal/core"
)

//go:embed SKILL.md
var template string

// Agents lists the agents the skill can be installed for.
var Agents = []string{"claude", "codex"}

// File is the skill's only file.
const File = "SKILL.md"

// marker starts the comment Render puts in every skill it writes, which is
// how Status knows the skill is Taskship's.
const marker = "<!-- Installed by taskship "

var markerVersion = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(marker) + `(\S+)\.`)

// State is how a skill folder looks to Taskship.
type State string

const (
	NotInstalled State = "not installed"
	Installed    State = "installed" // Taskship's, the same as this version would write
	Outdated     State = "outdated"  // Taskship's, from an older or unknown version
	Newer        State = "newer"     // Taskship's, from a newer version
	Modified     State = "modified"  // Taskship's, edited since
	NotOurs      State = "not ours"  // a skill Taskship didn't write
)

// ErrNotOurs reports a skill folder Taskship didn't write.
var ErrNotOurs = errors.New("the skill there wasn't installed by Taskship")

// Render returns the skill for version of taskship. The skill runs
// "taskship" from PATH, not the binary that installed it: it outlives that
// binary and is used in sessions Taskship didn't launch.
func Render(version string) string {
	return strings.ReplaceAll(template, "{{version}}", version)
}

// Dir returns the folder the skill goes in for agent: "taskship" in
// $CLAUDE_CONFIG_DIR/skills (default ~/.claude/skills) for Claude, and in
// ~/.agents/skills for Codex. Codex also reads $CODEX_HOME/skills, but its
// docs name ~/.agents/skills, the folder shared by agents that follow the
// Agent Skills standard; it lists a skill found in both twice.
func Dir(agent string) (string, error) {
	var root, def string
	switch agent {
	case "claude":
		root, def = os.Getenv("CLAUDE_CONFIG_DIR"), ".claude"
	case "codex":
		def = ".agents"
	default:
		return "", fmt.Errorf("%w: unknown agent %q (valid: %s)", core.ErrInvalid, agent, strings.Join(Agents, ", "))
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, def)
	}
	return filepath.Join(root, "skills", "taskship"), nil
}

// Status reports the state of the skill folder dir compared with want, the
// skill this taskship would write, and the installed skill's version, if
// it's Taskship's.
func Status(dir, want string) (State, string, error) {
	got, version, err := read(dir)
	switch {
	case err != nil:
		return "", "", err
	case got == nil:
		return NotInstalled, "", nil
	case version == "":
		return NotOurs, "", nil
	case *got == want:
		return Installed, version, nil
	case newer(version, markerVersion.FindStringSubmatch(want)[1]):
		return Newer, version, nil
	case version != markerVersion.FindStringSubmatch(want)[1]:
		return Outdated, version, nil
	default:
		return Modified, version, nil
	}
}

var release = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)`)

// newer reports whether version a is a later release than b. Versions that
// aren't releases, such as "dev", are never newer.
func newer(a, b string) bool {
	ma, mb := release.FindStringSubmatch(a), release.FindStringSubmatch(b)
	if ma == nil || mb == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(ma[i])
		y, _ := strconv.Atoi(mb[i])
		if x != y {
			return x > y
		}
	}
	return false
}

// read returns the skill file in dir and its version if Taskship wrote it.
// A missing folder gives nil; a folder without the file gives "".
func read(dir string) (*string, string, error) {
	b, err := os.ReadFile(filepath.Join(dir, File))
	if errors.Is(err, fs.ErrNotExist) {
		if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
			return nil, "", nil
		} else if err != nil {
			return nil, "", err
		}
		empty := ""
		return &empty, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	got := string(b)
	if m := markerVersion.FindStringSubmatch(got); m != nil {
		return &got, m[1], nil
	}
	return &got, "", nil
}

// Install writes skill to dir, whose state Status reported. A folder holding
// a skill Taskship didn't write is replaced whole, so none of its files are
// left mixed in with Taskship's; the caller asks the user first.
func Install(dir, skill string, state State) error {
	if state == NotOurs {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, File), []byte(skill), 0o644)
}

// Uninstall removes Taskship's skill from dir: its file, then the folder if
// nothing else is in it. It reports whether there was a skill to remove. A
// skill Taskship didn't write is ErrNotOurs.
func Uninstall(dir string) (bool, error) {
	got, version, err := read(dir)
	if err != nil || got == nil {
		return false, err
	}
	if version == "" {
		return false, ErrNotOurs
	}
	if err := os.Remove(filepath.Join(dir, File)); err != nil {
		return false, err
	}
	if err := os.Remove(dir); err != nil && !errors.Is(err, syscall.ENOTEMPTY) && !errors.Is(err, fs.ErrExist) {
		return true, err
	}
	return true, nil
}
