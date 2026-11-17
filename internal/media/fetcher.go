// Package media loads the pictures a bookmark points at.
//
// A native app has to fetch, cache, decode and handle failure itself; there is no
// browser to write <img src="…"> and let it do all four. Without this the media
// section can show with nothing in it.
//
// The limits below exist because the URLs come from the network and the files are
// decoded into memory: without them a single bookmark could pull an arbitrary amount of
// data into the process.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	// Decoders are registered for the formats a bookmark can carry.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/taoquo/trove/internal/netproxy"
)

// The fetcher's limits.
const (
	// MaxBytes is the largest image that will be read; an avatar-sized payload is far
	// below this.
	MaxBytes = 12 << 20 // 12 MiB
	// Timeout bounds one request, including the body read.
	//
	// Measured on this machine over a proxied link: a 2 KB avatar answered in 4.8s and a
	// 107 KB photo in 9.3s, with individual requests reaching 40s. Twenty seconds cut the
	// slow half of a first screen off, which is the "some pictures never arrive" report.
	// The retry below covers what this still misses.
	Timeout = 45 * time.Second
	// MaxEntries bounds the cache by count. A bookmark list can be thousands of rows
	// and each entry holds a decoded image.
	MaxEntries = 64
	// MaxCacheBytes bounds the same cache by what its pictures cost once decoded.
	//
	// The count alone measured the wrong thing: 64 avatars and 64 photographs of the
	// same count differ by two orders of magnitude in memory, and the second is what
	// makes the app grow. The estimate is four bytes per pixel (see decodedBytes).
	MaxCacheBytes = 64 << 20 // 64 MiB
	// MaxConcurrent bounds the requests in flight.
	//
	// A rendered grid asks for every picture on screen at once -- 92 distinct URLs in this
	// library, about 160 frames. A proxy answers a burst that size by queueing: measured on
	// this machine, 3 of those 92 never arrived inside a thirty-second attempt, and each of
	// the three answered in 3 to 10 seconds when asked on its own. Six is what a browser
	// allows per host, which is the behaviour being replicated.
	MaxConcurrent = 6
)

// ErrTooLarge is returned when a response is bigger than MaxBytes.
var ErrTooLarge = errors.New("media: the file is larger than the limit")

// Image is a decoded picture, ready to draw.
type Image struct {
	Pixels image.Image
	Width  int
	Height int
}

// Fetcher loads and caches images.
//
// It is safe for concurrent use: the interface asks for several pictures at once, and
// the cache is what keeps a redraw from re-fetching.
type Fetcher struct {
	// disk is the persistent cache, when one is configured.
	disk *DiskCache

	client  *http.Client
	mu      sync.Mutex
	cache   map[string]*Image
	order   []string
	loading map[string]bool
	// cacheBytes is the estimate of what cache holds, and cacheBudget is the bound
	// on it. The pair replaces measuring by count alone.
	cacheBytes  int64
	cacheBudget int64
	// slots holds MaxConcurrent tokens, one per request on the wire.
	slots chan struct{}
}

// NewFetcher builds a fetcher with the default limits.
//
// The client carries the shared transport so pictures go through the same proxy
// the machine is configured with: a sync that succeeds while every cover image
// times out is half a fix, and the media host is as unreachable as x.com on a
// proxied network.
func NewFetcher() *Fetcher {
	return NewFetcherWithClient(netproxy.NewClient(Timeout))
}

// NewFetcherWithClient builds a fetcher against a given client, for tests.
func NewFetcherWithClient(client *http.Client) *Fetcher {
	if client == nil {
		client = netproxy.NewClient(Timeout)
	}
	return &Fetcher{
		client:      client,
		cache:       map[string]*Image{},
		loading:     map[string]bool{},
		cacheBudget: MaxCacheBytes,
		slots:       make(chan struct{}, MaxConcurrent),
	}
}

// Cached returns an image that has already been loaded.
//
// The interface calls this while drawing: a miss means "not yet", never a request,
// because fetching inside a frame is how a redraw turns into a stall.
func (f *Fetcher) Cached(url string) (*Image, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	image, ok := f.cache[url]
	if ok {
		f.touch(url)
	}
	return image, ok
}

// Loading reports whether a URL is being fetched right now.
func (f *Fetcher) Loading(url string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loading[url]
}

// Size returns how many images are cached, for tests and diagnostics.
func (f *Fetcher) Size() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.cache)
}

// CachedBytes reports what the cached pictures cost, by the same estimate eviction
// uses. It never exceeds the cache's budget.
func (f *Fetcher) CachedBytes() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cacheBytes
}

// UseDiskCache gives the fetcher a persistent cache.
//
// With one, a picture fetched on an earlier run is decoded from disk instead of being
// downloaded again.
func (f *Fetcher) UseDiskCache(cache *DiskCache) { f.disk = cache }

