package bundle

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

// ExportData is what a caller offers to a bundle.
//
// Each field is a slice the writer marshals itself, so the record count in the
// manifest is derived from the same bytes that are written rather than from a
// number the caller supplied.
type ExportData struct {
	Bookmarks      []json.RawMessage
	Tags           []json.RawMessage
	BookmarkTags   []json.RawMessage
	Lists          []json.RawMessage
	BookmarkLists  []json.RawMessage
	TagSuggestions []json.RawMessage
	LLMAnalysis    []json.RawMessage
	SavedViews     []json.RawMessage
	// Settings is an object, not an array: it is the filtered preference set.
	//
	// It is supplied already filtered by the caller, because which keys may leave
	// the machine is a policy decision that lives with the settings, not here.
	Settings map[string]json.RawMessage
	// SyncRuns and RawPayloads are optional and large.
	SyncRuns    []json.RawMessage
	RawPayloads []json.RawMessage
	// Tombstones is the record of posts deleted locally. It is small and always
	// written: a restore without it lets the next sync bring back what the user
	// deliberately removed.
	Tombstones []json.RawMessage
}

// WriteOptions configures an export.
type WriteOptions struct {
	SourceAppVersion string
	Baseline         SourceBaseline
	// IncludeSyncRuns and IncludeRawPayloads record what was chosen, so a reader
	// can tell an excluded file from a missing one.
	IncludeSyncRuns    bool
	IncludeRawPayloads bool
	Now                time.Time
}

// Write produces a bundle.
//
// Each file is streamed into its zip entry as it is produced, and the manifest is
// written last, from the sizes and hashes collected while streaming. The earlier
// shape buffered every file's body in memory before writing any of them, which puts
// the whole library in RAM twice -- once as records and once as encoded arrays --
// exactly when it is largest. A reader looks entries up by name, so the manifest
// being last costs nothing.
func Write(w io.Writer, data ExportData, options WriteOptions) error {
	archive := zip.NewWriter(w)

	manifest := Manifest{
		Format:           Format,
		FormatVersion:    FormatVersion,
		SourceAppVersion: options.SourceAppVersion,
		SourceBaseline:   options.Baseline,
		ExportedAt:       Now(options.Now),
		Options: Options{
			IncludeRawPayloads: options.IncludeRawPayloads,
			IncludeSyncRuns:    options.IncludeSyncRuns,
		},
	}

	// Arrays, in a fixed order, so two exports of the same data are byte-identical
	// apart from the timestamp.
	arrays := []struct {
		name    string
		records []json.RawMessage
	}{
		{FileBookmarks, data.Bookmarks},
		{FileTags, data.Tags},
		{FileBookmarkTags, data.BookmarkTags},
		{FileLists, data.Lists},
		{FileBookmarkLists, data.BookmarkLists},
		{FileTagSuggestions, data.TagSuggestions},
		{FileLLMAnalysis, data.LLMAnalysis},
		{FileSavedViews, data.SavedViews},
		{FileTombstones, data.Tombstones},
	}
	if options.IncludeSyncRuns {
		arrays = append(arrays, struct {
			name    string
			records []json.RawMessage
		}{FileSyncRuns, data.SyncRuns})
	}
	if options.IncludeRawPayloads {
		arrays = append(arrays, struct {
			name    string
			records []json.RawMessage
		}{FileRawPayloads, data.RawPayloads})
	}

	for _, entry := range arrays {
		file, err := writeArrayEntry(archive, entry.name, entry.records)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, file)
	}

	// The settings object is two-space indented, so a user can read what their
	// backup contains without a JSON tool.
	settings, err := writeSettingsEntry(archive, data.Settings)
	if err != nil {
		return err
	}
	manifest.Files = append(manifest.Files, settings)

	manifest.Counts = countsFor(data, options)

	manifestBody, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("bundle: encode manifest: %w", err)
	}
	if err := writeEntry(archive, FileManifest, manifestBody); err != nil {
		return err
	}

	if err := archive.Close(); err != nil {
		return fmt.Errorf("bundle: finish archive: %w", err)
	}
	return nil
}

// writeEntry adds one file at the archive root.
func writeEntry(archive *zip.Writer, name string, body []byte) error {
	entry, err := createEntry(archive, name)
	if err != nil {
		return err
	}
	if _, err := entry.Write(body); err != nil {
		return fmt.Errorf("bundle: write %s: %w", name, err)
	}
	return nil
}

// createEntry opens one whitelisted file in the archive.
func createEntry(archive *zip.Writer, name string) (io.Writer, error) {
	if !isKnownFile(name) {
		// A guard rather than a formality: nothing outside the whitelist may be
		// written, so a caller cannot smuggle a file into a bundle.
		return nil, fmt.Errorf("bundle: %q is not a bundle file", name)
	}

	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o644)

	entry, err := archive.CreateHeader(header)
	if err != nil {
		return nil, fmt.Errorf("bundle: create %s: %w", name, err)
	}
	return entry, nil
}

