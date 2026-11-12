package ui

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/taoquo/trove/apps/macos/internal/domain"
	"github.com/taoquo/trove/apps/macos/internal/media"
)

// Drawing remote media.
//
// The interface asks for a picture while it draws, and a fetch is a network round trip.
// So the draw path only ever reads a cache: a miss starts a background load and asks
// for one more frame, and the picture appears on the frame after it arrives. Fetching
// inside the frame would turn a redraw into a stall on the UI thread.

// MediaImageSize is the box a detail thumbnail occupies.
const MediaImageSize = 160

// DrawCacheBudget is how many bytes of drawable pictures are kept.
//
// A picture that is on screen is asked for on every frame, and the source's grid renders
// up to 80 cards at a time (BookmarkResultsPane.tsx:264, OptionsApp.tsx:240-243) -- about
// 160 pictures with their avatars. Measured on this machine, the library's 84 cached
// pictures decode to 93.6 MB of RGBA, so a full first screen is around 190 MB. The budget
// is above that: a picture that is not evicted is a picture that does not blink.
const DrawCacheBudget = 256 << 20

// drawCache holds the drawable form of each decoded picture.
//
// MyGo's Bitmap is what the painter takes, so the conversion happens once per picture
// rather than on every frame. It is also the record of "this picture is ready": a frame
// draws from here and nowhere else, so a picture that has been seen stays on screen even
// when the fetcher's much smaller cache forgets the image behind it.
//
// The cache is bounded by bytes, but the bound is soft: entries drawn in the current
// frame are never evicted. Evicting one would put a placeholder where a picture was a
// moment ago, which is exactly the flicker this cache exists to prevent.
type drawCache struct {
	mu      sync.Mutex
	entries map[string]*drawnPicture
	order   *list.List // *drawnPicture, most recently drawn at the front
	bytes   int
	frame   uint64
	// budget overrides DrawCacheBudget, so a test can reach the eviction path without
	// allocating a quarter of a gigabyte.
	budget int
}

type drawnPicture struct {
	url    string
	bitmap *ui.Bitmap
	bytes  int
	used   uint64
	elem   *list.Element
}

// beginFrame marks a new build pass. Pictures drawn from here on are in use.
func (d *drawCache) beginFrame() {
	d.mu.Lock()
	d.frame++
	d.mu.Unlock()
}

// get returns a picture that is ready to draw, and marks it as in use.
func (d *drawCache) get(url string) (*ui.Bitmap, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	entry, ok := d.entries[url]
	if !ok {
		return nil, false
	}
	d.touch(entry)
	return entry.bitmap, true
}

// put stores a picture, converting it the first time.
//
// It returns the picture to draw: an entry already present wins, because converting twice
// would allocate twice for one URL.
func (d *drawCache) put(url string, loaded *media.Image) *ui.Bitmap {
	d.mu.Lock()
	defer d.mu.Unlock()

	if entry, ok := d.entries[url]; ok {
		d.touch(entry)
		return entry.bitmap
	}
	if d.order == nil {
		d.order = list.New()
		d.entries = map[string]*drawnPicture{}
	}

	bitmap := ui.NewBitmap(loaded.Pixels)
	entry := &drawnPicture{
		url:    url,
		bitmap: bitmap,
		// MyGo copies the pixels into an RGBA buffer, so this is the memory the bitmap
		// really holds -- not the smaller decoded form it was made from.
		bytes: 4 * loaded.Width * loaded.Height,
		// A picture that has just arrived belongs to the frame that asked for it.
		used: d.frame,
	}
	entry.elem = d.order.PushFront(entry)
	d.entries[url] = entry
	d.bytes += entry.bytes
	d.evictOverBudget()
	return bitmap
}

// len reports how many pictures are held, for tests and diagnostics.
func (d *drawCache) len() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.entries)
}

func (d *drawCache) touch(entry *drawnPicture) {
	if entry.used != d.frame {
		entry.used = d.frame
		d.order.MoveToFront(entry.elem)
	}
}

// evictOverBudget drops the least recently drawn pictures until the cache fits.
//
// A picture drawn in the current frame is skipped: it is on screen, and replacing it with
// a placeholder for a frame is the flicker the budget exists to control, not to cause. If
// every entry is in use the cache goes over budget instead, which is the honest trade --
// the screen is showing that much.
func (d *drawCache) evictOverBudget() {
	budget := d.budget
	if budget <= 0 {
		budget = DrawCacheBudget
	}
	for d.bytes > budget {
		victim := d.oldestNotInUse()
		if victim == nil {
			return
		}
		d.order.Remove(victim.elem)
		delete(d.entries, victim.url)
		d.bytes -= victim.bytes
	}
}

func (d *drawCache) oldestNotInUse() *drawnPicture {
	for elem := d.order.Back(); elem != nil; elem = elem.Prev() {
		entry := elem.Value.(*drawnPicture)
		if entry.used < d.frame {
			return entry
		}
	}
	return nil
}

