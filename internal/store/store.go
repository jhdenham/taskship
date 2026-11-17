// Package store persists Taskship data in a local SQLite database.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
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
	db     *sql.DB
	backup string
}

// NewerSchemaError reports a database whose schema is newer than this
// build's migrations, so a newer taskship has used it.
type NewerSchemaError struct {
	Path      string
	Version   int // the database's schema version
	Supported int // the newest schema this build knows
}

func (e *NewerSchemaError) Error() string {
	return fmt.Sprintf("%s: schema version %d is newer than the supported %d", e.Path, e.Version, e.Supported)
}

// BackupError reports a backup that failed before migrating the database
// at Path, which is left unchanged.
type BackupError struct {
	Path string
	Err  error
}

func (e *BackupError) Error() string {
	return fmt.Sprintf("back up %s: %v", e.Path, e.Err)
}

func (e *BackupError) Unwrap() error { return e.Err }

// Denied reports whether the backup failed because the Taskship home
// directory can't be written, as inside an agent's sandbox.
func (e *BackupError) Denied() bool {
	var s *sqlite.Error
	if errors.As(e.Err, &s) {
		code := s.Code() & 0xff
		return code == sqlite3.SQLITE_READONLY || code == sqlite3.SQLITE_CANTOPEN
	}
	return errors.Is(e.Err, fs.ErrPermission)
}

// BackupDir is where Open copies the database, inside the Taskship home
// directory, before migrating it.
const BackupDir = "backups"

// Open creates dir and the database if needed and applies pending
// migrations. Before migrating an existing database, it copies it into
// BackupDir; Backup reports the copy.
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
	backup, err := migrate(ctx, db, dir)
	if err != nil {
		db.Close()
		var newer *NewerSchemaError
		var backupErr *BackupError
		if errors.As(err, &newer) || errors.As(err, &backupErr) {
			return nil, err
		}
		return nil, fmt.Errorf("migrate %s: %w", filepath.Join(dir, DBFile), err)
	}
	return &Store{db: db, backup: backup}, nil
}

// Backup returns the path of the copy Open made before migrating the
// database, or "" if it didn't migrate one.
func (s *Store) Backup() string {
	return s.backup
}

// IsReadOnly reports whether err is SQLite refusing a write because the
// database can't be written, as inside an agent's sandbox.
func IsReadOnly(err error) bool {
	var e *sqlite.Error
	return errors.As(err, &e) && e.Code()&0xff == sqlite3.SQLITE_READONLY
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate applies pending migrations to the database in dir. Before
// migrating an existing database, it copies it into BackupDir and returns
// the copy's path.
func migrate(ctx context.Context, db *sql.DB, dir string) (string, error) {
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, DBFile)
	// The transaction takes the write lock at BEGIN (_txlock=immediate), so
	// other taskship processes wait here, and only the one that migrates
	// makes a backup.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	version, err := schemaVersion(ctx, tx, path, len(files))
	if err != nil || version == len(files) {
		return "", err
	}
	backup := ""
	if version > 0 {
		// VACUUM can't run in a transaction, so another connection makes
		// the copy. It can still read while this one holds the write lock,
		// and sees the database as it was, since nothing is written yet.
		backup = filepath.Join(dir, BackupDir, fmt.Sprintf("schema-%d-%s.db", version, time.Now().Format("20060102-150405")))
		if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
			return "", &BackupError{path, err}
		}
		if _, err := db.ExecContext(ctx, "VACUUM INTO ?", backup); err != nil {
			return "", &BackupError{path, err}
		}
	}
	for _, f := range files[version:] {
		sqlText, err := migrations.ReadFile(f)
		if err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx, string(sqlText)); err != nil {
			return "", fmt.Errorf("%s: %w", f, err)
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", len(files))); err != nil {
		return "", err
	}
	return backup, tx.Commit()
}

// schemaVersion returns the schema version of the database at path, which
// this build supports up to supported.
func schemaVersion(ctx context.Context, tx *sql.Tx, path string, supported int) (int, error) {
	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, err
	}
	if version > supported {
		return 0, &NewerSchemaError{path, version, supported}
	}
	return version, nil
}
