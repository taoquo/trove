package media

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// The disk cache.
//
// Without it every launch re-downloads every picture the user has already seen: the
// in-memory cache holds 64 entries for the life of one run, and a library of thousands
// of bookmarks scrolls past far more than that.
//
// It stores the bytes exactly as received rather than re-encoding them, so a cached file
// is what the server sent and the decode path is the same whether the bytes came from
// the network or the disk.

// DefaultDiskBudget is how much of the user's disk the cache may use.
const DefaultDiskBudget = 256 << 20 // 256 MiB

// DiskCache stores fetched bodies under a directory, bounded by total size.
type DiskCache struct {
	dir      string
	budget   int64
	mu       sync.Mutex
	used     int64
	measured bool
}

// NewDiskCache opens a cache in a directory, creating it if needed.
//
// A directory that cannot be created or read yields a nil cache rather than an error:
// pictures are not worth refusing to start over.
func NewDiskCache(dir string, budget int64) *DiskCache {
	if dir == "" {
		return nil
	}
	if budget <= 0 {
		budget = DefaultDiskBudget
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil
	}
	// An earlier build created this directory world-readable. Narrow it rather than
	// leave a library's pictures open to every account on the machine; the files
	// inside are rewritten private as they are replaced.
	_ = os.Chmod(dir, 0o700)
	return &DiskCache{dir: dir, budget: budget}
}

// filePath is where a URL's body lives.
//
// The name is the URL's digest: a URL is not a safe file name, and hashing keeps two
// urls that differ by a query string apart.
func (c *DiskCache) filePath(url string) string {
	sum := sha256.Sum256([]byte(url))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:])+".bin")
}

// Get returns a cached body.
func (c *DiskCache) Get(url string) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	body, err := os.ReadFile(c.filePath(url))
	if err != nil {
		return nil, false
	}
	// Touch it so eviction sees it as recently used.
	_ = os.Chtimes(c.filePath(url), time.Now(), time.Now())
	return body, true
}

// Put stores a body and evicts until the cache is inside its budget.
//
// The file is written private: it holds a picture from the user's library, and the
// rest of the library's data is already readable only by its owner.
func (c *DiskCache) Put(url string, body []byte) {
	if c == nil || int64(len(body)) > c.budget {
		return
	}

	path := c.filePath(url)

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.measured {
		c.used = c.measureLocked()
		c.measured = true
	}

	// An overwrite replaces a file the total already includes. Charging the new body
	// without crediting the old one made the count drift upward: after a hundred
	// refreshes of one picture the cache believed it held a hundred copies and evicted
	// files it had never counted.
	replaced := int64(0)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		replaced = info.Size()
	}

	if err := os.WriteFile(path, body, 0o600); err != nil {
		return
	}
	c.used += int64(len(body)) - replaced
	if c.used < 0 {
		c.used = 0
	}
	c.evictLocked()
}

// evictLocked removes the least recently used files until the total fits.
//
// The total is measured from the directory the first time rather than tracked in memory,
// so a cache left by a previous run is accounted for. After that, Put keeps it exact by
// crediting the file it replaces.
func (c *DiskCache) evictLocked() {
	if !c.measured {
		c.used = c.measureLocked()
		c.measured = true
	}

	if c.used <= c.budget {
		return
	}

	type entry struct {
		path    string
		size    int64
		modTime time.Time
	}
	entries := make([]entry, 0, 64)

	files, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	for _, file := range files {
		info, err := file.Info()
		if err != nil || info.IsDir() {
			continue
		}
		entries = append(entries, entry{
			path:    filepath.Join(c.dir, file.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].modTime.Before(entries[j].modTime) })

	for _, item := range entries {
		if c.used <= c.budget {
			return
		}
		if err := os.Remove(item.path); err == nil {
			c.used -= item.size
		}
	}
}

// measureLocked adds up what is already in the directory.
func (c *DiskCache) measureLocked() int64 {
	var total int64
	files, err := os.ReadDir(c.dir)
	if err != nil {
		return 0
	}
	for _, file := range files {
		info, err := file.Info()
		if err != nil || info.IsDir() {
			continue
		}
		total += info.Size()
	}
	return total
}

// Size reports how much the cache holds, for tests and diagnostics.
func (c *DiskCache) Size() (int64, error) {
	if c == nil {
		return 0, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.measureLocked(), nil
}

// String describes the cache's directory.
func (c *DiskCache) String() string {
	if c == nil {
		return "media: no disk cache"
	}
	return fmt.Sprintf("media: disk cache in %s (budget %d bytes)", c.dir, c.budget)
}
