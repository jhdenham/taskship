// Package session records long-running taskship processes, such as the TUI
// and agent runs, so that upgrade can say which to restart.
//
// Each session is a file in the sessions folder named after its process ID,
// holding a label, which the process keeps locked while it runs. The
// operating system drops the lock when the process exits, even if it
// crashes, so a file that can be locked is left over from a session that
// has ended.
package session

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

// Dir is the sessions folder inside the Taskship home directory.
const Dir = "sessions"

// Session is a running taskship process.
type Session struct {
	PID   int
	Label string // the command it runs, such as "taskship run PROJ-42"
}

// Start records this process as a session described by label in home, the
// Taskship home directory. The record lasts until stop is called or the
// process exits.
func Start(home, label string) (stop func(), err error) {
	dir := filepath.Join(home, Dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// Lock and fill the file before it gets its name, so Open never sees it
	// unlocked and removes it.
	f, err := os.CreateTemp(dir, ".new-*")
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, strconv.Itoa(os.Getpid()))
	err = func() error {
		if ok, err := tryLock(f); err != nil || !ok {
			return errors.Join(err, errors.New("couldn't lock the session file"))
		}
		if _, err := io.WriteString(f, label); err != nil {
			return err
		}
		return os.Rename(f.Name(), path)
	}()
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return func() {
		os.Remove(path)
		f.Close()
	}, nil
}

// Open returns the sessions still running in home, the Taskship home
// directory, and removes the records of those that have ended. It creates
// nothing.
func Open(home string) ([]Session, error) {
	dir := filepath.Join(home, Dir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sessions []Session
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		ended, err := tryLock(f)
		switch {
		case err != nil:
		case ended:
			os.Remove(path)
		default:
			label, err := io.ReadAll(f)
			if err == nil {
				sessions = append(sessions, Session{pid, string(label)})
			}
		}
		f.Close()
	}
	return sessions, nil
}
