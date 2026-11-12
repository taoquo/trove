package main

import (
	"bytes"
	"image"
	"image/png"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	appui "github.com/taoquo/trove/apps/macos/internal/ui"
)

// captureState is the panel TROVE_CAPTURE_STATE asks for before the picture is taken.
//
// It exists because the states worth photographing are the ones behind an interaction: the
// detail overlay, the settings panel, the tag library. The capture fires once, on the first
// frames, so without this there is no way to see them without a person at the screen.
//
// It is inert unless TROVE_CAPTURE is set, and it is read here rather than in the shell so
// the only reader of these variables stays in this file.
func captureState() string {
	if os.Getenv("TROVE_CAPTURE") == "" {
		return ""
	}
	return os.Getenv("TROVE_CAPTURE_STATE")
}

// settleAfterState is how long the redraw a capture state asked for is given.
const settleAfterState = 900 * time.Millisecond

// captureForReview writes a PNG of the window and quits, when TROVE_CAPTURE names a file.
//
// This exists because comparing the replication against the source needs pictures. The
// extension's options page can be photographed with headless Chrome; this is the other
// half of that pair, and it is the only way to see what this build actually looks like
// without a person at the screen.
//
// It is inert unless the variable is set, so the app's normal start is unchanged.
//
//	TROVE_CAPTURE=/tmp/go.png TROVE_CAPTURE_DELAY=3s ./X\ Bookmark\ Manager.app/Contents/MacOS/x-bookmark-manager
func captureForReview(window *mygo.Window, shell *appui.Shell) {
	path := os.Getenv("TROVE_CAPTURE")
	if path == "" {
		return
	}

	// The first frames are the entrance animation, and the data load finishes after them.
	// Waiting past both is what makes the picture comparable to the source's.
	delay := 3 * time.Second
	if raw := os.Getenv("TROVE_CAPTURE_DELAY"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			log.Printf("TROVE_CAPTURE_DELAY %q is not a duration; using %s", raw, delay)
		} else {
			delay = parsed
		}
	}

	go func() {
		time.Sleep(delay)
		// After the delay, not before it: a state that needs data -- the detail overlay
		// needs a selected bookmark -- has nothing to open until the first read lands, and
		// the delay is what waits for that read.
		//
		// What this cannot do is photograph an overlay. A panel's visibility comes from
		// EntranceProgress, which is root.Animate over the entrance duration, and that value
		// does not advance on this path: with the state applied the picture was byte-for-byte
		// identical to the one without it, and a two-second pause changed nothing. The window
		// is not being ticked the way a live one is, so the panel stays at its start value and
		// draws nothing.
		//
		// The overlay's contents are covered by ui.Tester instead, which reads the rendered
		// text and does not depend on an animation completing.
		// The state is applied here so that data-dependent states -- the detail needs a chosen
		// bookmark, and a chosen bookmark needs the first read -- have something to act on. It
		// is also applied before the wait, above, and that is the application that shows: on
		// this path the content is built once, at startup, and nothing redraws it afterwards.
		// Round 248 traced the order: early state applied, VIEW drawn, late state applied,
		// capture -- one VIEW, before the late application, which is therefore invisible.
		// Invalidate and Update were both tried and neither produced a second build.
		//
		// So what is photographed is the startup frame plus whatever the early application set,
		// which is why the appearance works and a panel does not.
		//
		// Invalidate asks for a redraw rather than performing one, and on this path the redraw
		// does not come: CapturePage snapshots the last frame the Content built. Round 248
		// traced the order -- early state applied, VIEW drawn, late state applied, capture -- and
		// there is no second VIEW, so everything the late application set is invisible in the
		// picture. That is why the detail could be opened and never photographed.
		//
		// Update runs the function on the main thread and then redraws the Content, which is
		// where a window's UI is built, so the frame CapturePage takes is one drawn afterwards.
		window.Update(func() { applyCaptureState(shell) })
		time.Sleep(settleAfterState)
		png, err := window.CapturePage()
		if err != nil {
			log.Printf("capture failed: %v", err)
			os.Exit(1)
		}
		if err := os.WriteFile(path, png, 0o644); err != nil {
			log.Printf("capture could not be written: %v", err)
			os.Exit(1)
		}
		log.Printf("captured %s (%d bytes)", path, len(png))
		os.Exit(0)
	}()
}

