package ui

import (
	"strings"

	"github.com/egoist/mygo"
)

// Playing a post's video.
//
// The app stores the highest-bitrate mp4 X offers. The detail and the lightbox draw the
// poster, which is a still picture of a moving thing, so the app needs a player for the
// video itself. MyGo has no video element and the toolchain forbids cgo, so the player is a
// native AVKit view reached through the Objective-C runtime -- the same route MyGo itself
// takes to AppKit (purego, no cgo). It is kept behind this interface so the shell's
// behaviour -- what opens it, what closes it, what happens when it cannot open -- is
// testable without a window.

// VideoPlayer is the native surface that shows a video. Implementations drive the window
// they are given and must be called from the main thread, which is where MyGo runs event
// handlers.
type VideoPlayer interface {
	// Show plays url in its own view over the window, replacing whatever was playing.
	Show(window *mygo.Window, url string) error
	// Hide stops the video and takes its view away.
	Hide()
}

// UseVideoPlayer installs the native player.
//
// Without one the app still works: PlayVideo falls back to the system's own player, which is
// what a build for another platform gets.
func (s *Shell) UseVideoPlayer(player VideoPlayer) { s.video = player }

// PlayVideo plays a video in the window.
//
// The native player is a view over the window rather than a picture in the layout: a video
// element inside the detail's scroll region would have to be positioned frame by frame as
// the reader scrolls, and MyGo reports no scroll offset to position it with. So the video
// takes the lightbox's stage, which is already the room sized for one picture.
//
// If the player cannot open -- no AVKit, no window, a build for another platform -- the file
// goes to the system's player instead of failing silently. A video that plays somewhere is
// better than a poster that does nothing, and the fallback is a sentence the reader can see
// rather than a dead control.
func (s *Shell) PlayVideo(url string) {
	if strings.TrimSpace(url) == "" {
		return
	}
	if s.video == nil || s.window == nil {
		s.openVideoExternally(url)
		return
	}
	// The window is dropped and rebuilt on some events; a view from the previous one would
	// be a subview of a window nobody sees.
	if err := s.video.Show(s.window, url); err != nil {
		s.openVideoExternally(url)
		return
	}
	s.state.VideoURL = url
	s.invalidate()
}

// StopVideo takes the player away. It is a no-op when nothing is playing, so every close
// path can call it without asking first.
func (s *Shell) StopVideo() {
	if s.state.VideoURL == "" {
		return
	}
	s.state.VideoURL = ""
	if s.video != nil {
		s.video.Hide()
	}
	s.invalidate()
}

// VideoPlaying reports whether a video is on screen, which is what hides the poster's play
// mark behind it.
func (s *Shell) VideoPlaying() bool { return s.state.VideoURL != "" }

// openVideoExternally hands the file to whatever the system plays it with.
func (s *Shell) openVideoExternally(url string) {
	if s.openExternal == nil {
		return
	}
	if err := s.openExternal(url); err != nil {
		s.reportErr(err)
	}
}
