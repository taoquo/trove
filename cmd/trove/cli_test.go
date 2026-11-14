package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/taoquo/trove/internal/bundle"
)

// TestOnlyTheCommandsOpenNoWindow is about the dispatch: anything the app does not know,
// including the argument macOS adds when it opens the bundle, has to fall through to the
// window rather than be swallowed as a mistyped command.
func TestOnlyTheCommandsOpenNoWindow(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"-psn_0_1234567"},
		{"--some-launch-argument"},
		{"Syncing"},
	} {
		if handled, err := runCommand(args); handled || err != nil {
			t.Errorf("runCommand(%q) = %v, %v; want the window", args, handled, err)
		}
	}

	for _, args := range [][]string{{"sync", "-h"}, {"export", "-h"}, {"auth", "-h"}, {"version"}, {"help"}} {
		handled, _ := runCommand(args)
		if !handled {
			t.Errorf("runCommand(%q) did not handle a command it names", args)
		}
	}
}

// TestExportWritesABundleTheAppCanRead is the contract behind "schedule a nightly
// backup": the file the command writes is the same container the export panel writes, so
// the app can restore it.
func TestExportWritesABundleTheAppCanRead(t *testing.T) {
	dataDir := t.TempDir()
	target := filepath.Join(t.TempDir(), "nightly.trovebundle")

	if err := runExport([]string{"-data-dir", dataDir, target}); err != nil {
		t.Fatalf("export: %v", err)
	}

	file, err := os.Open(target)
	if err != nil {
		t.Fatalf("open the bundle: %v", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("stat the bundle: %v", err)
	}
	// A bundle of an empty library still names its producer, which is the version the
	// binary was built with -- "dev" outside a MyGo bundle.
	read, err := bundle.Read(file, info.Size(), bundle.DefaultLimits())
	if err != nil {
		t.Fatalf("the written file is not a bundle: %v", err)
	}
	if read.Manifest.SourceAppVersion == "" {
		t.Error("the bundle does not record the app version that wrote it")
	}
	if read.Manifest.Counts.Bookmarks != 0 {
		t.Errorf("an empty library exported %d bookmark(s)", read.Manifest.Counts.Bookmarks)
	}
}

// TestExportNeedsATarget keeps the command from writing the bundle to an empty path,
// which would put a file named "" somewhere or fail obscurely.
func TestExportNeedsATarget(t *testing.T) {
	if err := runExport([]string{"-data-dir", t.TempDir()}); err == nil {
		t.Fatal("export without a file succeeded")
	}
}

// TestAuthTakesNoArguments keeps a mistyped argument from being ignored: the command's
// only work is the wait, and an argument it does not understand is a mistake the reader
// should hear about before a browser opens.
func TestAuthTakesNoArguments(t *testing.T) {
	if err := runAuth([]string{"-timeout", "1s", "extra"}); err == nil {
		t.Fatal("auth accepted an argument it does not define")
	}
}

// TestTheOpenerRefusesAnythingButTheWeb pins the rule the window's bookmark opener and
// the sign-in page now share. The opener hands its argument to the system, which will
// open a local file or a custom scheme just as happily as a web page, and the address
// can come from an imported backup.
func TestTheOpenerRefusesAnythingButTheWeb(t *testing.T) {
	for _, rawURL := range []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"ftp://example.com/file",
		"x.com/i/flow/login",
		"",
	} {
		if err := openWebURL(rawURL); err == nil {
			t.Errorf("openWebURL(%q) opened it; only http and https may be handed to the system", rawURL)
		}
	}
}
