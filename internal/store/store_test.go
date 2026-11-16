package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/jhdenham/taskship/internal/core"
)

func openTest(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(context.Background(), dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func userVersion(t *testing.T, s *Store) int {
	t.Helper()
	var v int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestOpenCreatesHomeAndMigrates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "home")
	s := openTest(t, dir)
	if v := userVersion(t, s); v != 10 {
		t.Errorf("user_version = %d, want 10", v)
	}
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM projects").Scan(&n); err != nil {
		t.Fatalf("projects table missing: %v", err)
	}
}

func TestReopenPreservesDataAndSchema(t *testing.T) {
	dir := t.TempDir()
	s := openTest(t, dir)
	if _, err := s.db.Exec(`INSERT INTO projects (id, slug, task_prefix, created_at, updated_at)
		VALUES ('p1', 'myproject', 'PROJ', 'now', 'now')`); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s = openTest(t, dir)
	if v := userVersion(t, s); v != 10 {
		t.Errorf("user_version after reopen = %d, want 10", v)
	}
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM projects").Scan(&n); err != nil || n != 1 {
		t.Errorf("projects after reopen = %d (err %v), want 1", n, err)
	}
}

// A database created before the display name was dropped keeps its projects
// and loses the name column.
func TestMigrateDropsProjectDisplayName(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, DBFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"migrations/001_init.sql", "migrations/002_workspaces.sql"} {
		b, err := migrations.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`PRAGMA user_version = 2;
		INSERT INTO projects (id, slug, name, task_prefix, created_at, updated_at)
		VALUES ('p1', 'myproject', 'My Project', 'PROJ', 'now', 'now')`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s := openTest(t, dir)
	if v := userVersion(t, s); v != 10 {
		t.Errorf("user_version = %d, want 10", v)
	}
	var slug string
	if err := s.db.QueryRow("SELECT slug FROM projects WHERE id = 'p1'").Scan(&slug); err != nil || slug != "myproject" {
		t.Errorf("project after migration = %q, %v", slug, err)
	}
	if _, err := s.db.Exec("SELECT name FROM projects"); err == nil {
		t.Error("projects.name still exists")
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, DBFile))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	db.Close()

	_, err = Open(context.Background(), dir)
	var newer *NewerSchemaError
	if !errors.As(err, &newer) || newer.Version != 999 || newer.Supported != 10 || newer.Path != filepath.Join(dir, DBFile) {
		t.Fatalf("Open with newer schema: err = %v, want NewerSchemaError", err)
	}
	if _, err := os.Stat(filepath.Join(dir, BackupDir)); !os.IsNotExist(err) {
		t.Errorf("backup folder created for a newer schema: %v", err)
	}
}

// oldDatabase creates a database in dir with only the first n migrations,
// as an older taskship left it, holding one project.
func oldDatabase(t *testing.T, dir string, n int) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, DBFile))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	files, _ := fs.Glob(migrations, "migrations/*.sql")
	for _, f := range files[:n] {
		b, err := migrations.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d;
		INSERT INTO projects (id, slug, task_prefix, created_at, updated_at)
		VALUES ('p1', 'myproject', 'PROJ', 'now', 'now')`, n)); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateBacksUpFirst(t *testing.T) {
	dir := t.TempDir()
	oldDatabase(t, dir, 9)
	s := openTest(t, dir)
	if v := userVersion(t, s); v != 10 {
		t.Errorf("user_version = %d, want 10", v)
	}
	backup := s.Backup()
	if ok, _ := filepath.Match(filepath.Join(dir, BackupDir, "schema-9-*.db"), backup); !ok {
		t.Fatalf("backup = %q", backup)
	}
	db, err := sql.Open("sqlite", backup)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version, projects int
	db.QueryRow("PRAGMA user_version").Scan(&version)
	db.QueryRow("SELECT count(*) FROM projects").Scan(&projects)
	if version != 9 || projects != 1 {
		t.Errorf("backup has schema %d and %d projects, want 9 and 1", version, projects)
	}

	// Nothing to migrate: no backup.
	s.Close()
	if s := openTest(t, dir); s.Backup() != "" {
		t.Errorf("backup without a migration: %q", s.Backup())
	}
	// A new database has nothing to back up.
	if s := openTest(t, t.TempDir()); s.Backup() != "" {
		t.Errorf("backup of a new database: %q", s.Backup())
	}
}

func TestMigrateStopsWhenBackupFails(t *testing.T) {
	dir := t.TempDir()
	oldDatabase(t, dir, 9)
	if err := os.WriteFile(filepath.Join(dir, BackupDir), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(context.Background(), dir)
	var backupErr *BackupError
	if !errors.As(err, &backupErr) || backupErr.Path != filepath.Join(dir, DBFile) {
		t.Fatalf("err = %v, want BackupError", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, DBFile))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if db.QueryRow("PRAGMA user_version").Scan(&version); version != 9 {
		t.Errorf("user_version = %d after a failed backup, want 9", version)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	s := openTest(t, t.TempDir())
	_, err := s.db.Exec(`INSERT INTO tasks (id, project_id, sequence_number, title, created_at, updated_at)
		VALUES ('t1', 'missing', 1, 'x', 'now', 'now')`)
	if err == nil {
		t.Fatal("insert with unknown project_id succeeded; foreign keys not enforced")
	}
}

func TestMigrateNumbersExistingRuns(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, DBFile))
	if err != nil {
		t.Fatal(err)
	}
	files, _ := fs.Glob(migrations, "migrations/*.sql")
	for _, f := range files[:7] {
		b, err := migrations.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
	}
	if _, err := db.Exec(`PRAGMA user_version = 7;
		INSERT INTO projects (id, slug, task_prefix, created_at, updated_at) VALUES ('p1', 'myproject', 'PROJ', 'now', 'now');
		INSERT INTO tasks (id, project_id, sequence_number, title, created_at, updated_at) VALUES
			('t1', 'p1', 1, 'One', 'now', 'now'), ('t2', 'p1', 2, 'Two', 'now', 'now');
		INSERT INTO runs (id, task_id, task_ref, agent, machine, workspace_path, state, started_at) VALUES
			('r-c', 't1', 'PROJ-1', 'claude', 'm', '/w', 'ended', '2026-11-12T10:00:00.000000Z'),
			('r-a', 't1', 'PROJ-1', 'claude', 'm', '/w', 'ended', '2026-11-12T11:00:00.000000Z'),
			('r-b', 't1', 'PROJ-1', 'claude', 'm', '/w', 'ended', '2026-11-12T11:00:00.000000Z'),
			('r-d', 't2', 'PROJ-2', 'claude', 'm', '/w', 'failed', '2026-11-12T09:00:00.000000Z')`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s := openTest(t, dir)
	want := map[string]int{"r-c": 1, "r-a": 2, "r-b": 3, "r-d": 1}
	for id, n := range want {
		var got int
		if err := s.db.QueryRow("SELECT number FROM runs WHERE id = ?", id).Scan(&got); err != nil || got != n {
			t.Errorf("run %s number = %d, %v; want %d", id, got, err, n)
		}
	}
	r, err := s.CreateRun(context.Background(), core.Run{TaskID: "t1", TaskRef: "PROJ-1", Agent: "claude", Machine: "m", WorkspacePath: "/w"})
	if err != nil || r.Number != 4 {
		t.Errorf("next run = %+v, %v; want number 4", r, err)
	}
}
