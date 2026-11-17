// Package backup keeps a daily copy of the library.
//
// The copies are the answer to the one failure the app cannot undo: a migration that
// had to be reverted by hand, or a database file that was damaged while the window was
// open. They are written with VACUUM INTO, so a copy is a checkpointed database that
// opens on its own, and they are named by the day they were taken.
package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/taoquo/trove/internal/ports"
)

const (
	// DirName is the folder under the data directory that holds the copies.
	DirName = "backups"
	// Keep is how many daily copies stay on disk. A week is enough to notice a
	// problem and still find a copy from before it; more would quietly grow into a
	// second library on the disk.
	Keep = 7

	dayFormat  = "20060102"
	filePrefix = "trove-"
	fileSuffix = ".db"
)

// Snapshotter is the database a copy is taken from.
type Snapshotter interface {
	// SnapshotInto writes a consistent copy of the database to path.
	SnapshotInto(ctx context.Context, path string) error
}

// Runner writes one copy per day and keeps the newest seven.
type Runner struct {
	snapshot Snapshotter
	dir      string
	clock    ports.Clock
}

// New builds a runner over a database and the folder its copies live in.
func New(snapshot Snapshotter, dir string, clock ports.Clock) *Runner {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Runner{snapshot: snapshot, dir: dir, clock: clock}
}

// Directory is where the copies live.
func (r *Runner) Directory() string { return r.dir }

// PathFor names the copy for one day.
//
// The date is UTC, so a copy taken at 23:00 in one time zone and one taken at 01:00 in
// the next are named by the day they were taken on this machine, not by a local date
// that could repeat or skip when the clock changes. The name is the whole record: there
// is no index to drift from the files.
func (r *Runner) PathFor(now time.Time) string {
	return filepath.Join(r.dir, filePrefix+now.UTC().Format(dayFormat)+fileSuffix)
}

// Run writes today's copy unless it is already there, then prunes old ones.
//
// The returned path is the copy written, or "" when today already had one. A second
// launch on the same day therefore costs one stat, and the naming is what makes the rule
// "one per day" true however many times the process starts.
func (r *Runner) Run(ctx context.Context) (string, error) {
	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return "", fmt.Errorf("backup: create the backup directory: %w", err)
	}

	target := r.PathFor(r.clock.Now())
	if _, err := os.Stat(target); err == nil {
		return "", nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("backup: look for today's copy: %w", err)
	}

	if err := r.snapshot.SnapshotInto(ctx, target); err != nil {
		return "", fmt.Errorf("backup: write today's copy: %w", err)
	}

	// Pruning is not part of the success: the copy just written is valid, and failing
	// the whole run because an older file would not go away would report the backup as
	// lost when it is not.
	_ = r.prune(Keep)
	return target, nil
}

// Copies lists the copies on disk, newest first.
//
// Only files this package names are listed, so anything else a reader keeps in the
// folder is left alone.
func (r *Runner) Copies() ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(r.dir, filePrefix+"*"+fileSuffix))
	if err != nil {
		return nil, fmt.Errorf("backup: list the copies: %w", err)
	}
	// The names carry the day, so byte order is date order.
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	return matches, nil
}

// prune keeps the newest keep copies and removes the rest.
func (r *Runner) prune(keep int) error {
	copies, err := r.Copies()
	if err != nil {
		return err
	}
	if len(copies) <= keep {
		return nil
	}
	for _, path := range copies[keep:] {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
