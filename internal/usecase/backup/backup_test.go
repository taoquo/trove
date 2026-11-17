package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }
func (c *fixedClock) advance(days int) {
	c.now = c.now.AddDate(0, 0, days)
}

// fakeSnapshot stands in for the database: it records how many copies were asked for
// and leaves the file VACUUM INTO would have left.
type fakeSnapshot struct {
	calls int
	body  string
	err   error
}

func (f *fakeSnapshot) SnapshotInto(ctx context.Context, path string) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(path, []byte(f.body), 0o600)
}

func newRunner(t *testing.T, snapshot *fakeSnapshot, clock *fixedClock) *Runner {
	t.Helper()
	return New(snapshot, filepath.Join(t.TempDir(), DirName), clock)
}

// TestOneDayGetsOneCopyHoweverOftenTheAppStarts pins the rule that makes the feature
// cheap: the copy is named by the day, so a second launch on the same day is a stat
// rather than a second vacuum.
func TestOneDayGetsOneCopyHoweverOftenTheAppStarts(t *testing.T) {
	clock := &fixedClock{now: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)}
	snapshot := &fakeSnapshot{body: "a library"}
	runner := newRunner(t, snapshot, clock)
	ctx := context.Background()

	first, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if first == "" {
		t.Fatal("the first run wrote no copy")
	}
	if snapshot.calls != 1 {
		t.Errorf("the first run asked for %d copies, want 1", snapshot.calls)
	}

	// Another launch, later the same day.
	clock.now = clock.now.Add(6 * time.Hour)
	second, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second != "" {
		t.Errorf("the second run wrote %q, want no copy: today already has one", second)
	}
	if snapshot.calls != 1 {
		t.Errorf("the second run asked for another copy (%d total)", snapshot.calls)
	}

	if _, err := os.Stat(first); err != nil {
		t.Errorf("the copy is not on disk: %v", err)
	}
}

// TestTheFolderKeepsTheNewestSeven runs nine days and checks what is left: the week the
// rule promises, and nothing of the reader's own in the folder taken with it.
func TestTheFolderKeepsTheNewestSeven(t *testing.T) {
	clock := &fixedClock{now: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)}
	snapshot := &fakeSnapshot{body: "a library"}
	runner := newRunner(t, snapshot, clock)
	ctx := context.Background()
	if err := os.MkdirAll(runner.Directory(), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// A reader's own file in the folder is not a copy and must survive the pruning.
	notes := filepath.Join(runner.Directory(), "notes.txt")
	if err := os.WriteFile(notes, []byte("mine"), 0o600); err != nil {
		t.Fatalf("write the reader's file: %v", err)
	}

	for day := 0; day < 9; day++ {
		if _, err := runner.Run(ctx); err != nil {
			t.Fatalf("day %d: Run: %v", day, err)
		}
		clock.advance(1)
	}

	copies, err := runner.Copies()
	if err != nil {
		t.Fatalf("Copies: %v", err)
	}
	if len(copies) != Keep {
		t.Fatalf("%d copies kept, want %d: %v", len(copies), Keep, copies)
	}
	if got, want := filepath.Base(copies[0]), "trove-20261009.db"; got != want {
		t.Errorf("newest copy is %q, want %q", got, want)
	}
	if got, want := filepath.Base(copies[len(copies)-1]), "trove-20261003.db"; got != want {
		t.Errorf("oldest kept copy is %q, want %q", got, want)
	}
	for _, gone := range []string{"trove-20261001.db", "trove-20261002.db"} {
		if _, err := os.Stat(filepath.Join(runner.Directory(), gone)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s was not pruned", gone)
		}
	}
	if _, err := os.Stat(notes); err != nil {
		t.Errorf("pruning took the reader's file: %v", err)
	}
}

// TestAFailedCopyIsReportedAndLeavesNothingBehind covers the read-only folder a backup
// cannot write to: the failure has to reach the caller, because that is what the status
// line shows.
func TestAFailedCopyIsReportedAndLeavesNothingBehind(t *testing.T) {
	clock := &fixedClock{now: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)}
	snapshot := &fakeSnapshot{err: errors.New("read-only file system")}
	runner := newRunner(t, snapshot, clock)

	if _, err := runner.Run(context.Background()); err == nil {
		t.Fatal("a failed copy was reported as success")
	} else if !errors.Is(err, snapshot.err) {
		t.Errorf("the failure lost its cause: %v", err)
	}
	if _, err := os.Stat(runner.PathFor(clock.now)); !errors.Is(err, os.ErrNotExist) {
		t.Error("the failed run left a file that would pass for today's copy")
	}
}

// TestTheFolderIsPrivate keeps the copies as private as the library they came from: a
// copy of the whole library is not something to leave world-readable.
func TestTheFolderIsPrivate(t *testing.T) {
	clock := &fixedClock{now: time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)}
	runner := newRunner(t, &fakeSnapshot{body: "a library"}, clock)
	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	info, err := os.Stat(runner.Directory())
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Errorf("the backup folder is %o, want 700", mode)
	}
}

// TestTheCopyIsNamedByTheUTCDay pins the name: the day is the date the copy was taken on
// this machine, and two launches either side of a time-zone change cannot share a name
// by accident.
func TestTheCopyIsNamedByTheUTCDay(t *testing.T) {
	runner := New(nil, "/data/backups", nil)
	// 23:30 in UTC+8 is still the 12th in UTC.
	local := time.Date(2026, 10, 13, 7, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	if got, want := filepath.Base(runner.PathFor(local)), "trove-20261012.db"; got != want {
		t.Errorf("PathFor = %q, want %q", got, want)
	}
}
