// Package store persists Taskship data in a local SQLite database.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DBFile is the database file name inside the Taskship home directory.
const DBFile = "taskship.db"

// Migrations are applied in file-name order; the schema version stored in
// PRAGMA user_version is the number of migrations applied. Never edit or
// reorder a released migration; add a new file instead.
//
//go:embed migrations/*.sql
var migrations embed.FS

// Store is a Taskship SQLite database.
type Store struct {
	db *sql.DB
}

// Open creates dir and the database if needed and applies pending migrations.
func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	// _txlock=immediate takes the write lock at BEGIN, so concurrent
	// taskship processes wait (busy_timeout) instead of failing mid-transaction.
	dsn := filepath.Join(dir, DBFile) +
		"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", filepath.Join(dir, DBFile), err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(ctx context.Context, db *sql.DB) error {
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(files) {
		return fmt.Errorf("database schema version %d is newer than this taskship supports (%d); upgrade taskship", version, len(files))
	}
	if version == len(files) {
		return nil
	}
	for _, f := range files[version:] {
		sqlText, err := migrations.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(sqlText)); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", len(files))); err != nil {
		return err
	}
	return tx.Commit()
}
