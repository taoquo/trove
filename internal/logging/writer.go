package logging

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"sync"
)

// rotatingFile appends to one path and shifts it aside when it reaches its limit.
//
// The copies are trove.log, trove.log.1 and trove.log.2 for Keep 3: the current file, then
// the previous ones newest first. The oldest is removed before the shift, so a rotation
// costs three renames and no copy, and the set is never larger than Keep.
type rotatingFile struct {
	path string
	max  int64
	keep int

	mu   sync.Mutex
	file *os.File
	size int64
}

// openRotating opens (or creates) the current file and learns its length.
func openRotating(path string, max int64, keep int) (*rotatingFile, error) {
	if max <= 0 {
		return nil, fmt.Errorf("logging: a rotation limit of %d bytes cannot hold a line", max)
	}
	if keep < 1 {
		return nil, fmt.Errorf("logging: keeping %d files would leave nowhere to write", keep)
	}

	file, size, err := openAppend(path)
	if err != nil {
		return nil, err
	}
	return &rotatingFile{path: path, max: max, keep: keep, file: file, size: size}, nil
}

// openAppend opens path for appending with the library's own permissions, refusing a
// file that belongs to someone else's mode. It returns the size the log will grow from.
func openAppend(path string) (*os.File, int64, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, 0, fmt.Errorf("logging: open %s: %w", path, err)
	}
	// An existing file keeps the mode it was created with; a log is as private as the
	// library, so the mode is enforced rather than merely requested.
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("logging: restrict %s: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("logging: stat %s: %w", path, err)
	}
	return file, info.Size(), nil
}

// Write appends p, rotating first when the line would take the file past its limit.
func (w *rotatingFile) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size > 0 && w.size+int64(len(p)) > w.max {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

// rotate moves the current file to .1 and drops the oldest copy.
//
// A rename that finds nothing is not a failure: the first rotations of a new log have
// nothing to shift, and an empty slot is the state they are moving toward anyway.
func (w *rotatingFile) rotate() error {
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("logging: close %s: %w", w.path, err)
	}
	w.file = nil

	oldest := w.path + "." + strconv.Itoa(w.keep-1)
	if err := os.Remove(oldest); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("logging: remove %s: %w", oldest, err)
	}
	for i := w.keep - 2; i >= 1; i-- {
		from := w.path + "." + strconv.Itoa(i)
		to := w.path + "." + strconv.Itoa(i+1)
		if err := os.Rename(from, to); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("logging: rotate %s: %w", from, err)
		}
	}
	if err := os.Rename(w.path, w.path+".1"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("logging: rotate %s: %w", w.path, err)
	}

	file, size, err := openAppend(w.path)
	if err != nil {
		return err
	}
	w.file = file
	w.size = size
	return nil
}

// Close closes the open file. A second close is a no-op.
func (w *rotatingFile) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	if err != nil {
		return fmt.Errorf("logging: close %s: %w", w.path, err)
	}
	return nil
}

// redactWriter removes credential shapes from each write before it reaches the disk.
type redactWriter struct {
	w io.Writer
}

// Write redacts p and writes it. It reports len(p) on success so the writers above it see
// the bytes they handed over consumed, whatever the redaction shortened.
func (r *redactWriter) Write(p []byte) (int, error) {
	if _, err := r.w.Write([]byte(redact(string(p)))); err != nil {
		return 0, err
	}
	return len(p), nil
}
