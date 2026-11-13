//go:build darwin

package ui

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/egoist/mygo"
)

// The native video player.
//
// AVKit's AVPlayerView is the player macOS itself shows: transport controls, scrubbing,
// volume, picture-in-picture and full screen, all of it maintained by the system. The
// toolchain forbids cgo (GOTOOLCHAIN and CGO_ENABLED=0 are pinned), so it is reached the way
// MyGo reaches AppKit: purego registers typed functions for objc_msgSend, and the classes
// come from frameworks loaded with Dlopen.
//
// The view is a subview of the window's content view rather than an element in the layout.
// MyGo paints its own interface into that view; an AppKit subview sits above it, which is
// exactly where a player belongs -- over the poster it replaces, with the lightbox's own
// controls left uncovered around it.

// The CoreGraphics structs, laid out as Objective-C lays them out: four doubles in a row.
type nsPoint struct{ X, Y float64 }
type nsSize struct{ Width, Height float64 }
type nsRect struct {
	Origin nsPoint
	Size   nsSize
}

// The player's box: the same proportions the lightbox's picture card takes, so the video
// opens where the poster was.
const (
	videoBoxWidthPct  = 0.80
	videoBoxMaxWidth  = 1100
	videoBoxHeightPct = 0.76
	videoBoxMaxHeight = 800
	// AVPlayerViewControlsStyleInline. The floating style follows the pointer and covers the
	// frame; inline sits at the foot of the picture, as QuickTime's does.
	videoControlsStyleInline = 1
	// NSViewMinXMargin | NSViewMaxXMargin | NSViewMinYMargin | NSViewMaxYMargin: AppKit keeps
	// the view centred when the window resizes, without this app placing it again.
	videoFlexibleMargins = 1 | 4 | 8 | 32
)

// Registered once. objc_msgSend is variadic at the C level and has to be given a concrete
// signature per call shape, which is what these are.
var (
	videoLoadOnce sync.Once
	videoLoadErr  error

	sendAlloc       func(objc.Class, objc.SEL) objc.ID
	sendInitFrame   func(objc.ID, objc.SEL, nsRect) objc.ID
	sendSetObject   func(objc.ID, objc.SEL, objc.ID)
	sendGetObject   func(objc.ID, objc.SEL) objc.ID
	sendGetRect     func(objc.ID, objc.SEL) nsRect
	sendSetUint     func(objc.ID, objc.SEL, uint64)
	sendClassString func(objc.Class, objc.SEL, string) objc.ID
	sendClassObject func(objc.Class, objc.SEL, objc.ID) objc.ID
	sendNoArgs      func(objc.ID, objc.SEL)

	selAlloc, selInitWithFrame, selContentView, selBounds           objc.SEL
	selAddSubview, selRemoveFromSuperview, selRelease, selSetPlayer objc.SEL
	selSetControlsStyle, selSetAutoresizingMask, selPlay, selPause  objc.SEL
	selStringWithUTF8, selURLWithString, selPlayerWithURL           objc.SEL
)

// loadVideoFrameworks opens AVKit and prepares the message functions.
func loadVideoFrameworks() error {
	videoLoadOnce.Do(func() {
		libobjc, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if err != nil {
			videoLoadErr = fmt.Errorf("ui: no Objective-C runtime: %w", err)
			return
		}
		for _, path := range []string{
			"/System/Library/Frameworks/Foundation.framework/Foundation",
			"/System/Library/Frameworks/AVFoundation.framework/AVFoundation",
			"/System/Library/Frameworks/AVKit.framework/AVKit",
		} {
			if _, err := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_NOW); err != nil {
				videoLoadErr = fmt.Errorf("ui: cannot load %s: %w", path, err)
				return
			}
		}

		purego.RegisterLibFunc(&sendAlloc, libobjc, "objc_msgSend")
		purego.RegisterLibFunc(&sendInitFrame, libobjc, "objc_msgSend")
		purego.RegisterLibFunc(&sendSetObject, libobjc, "objc_msgSend")
		purego.RegisterLibFunc(&sendGetObject, libobjc, "objc_msgSend")
		purego.RegisterLibFunc(&sendGetRect, libobjc, "objc_msgSend")
		purego.RegisterLibFunc(&sendSetUint, libobjc, "objc_msgSend")
		purego.RegisterLibFunc(&sendClassString, libobjc, "objc_msgSend")
		purego.RegisterLibFunc(&sendClassObject, libobjc, "objc_msgSend")
		purego.RegisterLibFunc(&sendNoArgs, libobjc, "objc_msgSend")

		selAlloc = objc.RegisterName("alloc")
		selInitWithFrame = objc.RegisterName("initWithFrame:")
		selContentView = objc.RegisterName("contentView")
		selBounds = objc.RegisterName("bounds")
		selAddSubview = objc.RegisterName("addSubview:")
		selRemoveFromSuperview = objc.RegisterName("removeFromSuperview")
		selRelease = objc.RegisterName("release")
		selSetPlayer = objc.RegisterName("setPlayer:")
		selSetControlsStyle = objc.RegisterName("setControlsStyle:")
		selSetAutoresizingMask = objc.RegisterName("setAutoresizingMask:")
		selPlay = objc.RegisterName("play")
		selPause = objc.RegisterName("pause")
		selStringWithUTF8 = objc.RegisterName("stringWithUTF8String:")
		selURLWithString = objc.RegisterName("URLWithString:")
		selPlayerWithURL = objc.RegisterName("playerWithURL:")

		if objc.GetClass("AVPlayerView") == 0 || objc.GetClass("AVPlayer") == 0 {
			videoLoadErr = errors.New("ui: AVKit is loaded but its player classes are missing")
		}
	})
	return videoLoadErr
}

