package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/jhdenham/taskship/internal/store"
)

// olderDatabase creates the database in a fresh Taskship home with every
// migration but the last, as the previous release would have left it, and
// returns the home and its schema version.
func olderDatabase(t *testing.T) (string, int) {
	t.Helper()
	home := tempHome(t)
	files, err := filepath.Glob("../store/migrations/*.sql")
	if err != nil || len(files) < 2 {
		t.Fatalf("migrations: %v, %v", files, err)
	}
	os.MkdirAll(home, 0o700)
	db, err := sql.Open("sqlite", filepath.Join(home, store.DBFile))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	version := len(files) - 1
	for _, f := range files[:version] {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		t.Fatal(err)
	}
	return home, version
}

func TestMigrationBackupNotice(t *testing.T) {
	home, version := olderDatabase(t)
	out := run(t, "", "project", "list")
	want := regexp.MustCompile(`^Backed up your Taskship data to ` + regexp.QuoteMeta(filepath.Join(home, store.BackupDir)) +
		fmt.Sprintf(`/schema-%d-\d{8}-\d{6}\.db before updating it\.\n$`, version))
	if out.code != 0 || !want.MatchString(out.stderr) {
		t.Errorf("%+v", out)
	}
	if out := run(t, "", "project", "list"); out.stderr != "" {
		t.Errorf("notice without a migration: %q", out.stderr)
	}
}

func TestMigrationBackupFails(t *testing.T) {
	home, _ := olderDatabase(t)
	os.WriteFile(filepath.Join(home, store.BackupDir), nil, 0o600)
	out := run(t, "", "project", "list")
	want := regexp.MustCompile(`^taskship: couldn't back up ` + regexp.QuoteMeta(filepath.Join(home, store.DBFile)) +
		` before updating it: .+\n\nYour data wasn't changed\.\n$`)
	if out.code != exitFailure || !want.MatchString(out.stderr) {
		t.Errorf("%+v", out)
	}
}

func TestNewerDatabase(t *testing.T) {
	home := tempHome(t)
	mustRun(t, "project", "list")
	db, err := sql.Open("sqlite", filepath.Join(home, store.DBFile))
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("PRAGMA user_version = 99")
	db.Close()
	files, _ := filepath.Glob("../store/migrations/*.sql")
	out := run(t, "v0.1.0", "project", "list")
	want := fmt.Sprintf("taskship: %s was updated by a newer taskship version (schema 99; taskship v0.1.0 supports up to %d)\n\nRun \"taskship upgrade\" to install the latest version.\n",
		filepath.Join(home, store.DBFile), len(files))
	if out.code != exitFailure || out.stderr != want {
		t.Errorf("%+v\nwant stderr:\n%s", out, want)
	}
}
