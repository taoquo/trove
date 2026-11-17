package media

// The cache bounds and the cache's privacy.
//
// Both caches used to be bounded by something other than what they cost: the memory
// cache by entry count, and the disk cache by a running total that grew every time a
// file was rewritten. These tests pin the numbers the budgets promise.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestTheInMemoryCacheStaysInsideItsByteBudget covers the memory bound: the count
// alone cannot stop 64 photographs from filling the process.
func TestTheInMemoryCacheStaysInsideItsByteBudget(t *testing.T) {
	fetcher := NewFetcher()
	// A 512x512 picture costs 1 MiB by the estimate, so three fit and the fourth must
	// push the first out.
	fetcher.cacheBudget = 3 << 20
	picture := &Image{Width: 512, Height: 512}

	for i := 0; i < 4; i++ {
		fetcher.store(fmt.Sprintf("https://x/%d.png", i), picture)
	}

	if got := fetcher.CachedBytes(); got > fetcher.cacheBudget {
		t.Errorf("the cache holds %d bytes, over its %d budget", got, fetcher.cacheBudget)
	}
	if got := fetcher.Size(); got != 3 {
		t.Errorf("the cache holds %d pictures, want 3", got)
	}
	if _, ok := fetcher.Cached("https://x/0.png"); ok {
		t.Error("the oldest picture was not evicted")
	}
	if _, ok := fetcher.Cached("https://x/3.png"); !ok {
		t.Error("the newest picture was evicted instead of the oldest")
	}
}

// TestRewritingOneURLIsChargedOnce keeps a refresh loop from exhausting the budget.
func TestRewritingOneURLIsChargedOnce(t *testing.T) {
	fetcher := NewFetcher()
	picture := &Image{Width: 512, Height: 512}

	for i := 0; i < 100; i++ {
		fetcher.store("https://x/one.png", picture)
	}

	if got, want := fetcher.CachedBytes(), decodedBytes(picture); got != want {
		t.Errorf("one picture rewritten a hundred times costs %d bytes, want %d", got, want)
	}
	if got := fetcher.Size(); got != 1 {
		t.Errorf("the cache holds %d pictures, want 1", got)
	}
}

// TestAPictureLargerThanTheBudgetIsNotKept is the honest edge: evicting it at once
// keeps the bound true, and the caller still has the image it asked for.
func TestAPictureLargerThanTheBudgetIsNotKept(t *testing.T) {
	fetcher := NewFetcher()
	fetcher.cacheBudget = 1 << 20

	fetcher.store("https://x/huge.png", &Image{Width: 1024, Height: 1024})

	if _, ok := fetcher.Cached("https://x/huge.png"); ok {
		t.Error("a picture larger than the whole budget must not stay cached")
	}
	if got := fetcher.CachedBytes(); got != 0 {
		t.Errorf("the cache reports %d bytes, want 0", got)
	}
}

// TestTheDiskCacheDoesNotDriftWhenAFileIsRewritten covers the disk total: each Put
// used to add the new body without crediting the file it replaced.
func TestTheDiskCacheDoesNotDriftWhenAFileIsRewritten(t *testing.T) {
	cache := NewDiskCache(t.TempDir(), DefaultDiskBudget)
	if cache == nil {
		t.Fatal("the cache must open in a temporary directory")
	}
	body := bytes.Repeat([]byte{0x42}, 4096)

	for i := 0; i < 100; i++ {
		cache.Put("https://x/one.png", body)
	}

	cache.mu.Lock()
	used := cache.used
	cache.mu.Unlock()
	if used != int64(len(body)) {
		t.Errorf("the cache believes it holds %d bytes, want %d", used, len(body))
	}

	// And the directory agrees: one file, one body.
	if measured, err := cache.Size(); err != nil || measured != int64(len(body)) {
		t.Errorf("the directory holds %d bytes (err %v), want %d", measured, err, len(body))
	}
}

// TestTheDiskCacheKeepsItsDirectoryAndFilesPrivate matches the rest of the data
// directory: a picture from the library is not for every account on the machine.
func TestTheDiskCacheKeepsItsDirectoryAndFilesPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "media")
	// An earlier build created the directory world-readable; opening a cache over it
	// must narrow it rather than inherit the mode.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("prepare the directory: %v", err)
	}

	cache := NewDiskCache(dir, DefaultDiskBudget)
	if cache == nil {
		t.Fatal("the cache must open")
	}
	cache.Put("https://x/a.png", pngBytes(t, 2, 2))

	directory, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat directory: %v", err)
	}
	if mode := directory.Mode().Perm(); mode != 0o700 {
		t.Errorf("the cache directory is %#o, want 0700", mode)
	}

	file, err := os.Stat(cache.filePath("https://x/a.png"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if mode := file.Mode().Perm(); mode != 0o600 {
		t.Errorf("the cache file is %#o, want 0600", mode)
	}
}
