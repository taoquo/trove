// Package logging writes the optional troubleshooting log beside the library.
//
// The log is off unless the reader turns it on in Settings › Data safety. When it is on,
// the app's events go to <data>/logs/trove.log, the standard logger's lines included; the
// file rotates at one megabyte and keeps the newest three, so a long-running copy cannot
// fill a disk.
//
// Nothing in the log is content. Event carries a name and whole numbers, which is why a
// post, an API key or a cookie has no path in through it; and the writer redacts the
// shapes a credential takes anyway, because the standard logger still receives error
// strings from transports and from SQL, which this package does not compose.
package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const (
	// DirName is the folder created beside the library.
	DirName = "logs"
	// FileName is the current log.
	FileName = "trove.log"
	// MaxBytes is the size at which the current file is rotated.
	MaxBytes int64 = 1 << 20
	// Keep is how many files exist after a rotation, the current one included.
	Keep = 3
)

// Count is one number in an event: a name the callers of this package control, and a
// whole number.
//
// The value is an int on purpose. An event that could carry a string could carry a post,
// a key or a cookie, and the point of this type is that it cannot.
type Count struct {
	Name  string
	Value int
}

// Controller owns the log file and can open and close it while the app runs, which is
// what the Settings switch does.
type Controller struct {
	dir string

	mu     sync.Mutex
	file   *rotatingFile
	logger *slog.Logger
}

// New returns a controller for the log in dir. It writes nothing until Apply(true).
func New(dir string) *Controller {
	return &Controller{dir: dir}
}

// Apply opens the log when enabled is true and closes it when it is false.
//
// It is idempotent, so the launch path and the settings switch can both state the stored
// preference without the second call doing anything. The log goes to the file and to
// stderr, so a run from a terminal keeps its output while a reader who turned logging on
// also has the file to send.
func (c *Controller) Apply(enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !enabled {
		c.closeLocked()
		return nil
	}
	if c.file != nil {
		return nil
	}

	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("logging: create %s: %w", c.dir, err)
	}
	file, err := openRotating(filepath.Join(c.dir, FileName), MaxBytes, Keep)
	if err != nil {
		return err
	}
	writer := &redactWriter{w: file}
	// Appending to the process's own logger rather than replacing it: the lines the app
	// already writes are its events, and the file is a copy of them, not a second
	// vocabulary.
	log.SetOutput(io.MultiWriter(os.Stderr, writer))
	slog.SetDefault(slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo})))
	c.file = file
	c.logger = slog.Default()
	return nil
}

// Event records one event with its counts.
//
// A closed log drops the event: there is no buffering, because a line that appears after
// the reader turned logging off would be a surprise. The name and the counts are all that
// is written -- the time comes from the handler.
func (c *Controller) Event(name string, counts ...Count) {
	c.mu.Lock()
	logger := c.logger
	c.mu.Unlock()
	if logger == nil {
		return
	}

	attrs := make([]any, 0, len(counts)*2)
	for _, count := range counts {
		attrs = append(attrs, count.Name, count.Value)
	}
	logger.Info(name, attrs...)
}

// Close closes the log and puts the standard logger back on stderr.
func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeLocked()
}

// closeLocked closes the file and restores the process's loggers.
func (c *Controller) closeLocked() error {
	if c.file == nil {
		return nil
	}
	err := c.file.Close()
	c.file = nil
	c.logger = nil
	log.SetOutput(os.Stderr)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	return err
}
