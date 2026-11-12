package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
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
	if v := userVersion(t, s); v != 7 {
		t.Errorf("user_version = %d, want 3", v)
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
	if v := userVersion(t, s); v != 7 {
		t.Errorf("user_version after reopen = %d, want 3", v)
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
	if v := userVersion(t, s); v != 7 {
		t.Errorf("user_version = %d, want 3", v)
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
	if err == nil || !strings.Contains(err.Error(), "upgrade taskship") {
		t.Fatalf("Open with newer schema: err = %v, want upgrade error", err)
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