// UseMediaFetcher installs the loader. Without one, media is described rather than shown.
func (s *Shell) UseMediaFetcher(fetcher *media.Fetcher) { s.media = fetcher }

// mediaPicture resolves a picture for a URL, starting a load if needed.
//
// The bool says whether a picture is ready to draw. A false is not a failure: it means
// the load is in flight or it did not succeed, and the caller falls back to describing
// the media instead of drawing it.
func (s *Shell) mediaPicture(url string) (*ui.Bitmap, bool) {
	if s.media == nil || url == "" {
		return nil, false
	}

	// Already decoded: draw it. This is the first question because a picture that has been
	// seen must keep being drawn. Asking the fetcher first made the screen blink: its cache
	// is a fraction of the size of a rendered grid, so a picture drawn on one frame was
	// evicted by the next one's loads and reappeared as a placeholder.
	if bitmap, ok := s.draw.get(url); ok {
		return bitmap, true
	}

	// Not converted yet but the fetcher still has it: no network round trip needed.
	if loaded, ok := s.media.Cached(url); ok {
		return s.draw.put(url, loaded), true
	}

	// Not cached: start a load, once, and ask for another frame.
	if !s.media.Loading(url) {
		s.startMediaLoad(url)
	}
	return nil, false
}

// mediaRetryDelays is how long to wait before asking again, by attempt number.
//
// A picture that failed is usually a dropped connection rather than a bad URL: measured
// on this machine over a proxied link, 3 of 57 avatar requests timed out and every one of
// them succeeded on a second try. Without a retry the placeholder simply stayed: nothing
// schedules another frame after a failed load, so the picture only reappeared if the user
// happened to scroll or click. Three attempts is enough for a transient failure and stops
// a dead link from being requested forever.
var mediaRetryDelays = []time.Duration{2 * time.Second, 8 * time.Second, 20 * time.Second}

// mediaAttempts counts failed loads per URL, so one dead picture does not retry forever.
// It is package-level because the counter belongs to the URL, not to a shell.
var mediaAttempts sync.Map // url -> int

// mediaRetryDelay is how long to wait after a failed attempt, and whether to wait at all.
func mediaRetryDelay(attempt int) (time.Duration, bool) {
	if attempt < 1 || attempt > len(mediaRetryDelays) {
		return 0, false
	}
	return mediaRetryDelays[attempt-1], true
}

// mediaLoadTimeout bounds one picture's whole trip, including its wait for a request
// slot. The fetcher sends six at a time, so a cold first screen -- 92 pictures in this
// library -- queues; the request itself is still bounded by media.Timeout once it starts.
const mediaLoadTimeout = 3 * time.Minute

// startMediaLoad fetches in the background and requests a repaint when it lands.
func (s *Shell) startMediaLoad(url string) {
	fetcher := s.media
	window := s.window

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), mediaLoadTimeout)
		defer cancel()

		loaded, err := fetcher.Load(ctx, url)
		if err != nil {
			// A failure is not shown: there is nothing useful to say about a picture that
			// did not arrive. It is retried, though -- a dropped connection must not leave
			// a blank frame until the next unrelated redraw.
			attempt := 0
			if previous, ok := mediaAttempts.Load(url); ok {
				attempt = previous.(int)
			}
			attempt++
			mediaAttempts.Store(url, attempt)
			if delay, again := mediaRetryDelay(attempt); again && window != nil {
				time.AfterFunc(delay, window.Invalidate)
			}
			return
		}

		// Convert before asking for the frame that draws it. The fetcher's cache is small
		// and may drop this image as soon as the next one lands, so the drawable form has
		// to exist on its own.
		s.draw.put(url, loaded)
		mediaAttempts.Delete(url)
		if window != nil {
			window.Invalidate()
		}
	}()
}

// mediaLightboxImage draws the lightbox's picture.
//
// The item's own picture, with a video's poster standing in for the file
// (BookmarkInspector.tsx:60), contained rather than cropped: the stylesheet's
// .options-media-lightbox-image is object-fit: contain, so the whole picture shows. It
// carries its own entrance -- a 160ms fade and a 0.985 scale -- separate from the card's
// pop, which is why the two progress values are passed in rather than computed here.
func (s *Shell) mediaLightboxImage(c *ui.Context, item domain.Media, height, entrance float32, theme *ui.Theme) {
	url := domain.MediaThumbnailURL([]domain.Media{item})
	var image ui.Element
	if bitmap, ready := s.mediaPicture(url); ready {
		image = ui.Image(c, bitmap).FillWidth().Height(height).Fit(ui.Contain).Radius(LightboxImageRadius)
	} else {
		// The box holds the picture's place, so the card does not resize when it lands.
		image = ui.Box(c).FillWidth().Height(height).Radius(LightboxImageRadius).Background(theme.Background)
	}
	image.
		Opacity(EntranceLightboxImage.OpacityAt(entrance)).
		Scale(EntranceLightboxImage.ScaleAt(entrance), EntranceLightboxImage.ScaleAt(entrance))
}