// nativeVideo is the VideoPlayer.
type nativeVideo struct {
	mu     sync.Mutex
	view   objc.ID
	player objc.ID
}

// NewNativeVideoPlayer returns the macOS player.
func NewNativeVideoPlayer() VideoPlayer { return &nativeVideo{} }

// Show plays url in a view centred over the window's content, replacing any earlier video.
func (p *nativeVideo) Show(window *mygo.Window, url string) (err error) {
	if err := loadVideoFrameworks(); err != nil {
		return err
	}
	p.Hide()

	// A panic in a raw runtime call is not something the reader can act on, and the caller
	// has a working fallback; report it as the failed open it is.
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("ui: the video player failed: %v", recovered)
		}
	}()

	handle := window.NativeHandle()
	if handle == 0 {
		return errors.New("ui: the window has no native handle")
	}
	content := sendGetObject(objc.ID(handle), selContentView)
	if content == 0 {
		return errors.New("ui: the window has no content view")
	}

	// [NSURL URLWithString:] takes an NSString, not a C string: the Go string is wrapped by
	// +stringWithUTF8String: first.
	text := sendClassString(objc.GetClass("NSString"), selStringWithUTF8, url)
	player := sendClassObject(objc.GetClass("AVPlayer"), selPlayerWithURL,
		sendClassObject(objc.GetClass("NSURL"), selURLWithString, text))
	if player == 0 {
		return errors.New("ui: the video URL could not be opened")
	}

	view := sendInitFrame(sendAlloc(objc.GetClass("AVPlayerView"), selAlloc),
		selInitWithFrame, videoFrame(sendGetRect(content, selBounds)))
	if view == 0 {
		return errors.New("ui: the player view could not be created")
	}
	sendSetObject(view, selSetPlayer, player)
	sendSetUint(view, selSetControlsStyle, videoControlsStyleInline)
	sendSetUint(view, selSetAutoresizingMask, videoFlexibleMargins)
	sendSetObject(content, selAddSubview, view)
	sendNoArgs(player, selPlay)

	p.mu.Lock()
	p.view, p.player = view, player
	p.mu.Unlock()
	return nil
}

// Hide stops the video and takes its view out of the window.
func (p *nativeVideo) Hide() {
	p.mu.Lock()
	view, player := p.view, p.player
	p.view, p.player = 0, 0
	p.mu.Unlock()

	if view == 0 {
		return
	}
	defer func() { _ = recover() }()
	if player != 0 {
		sendNoArgs(player, selPause)
	}
	// The player is released with the view's reference to it; the view is ours from alloc,
	// and the window's reference goes with the removal.
	sendSetObject(view, selSetPlayer, 0)
	sendNoArgs(view, selRemoveFromSuperview)
	sendNoArgs(view, selRelease)
}

// videoFrame centres the player's box in the content view.
func videoFrame(bounds nsRect) nsRect {
	width := bounds.Size.Width * videoBoxWidthPct
	if width > videoBoxMaxWidth {
		width = videoBoxMaxWidth
	}
	height := bounds.Size.Height * videoBoxHeightPct
	if height > videoBoxMaxHeight {
		height = videoBoxMaxHeight
	}
	return nsRect{
		Origin: nsPoint{X: (bounds.Size.Width - width) / 2, Y: (bounds.Size.Height - height) / 2},
		Size:   nsSize{Width: width, Height: height},
	}
}
