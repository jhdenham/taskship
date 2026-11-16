//go:build unix

package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStartOpen(t *testing.T) {
	home := filepath.Join(t.TempDir(), "taskship-home")
	if got, err := Open(home); got != nil || err != nil {
		t.Errorf("Open without a folder = %v, %v", got, err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Errorf("Open created %s", home)
	}

	stop, err := Start(home, "taskship run PROJ-42")
	if err != nil {
		t.Fatal(err)
	}
	// A record left by a session that ended, and a file that isn't a record.
	dir := filepath.Join(home, Dir)
	os.WriteFile(filepath.Join(dir, "999999"), []byte("taskship"), 0o600)
	os.WriteFile(filepath.Join(dir, "notes"), nil, 0o600)

	got, err := Open(home)
	want := []Session{{os.Getpid(), "taskship run PROJ-42"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Open = %v, %v; want %v", got, err, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "999999")); !os.IsNotExist(err) {
		t.Errorf("ended session's record kept: %v", err)
	}
	// Open doesn't disturb a running session.
	if got, _ := Open(home); len(got) != 1 {
		t.Errorf("second Open = %v", got)
	}

	stop()
	if got, err := Open(home); got != nil || err != nil {
		t.Errorf("Open after stop = %v, %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "notes" {
		t.Errorf("left in %s: %v", dir, entries)
	}
}
