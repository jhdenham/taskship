package dispatch

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// TaskshipCommand returns the command an agent should run to reach this
// taskship: "taskship" when that name on PATH is the running executable, or
// else the executable's absolute path, quoted for a shell if needed. It is
// worked out on every launch, so a build run as ./bin/taskship, or a
// different version installed on PATH, never stands in for this one.
func TaskshipCommand() string {
	self, err := os.Executable()
	if err != nil {
		return "taskship"
	}
	selfInfo, err := os.Stat(self)
	if err != nil {
		return shellQuote(self)
	}
	if onPath, err := exec.LookPath("taskship"); err == nil {
		if info, err := os.Stat(onPath); err == nil && os.SameFile(info, selfInfo) {
			return "taskship"
		}
	}
	return shellQuote(self)
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_./+-]+$`)

// shellQuote quotes s for a POSIX shell unless it needs no quoting.
func shellQuote(s string) string {
	if shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