// applyCaptureState opens the panel TROVE_CAPTURE_STATE names.
//
// Called by start() after the first read has landed and before the window's content is built
// for the first time, which is the only moment at which this can take effect: on the capture
// path the content is built once, and neither Invalidate nor Update makes it build again
// (round 248 traced the order and tried both). So the state has to be in place for that one
// build, and it has to be after the read, or a state that needs data -- the detail needs a
// chosen bookmark -- has nothing to act on.
func applyCaptureState(shell *appui.Shell) bool {
	applied := false
	if state := captureState(); state != "" {
		if !shell.ApplyCaptureState(state) {
			log.Printf("TROVE_CAPTURE_STATE %q is not a known state", state)
			return false
		}
		applied = true
	}
	// The language is applied after the state and whether or not one was named.
	//
	// After, because ApplyCaptureState resets the locale to the system default as its first act --
	// it exists to keep one capture from inheriting the last one's preferences. Independent of the
	// state, because most of the eight baselines name no state at all and the Chinese layout is
	// exactly what those need to show.
	if locale := os.Getenv("TROVE_CAPTURE_LOCALE"); locale != "" {
		shell.ApplyCaptureLocale(locale)
		applied = true
	}
	// The appearance, pinned for the same reason: the reset above leaves it "system", which is
	// the machine's own theme, and the parity renditions of the extension are light.
	if appearance := os.Getenv("TROVE_CAPTURE_APPEARANCE"); appearance != "" {
		shell.ApplyCaptureAppearance(appearance)
		applied = true
	}
	// One-shot reports are written last for the same reason the language is applied last:
	// they are translated sentences, and the state path clears that class of message when
	// the language is pinned. The sync receipt also has a deadline of its own, which is why
	// the picture is drawn immediately after it rather than after a capture delay.
	switch captureState() {
	case "sync-done":
		shell.ApplyCaptureReceipt(2)
	case "settings-reset":
		shell.ApplyCaptureResetReport()
	case "settings-export":
		shell.ApplyCaptureExportPrompt()
	}
	return applied
}

// headlessCapture draws the shell in memory and writes it, without a window.
//
// The capture path used to need a real window: it photographed the last frame the content built,
// with window.CapturePage. That meant every screenshot opened a window on the machine running it --
// it took focus for a few seconds, and tools/baselines.sh opens eight of them in a row. Round 476
// moved the drawing to ui.Render, whose own documentation says it draws "without a window: for
// snapshots and tests", so the app now never appears.
//
// What it cannot photograph is the same as before: an overlay whose visibility comes from an
// animation. ui.Render settles the frames the view asks for, which is more than the window path
// did, but EntranceProgress is driven by root.Animate and a snapshot has no clock. Panels are
// covered by their own capture states and by ui.Tester in the tests.
func headlessCapture(shell *appui.Shell, path string) {
	width := 1400
	if raw := os.Getenv("TROVE_CAPTURE_WIDTH"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			width = n
		}
	}
	height := 760
	if raw := os.Getenv("TROVE_CAPTURE_HEIGHT"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			height = n
		}
	}
	// Scale 2 matches what the window path produced: a 1400-wide capture came out 2800 px.
	img := ui.Render(shell.View, width, height, 2)
	if img == nil {
		log.Printf("headless capture drew nothing")
		os.Exit(1)
	}
	if err := os.WriteFile(path, pngOf(img), 0o644); err != nil {
		log.Printf("capture could not be written: %v", err)
		os.Exit(1)
	}
	log.Printf("captured %s (%d by %d)", path, img.Rect.Dx(), img.Rect.Dy())
	os.Exit(0)
}

// pngOf encodes a rendered frame.
func pngOf(img *image.RGBA) []byte {
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.DefaultCompression}).Encode(&buf, img); err != nil {
		log.Printf("capture could not be encoded: %v", err)
		os.Exit(1)
	}
	return buf.Bytes()
}
