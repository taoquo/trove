package bundle

import (
	"archive/zip"
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
// The manifest is built from the bytes actually written, so a file can never be
// declared with a size, hash or record count that does not describe it.
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

	type pending struct {
		name   string
		body   []byte
		counts int
	}
	var files []pending

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
		body, err := encodeArray(entry.records)
		if err != nil {
			return err
		}
		files = append(files, pending{name: entry.name, body: body, counts: len(entry.records)})
	}

	// The settings object is two-space indented, so a user can read what their
	// backup contains without a JSON tool.
	settingsBody, err := encodeSettings(data.Settings)
	if err != nil {
		return err
	}
	files = append(files, pending{name: FileSettings, body: settingsBody, counts: 0})

	for _, file := range files {
		manifest.Files = append(manifest.Files, FileEntryFor(file.name, file.body, file.counts))
	}
	manifest.Counts = countsFor(data, options)

	manifestBody, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("bundle: encode manifest: %w", err)
	}

	if err := writeEntry(archive, FileManifest, manifestBody); err != nil {
		return err
	}
	for _, file := range files {
		if err := writeEntry(archive, file.name, file.body); err != nil {
			return err
		}
	}

	if err := archive.Close(); err != nil {
		return fmt.Errorf("bundle: finish archive: %w", err)
	}
	return nil
}

// writeEntry adds one file at the archive root.
func writeEntry(archive *zip.Writer, name string, body []byte) error {
	if !isKnownFile(name) {
		// A guard rather than a formality: nothing outside the whitelist may be
		// written, so a caller cannot smuggle a file into a bundle.
		return fmt.Errorf("bundle: %q is not a bundle file", name)
	}

	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o644)

	entry, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("bundle: create %s: %w", name, err)
	}
	if _, err := entry.Write(body); err != nil {
		return fmt.Errorf("bundle: write %s: %w", name, err)
	}
	return nil
}

// encodeArray renders a record array, preserving the caller's own encoding.
//
// The records arrive already marshalled so their field order and shape are
// whatever the exporter decided; re-marshalling through a Go struct would
// silently drop any field this build does not know about.
func encodeArray(records []json.RawMessage) ([]byte, error) {
	if records == nil {
		records = []json.RawMessage{}
	}

	var buffer []byte
	buffer = append(buffer, '[')
	for i, record := range records {
		if i > 0 {
			buffer = append(buffer, ',')
		}
		if !json.Valid(record) {
			return nil, fmt.Errorf("bundle: record %d is not valid JSON", i)
		}
		buffer = append(buffer, record...)
	}
	buffer = append(buffer, ']')
	return buffer, nil
}

// encodeSettings renders the settings object with stable key order.
func encodeSettings(settings map[string]json.RawMessage) ([]byte, error) {
	if len(settings) == 0 {
		return []byte("{}"), nil
	}

	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var buffer []byte
	buffer = append(buffer, '{')
	for i, key := range keys {
		if i > 0 {
			buffer = append(buffer, ',')
		}
		if !json.Valid(settings[key]) {
			return nil, fmt.Errorf("bundle: setting %q is not valid JSON", key)
		}
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buffer = append(buffer, encodedKey...)
		buffer = append(buffer, ':')
		buffer = append(buffer, settings[key]...)
	}
	buffer = append(buffer, '}')
	return buffer, nil
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
