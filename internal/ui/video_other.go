//go:build !darwin

package ui

import (
	"errors"

	"github.com/egoist/mygo"
)

// The app is built for macOS; this keeps `go build ./...` honest on a machine that is not,
// and it leaves PlayVideo's fallback -- the system's own player -- as the behaviour there.

type unsupportedVideo struct{}

// NewNativeVideoPlayer returns a player that cannot play, so the shell falls back.
func NewNativeVideoPlayer() VideoPlayer { return unsupportedVideo{} }

func (unsupportedVideo) Show(*mygo.Window, string) error {
	return errors.New("ui: video playback is built for macOS only")
}

func (unsupportedVideo) Hide() {}
