package sqlite

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// SnapshotInto writes a consistent copy of the library to path.
//
// VACUUM INTO writes a checkpointed database without the WAL, so the copy opens on its
// own rather than depending on the original's side files. SQLite refuses a path that
// already exists, which is what lets the caller treat "today's copy is there" as a
// finished job instead of a race. The copy is private like the library it came from.
func (db *DB) SnapshotInto(ctx context.Context, path string) error {
	// VACUUM INTO takes a path literal, not a bound parameter, so the one value that
	// reaches the statement is escaped here. This is the only string this package
	// splices into SQL.
	quoted := strings.ReplaceAll(path, "'", "''")
	if _, err := db.ExecContext(ctx, "VACUUM INTO '"+quoted+"'"); err != nil {
		return fmt.Errorf("sqlite: snapshot into %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("sqlite: restrict the snapshot at %s: %w", path, err)
	}
	return nil
}