// writeArrayEntry streams one record array into the archive and describes what
// it wrote.
//
// The hash and the byte count are collected from the same stream the archive
// receives, so the manifest can never describe a file other than the one in the
// zip. Nothing here holds the array in memory: a library of a hundred thousand
// bookmarks is encoded straight into the deflate stream.
func writeArrayEntry(archive *zip.Writer, name string, records []json.RawMessage) (FileEntry, error) {
	entry, err := createEntry(archive, name)
	if err != nil {
		return FileEntry{}, err
	}

	hash := sha256.New()
	bytes := &byteCounter{}
	if err := writeArray(io.MultiWriter(entry, hash, bytes), records); err != nil {
		return FileEntry{}, fmt.Errorf("bundle: write %s: %w", name, err)
	}

	return FileEntry{
		Path:    name,
		Records: len(records),
		Bytes:   bytes.n,
		SHA256:  hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

// writeSettingsEntry streams the settings object into the archive.
func writeSettingsEntry(archive *zip.Writer, settings map[string]json.RawMessage) (FileEntry, error) {
	entry, err := createEntry(archive, FileSettings)
	if err != nil {
		return FileEntry{}, err
	}

	hash := sha256.New()
	bytes := &byteCounter{}
	if err := writeSettings(io.MultiWriter(entry, hash, bytes), settings); err != nil {
		return FileEntry{}, fmt.Errorf("bundle: write %s: %w", FileSettings, err)
	}

	return FileEntry{
		Path:   FileSettings,
		Bytes:  bytes.n,
		SHA256: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

// byteCounter counts what passes through it, so a manifest entry can report the
// size of the stream rather than the size of some buffer that may not exist.
type byteCounter struct{ n int64 }

func (c *byteCounter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// writeArray renders a record array, preserving the caller's own encoding.
//
// The records arrive already marshalled so their field order and shape are
// whatever the exporter decided; re-marshalling through a Go struct would
// silently drop any field this build does not know about.
func writeArray(w io.Writer, records []json.RawMessage) error {
	if records == nil {
		records = []json.RawMessage{}
	}

	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	for i, record := range records {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if !json.Valid(record) {
			return fmt.Errorf("record %d is not valid JSON", i)
		}
		if _, err := w.Write(record); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}

// writeSettings renders the settings object with stable key order.
func writeSettings(w io.Writer, settings map[string]json.RawMessage) error {
	if len(settings) == 0 {
		_, err := io.WriteString(w, "{}")
		return err
	}

	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	for i, key := range keys {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if !json.Valid(settings[key]) {
			return fmt.Errorf("setting %q is not valid JSON", key)
		}
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return err
		}
		if _, err := w.Write(encodedKey); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ":"); err != nil {
			return err
		}
		if _, err := w.Write(settings[key]); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "}")
	return err
}

// countsFor tallies the manifest's counts from the data actually written.
func countsFor(data ExportData, options WriteOptions) Counts {
	counts := Counts{
		Bookmarks:      len(data.Bookmarks),
		Tags:           len(data.Tags),
		BookmarkTags:   len(data.BookmarkTags),
		Lists:          len(data.Lists),
		BookmarkLists:  len(data.BookmarkLists),
		TagSuggestions: len(data.TagSuggestions),
		LLMAnalysis:    len(data.LLMAnalysis),
		SavedViews:     len(data.SavedViews),
	}
	if options.IncludeSyncRuns {
		counts.SyncRuns = len(data.SyncRuns)
	}
	if options.IncludeRawPayloads {
		counts.RawPayloads = len(data.RawPayloads)
	}
	counts.Tombstones = len(data.Tombstones)
	return counts
}

// Preview is what a bundle would import, without importing it.
//
// It exists so the user can see the shape of the merge before agreeing to it,
// which is the whole point of separating Import from Restore.
type Preview struct {
	Manifest Manifest
	// NewRecords is how many records the bundle holds that the library does not.
	NewRecords int
	// ExistingRecords is how many it holds that the library already has.
	ExistingRecords int
	TotalBytes      int64
	// Warnings are consequences the user must be told about before agreeing.
	Warnings []LegacyWarning
	// Legacy is true when the input was a flat JSON backup rather than a bundle.
	Legacy bool
}

// LegacyWarning is a consequence the user must be told about.
//
// It is declared here rather than in the importer because the preview and the
// report both carry it, and one type means the two cannot disagree.
type LegacyWarning struct {
	Kind   string
	Detail string
}