// DiskCache returns the persistent cache, or nil.
func (f *Fetcher) DiskCache() *DiskCache { return f.disk }

// Load fetches and decodes an image, blocking until it is ready.
//
// A URL already in flight is not fetched twice: two cells showing the same picture would
// otherwise both request it.
func (f *Fetcher) Load(ctx context.Context, url string) (*Image, error) {
	if url == "" {
		return nil, errors.New("media: no url")
	}

	f.mu.Lock()
	if cached, ok := f.cache[url]; ok {
		f.touch(url)
		f.mu.Unlock()
		return cached, nil
	}
	if f.loading[url] {
		// Another caller has it in flight; report that rather than duplicating it.
		f.mu.Unlock()
		return nil, fmt.Errorf("media: %s is already loading", url)
	}
	f.loading[url] = true
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.loading, url)
		f.mu.Unlock()
	}()

	// The disk is consulted before the network: a picture from an earlier run is
	// already paid for.
	if body, ok := f.disk.Get(url); ok {
		if loaded, err := decodeImage(body); err == nil {
			f.store(url, loaded)
			return loaded, nil
		}
		// A body that no longer decodes is stale, not fatal: fall through and fetch.
	}

	body, err := f.fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	loaded, err := decodeImage(body)
	if err != nil {
		return nil, err
	}

	// Written after the decode: a body that does not decode is not worth keeping.
	f.disk.Put(url, body)
	f.store(url, loaded)
	return loaded, nil
}

// fetch performs the request and returns the body.
func (f *Fetcher) fetch(ctx context.Context, url string) ([]byte, error) {
	// One token per request on the wire. Waiting here is not a failure: the caller's
	// context covers the queue, and the client's timeout starts when the request does.
	if f.slots != nil {
		select {
		case f.slots <- struct{}{}:
			defer func() { <-f.slots }()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("media: build the request: %w", err)
	}

	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("media: fetch %s: %w", url, err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("media: %s answered %d", url, response.StatusCode)
	}

	// The declared length is checked before reading, so an oversized file is refused
	// without spending the memory.
	if response.ContentLength > MaxBytes {
		return nil, ErrTooLarge
	}

	// Reading one byte past the limit is what makes an undeclared oversize detectable
	// rather than silently truncated.
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("media: read %s: %w", url, err)
	}
	if int64(len(body)) > MaxBytes {
		return nil, ErrTooLarge
	}

	return body, nil
}

// decodeImage turns a body into a picture.
func decodeImage(body []byte) (*Image, error) {
	pixels, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("media: decode: %w", err)
	}
	bounds := pixels.Bounds()
	return &Image{Pixels: pixels, Width: bounds.Dx(), Height: bounds.Dy()}, nil
}

// store puts an image in the cache, evicting the least recently used entries past
// either bound.
func (f *Fetcher) store(url string, image *Image) {
	size := decodedBytes(image)

	f.mu.Lock()
	defer f.mu.Unlock()

	// A replaced entry already occupies the cache: charge the new picture and credit
	// the old one, or repeatedly refreshing one URL would exhaust the budget.
	if previous, exists := f.cache[url]; exists {
		f.cacheBytes -= decodedBytes(previous)
		f.touch(url)
	} else {
		f.order = append(f.order, url)
	}
	f.cache[url] = image
	f.cacheBytes += size

	// One picture can be larger than the whole budget. Evicting it immediately is the
	// honest outcome: the bound holds, and the caller still has the image it asked for.
	for len(f.order) > 0 && (len(f.order) > MaxEntries || f.cacheBytes > f.cacheBudget) {
		oldest := f.order[0]
		f.order = f.order[1:]
		if evicted, ok := f.cache[oldest]; ok {
			delete(f.cache, oldest)
			f.cacheBytes -= decodedBytes(evicted)
		}
	}
}

// decodedBytes estimates what a picture costs once decoded: four bytes per pixel.
//
// That is what a decoded RGBA frame holds and an upper bound for the packed formats,
// which is the right direction for a budget: over-counting evicts a little early,
// under-counting would let the process grow past the number the budget promises.
func decodedBytes(image *Image) int64 {
	if image == nil {
		return 0
	}
	return int64(image.Width) * int64(image.Height) * 4
}

// touch moves an entry to the recent end of the eviction order.
//
// The cache is least-recently-used rather than first-in-first-out: a picture that is on
// screen is read on every frame, so it is the one entry that must not be the next to go.
// The list is at most MaxEntries long, so the linear scan costs nothing.
func (f *Fetcher) touch(url string) {
	for i, candidate := range f.order {
		if candidate != url {
			continue
		}
		copy(f.order[i:], f.order[i+1:])
		f.order[len(f.order)-1] = url
		return
	}
}
