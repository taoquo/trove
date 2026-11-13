// Package sqlite owns the database: opening it, its pragmas, its schema
// migrations and its integrity checks. Nothing outside this package executes
// SQL.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// DB is the application database handle.
type DB struct {
	*sql.DB
	path string
}

// Open opens (creating if needed) the database at path and brings its schema up
// to date.
//
// The caller passes the per-user data directory from mygo.App.Path(mygo.PathUserData).
func Open(path string) (*DB, error) {
	if path == "" {
		return nil, errors.New("sqlite: empty database path")
	}

	if err := EnsurePrivateDir(filepath.Dir(path)); err != nil {
		return nil, err
	}

	// The pragmas are set through the DSN so that every connection the pool
	// opens carries them. foreign_keys in particular is per-connection and
	// defaults to OFF, which would silently disable every ON DELETE CASCADE in
	// the schema.
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	handle, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}

	// SQLite has a single writer. Serialising through one connection makes
	// writes deterministic and lets the read-modify-write paths (settings,
	// credentials, run state) rely on the database rather than on an
	// application-level queue to keep two concurrent writers from losing each
	// other's update.
	handle.SetMaxOpenConns(1)

	db := &DB{DB: handle, path: path}

	if err := db.applyPragmas(); err != nil {
		handle.Close()
		return nil, err
	}
	if err := db.verifyForeignKeys(); err != nil {
		handle.Close()
		return nil, err
	}
	if err := db.restrictFilePermissions(); err != nil {
		handle.Close()
		return nil, err
	}
	if err := db.Migrate(); err != nil {
		handle.Close()
		return nil, err
	}

	return db, nil
}

// Path returns the database file path.
func (db *DB) Path() string { return db.path }

// Revision reports a number that changes whenever this connection writes a row.
//
// total_changes() is SQLite's own count of the rows this connection has inserted, updated
// or deleted. The app serialises every write through one connection (SetMaxOpenConns(1)),
// so the count is a complete revision for the library: a reader that saw revision N can
// reuse a cache while the database still answers N. Reads do not move it.
func (db *DB) Revision(ctx context.Context) (int64, error) {
	var revision int64
	if err := db.QueryRowContext(ctx, "SELECT total_changes()").Scan(&revision); err != nil {
		return 0, fmt.Errorf("sqlite: read revision: %w", err)
	}
	return revision, nil
}

func (db *DB) applyPragmas() error {
	// WAL keeps readers from blocking the writer.
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return fmt.Errorf("sqlite: set journal_mode: %w", err)
	}
	// NORMAL is the usual WAL trade-off: durable across application crashes,
	// and only at risk from an OS-level crash.
	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		return fmt.Errorf("sqlite: set synchronous: %w", err)
	}
	return nil
}

// verifyForeignKeys asserts that foreign key enforcement is actually on. The
// pragma defaults to off, and leaving it off makes every ON DELETE CASCADE in
// the schema a no-op that fails silently.
func (db *DB) verifyForeignKeys() error {
	var enabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		return fmt.Errorf("sqlite: read foreign_keys: %w", err)
	}
	if enabled != 1 {
		return errors.New("sqlite: foreign key enforcement is off; refusing to continue because every cascade would be ignored")
	}
	return nil
}

// EnsurePrivateDir creates dir if needed and forces it to 0700.
//
// The chmod is not redundant: MyGo creates the per-user data directory with
// 0755, which would leave the library readable by every account on the
// machine.
func EnsurePrivateDir(dir string) error {
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("sqlite: create data directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("sqlite: restrict data directory: %w", err)
	}
	return nil
}

// restrictFilePermissions forces the database and its WAL companions to 0600.
func (db *DB) restrictFilePermissions() error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		path := db.path + suffix
		if _, err := os.Stat(path); err != nil {
			continue // not created yet
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("sqlite: restrict %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

// Check runs the integrity checks that must pass before the app trusts the file,
// and after any restore or import.
func (db *DB) Check() error {
	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("sqlite: integrity_check: %w", err)
	}
	if !strings.EqualFold(integrity, "ok") {
		return fmt.Errorf("sqlite: integrity_check reported %q", integrity)
	}

	var violations int
	if err := db.QueryRow("SELECT count(*) FROM pragma_foreign_key_check").Scan(&violations); err != nil {
		return fmt.Errorf("sqlite: foreign_key_check: %w", err)
	}
	if violations != 0 {
		return fmt.Errorf("sqlite: foreign_key_check found %d violations", violations)
	}

	return nil
}

// Close checkpoints the WAL and closes the handle.
//
// The checkpoint matters for backups: copying only the .db file while the WAL
// still holds committed pages produces a database that is missing recent writes.
func (db *DB) Close() error {
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		// A failed checkpoint must not stop the close.
		_ = err
	}
	return db.DB.Close()
}
