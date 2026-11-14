package main

import (
	"errors"
	"net/url"
	"os/exec"
	"runtime"
)

// openWebURL hands a web address to the operating system's browser.
//
// MyGo has no API for opening a URL, so the app uses the platform's opener: `open` on
// macOS, the shell-free url.dll handler on Windows, xdg-open elsewhere. Only the web is
// accepted. Every one of those openers also accepts a file path or a custom scheme, and
// the address can come from an imported backup, so an unchecked value here would let a
// crafted file ask the system to open something local.
//
// It is one function rather than one per caller because the window (opening a bookmark)
// and `trove auth` (opening the sign-in page) mean the same thing by "open this in a
// browser", and the validation is the part that must not drift between them.
func openWebURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("refusing to open a non-web URL")
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "windows":
		// rundll32 rather than "cmd /c start": the address reaches the handler without
		// passing through a shell that would read its & and %.
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}
