// Package bundle reads and writes .xmbundle backup files.
//
// The format is documented in docs/data-format-v1.md. Two properties shape this
// package:
//
//   - Reading is hostile-input territory. A bundle may come from anywhere, so
//     nothing is trusted: not the entry names, not the declared sizes, not the
//     compressed data. Validation is all-or-nothing, and a failure never touches
//     existing data.
//   - Writing is an allowlist. What leaves the machine is decided here, so a new
//     table added later is excluded by default rather than exported by accident.
package bundle

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

// Format identifies a bundle, so a wrong file is rejected before anything else.
const Format = "x-master-bundle"

// FormatVersion is the version this build writes and the highest it reads.
const FormatVersion = 1

// The file names a bundle may contain.
//
// Reading is driven by this list rather than by the archive's entry names, which
// is what makes Zip Slip structurally impossible: a name that is not here is never
// looked at, so there is no path to sanitise.
const (
	FileManifest       = "manifest.json"
	FileBookmarks      = "bookmarks.json"
	FileTags           = "tags.json"
	FileBookmarkTags   = "bookmark-tags.json"
	FileLists          = "lists.json"
	FileBookmarkLists  = "bookmark-lists.json"
	FileTagSuggestions = "tag-suggestions.json"
	FileLLMAnalysis    = "llm-analysis.json"
	FileSavedViews     = "saved-views.json"
	FileClassification = "classification-rules.json"
	FileSettings       = "settings.json"
	FileSyncRuns       = "sync-runs.json"
	FileRawPayloads    = "raw-payloads.json"
)

// KnownFiles is the whitelist.
var KnownFiles = []string{
	FileManifest, FileBookmarks, FileTags, FileBookmarkTags, FileLists,
	FileBookmarkLists, FileTagSuggestions, FileLLMAnalysis, FileSavedViews,
	FileClassification, FileSettings, FileSyncRuns, FileRawPayloads,
}

// Limits bound what a bundle may expand to.
//
// They exist because a ZIP's declared sizes are attacker-controlled: a few
// kilobytes can claim to be gigabytes, and reading it trustingly exhausts memory
// long before any hash check runs. Every limit is checked before the bytes are
// read, not after.
type Limits struct {
	// MaxEntries bounds the archive's entry count. A real bundle has at most
	// len(KnownFiles) entries.
	MaxEntries int
	// MaxFileBytes bounds one uncompressed file.
	MaxFileBytes int64
	// MaxTotalBytes bounds the uncompressed total.
	MaxTotalBytes int64
	// MaxCompressionRatio bounds uncompressed/compressed. A JSON bundle compresses
	// roughly 10:1; a thousand-to-one is a bomb, not data.
	MaxCompressionRatio int64
	// MaxRecords bounds the total records across all files.
	MaxRecords int
	// MaxStringLength bounds any single string read from the bundle, so a hostile
	// file cannot put a megabyte into one tag name.
	MaxStringLength int
}

// DefaultLimits are the limits a real bundle is comfortably inside.
func DefaultLimits() Limits {
	return Limits{
		MaxEntries:          len(KnownFiles) + 8,
		MaxFileBytes:        512 << 20, // 512 MiB
		MaxTotalBytes:       1 << 30,   // 1 GiB
		MaxCompressionRatio: 1000,
		MaxRecords:          2_000_000,
		MaxStringLength:     1 << 20, // 1 MiB
	}
}

// FileEntry is one file's declaration in the manifest.
type FileEntry struct {
	Path    string `json:"path"`
	Records int    `json:"records"`
	Bytes   int64  `json:"bytes"`
	SHA256  string `json:"sha256"`
}

// SourceBaseline records which implementation produced the data.
//
// It is required rather than informational: the importer needs it to know which
// schema mappings apply, such as the sync error-kind vocabulary.
type SourceBaseline struct {
	ExtensionCommit       string `json:"extensionCommit"`
	ExtensionDBVersion    int    `json:"extensionDbVersion"`
	SettingsSchemaVersion int    `json:"settingsSchemaVersion"`
	ExportVersion         int    `json:"exportVersion"`
}

// Options records what the exporter chose to include, so a reader can tell "this
// file is absent because it was excluded" from "this file is missing".
type Options struct {
	IncludeRawPayloads bool `json:"includeRawPayloads"`
	IncludeSyncRuns    bool `json:"includeSyncRuns"`
}

// Counts is the manifest's record tally, for a preview before importing.
type Counts struct {
	Bookmarks      int `json:"bookmarks"`
	Tags           int `json:"tags"`
	BookmarkTags   int `json:"bookmarkTags"`
	Lists          int `json:"lists"`
	BookmarkLists  int `json:"bookmarkLists"`
	TagSuggestions int `json:"tagSuggestions"`
	LLMAnalysis    int `json:"llmAnalysis"`
	SavedViews     int `json:"savedViews"`
	SyncRuns       int `json:"syncRuns"`
	RawPayloads    int `json:"rawPayloads"`
}

// Manifest describes a bundle.
type Manifest struct {
	Format           string         `json:"format"`
	FormatVersion    int            `json:"formatVersion"`
	SourceAppVersion string         `json:"sourceAppVersion"`
	SourceBaseline   SourceBaseline `json:"sourceBaseline"`
	ExportedAt       string         `json:"exportedAt"`
	Options          Options        `json:"options"`
	Files            []FileEntry    `json:"files"`
	Counts           Counts         `json:"counts"`
}

// Bundle is a validated bundle: everything in it has been checked.
//
// It is only produced by Read, and Read only returns one after every check has
// passed. That is what lets a caller act on it without re-validating.
type Bundle struct {
	Manifest Manifest
	// Files maps a whitelisted name to its verified contents.
	Files map[string][]byte
	// TotalBytes is how much was actually read.
	TotalBytes int64
}

// Decode unmarshals one of the bundle's files.
func Decode[T any](b *Bundle, name string) ([]T, error) {
	raw, present := b.Files[name]
	if !present {
		// An absent file is legal: the manifest says what was included.
		return nil, nil
	}

	var records []T
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("bundle: %s is not a JSON array: %w", name, err)
	}
	return records, nil
}

// validationError names the step that failed.
//
// The step is carried rather than only described, because the spec's validation
// order is a contract: a caller must be able to tell "the file is corrupt" from
// "the version is too new".
type validationError struct {
	Step   string
	Detail string
}

func (e *validationError) Error() string {
	return fmt.Sprintf("bundle: %s: %s", e.Step, e.Detail)
}

func invalid(step, format string, args ...any) error {
	return &validationError{Step: step, Detail: fmt.Sprintf(format, args...)}
}

// StepOf returns the validation step that failed, or an empty string.
func StepOf(err error) string {
	var validation *validationError
	if errors.As(err, &validation) {
		return validation.Step
	}
	return ""
}

// Validation steps, in the order the spec requires.
const (
	StepZipStructure = "zip-structure"
	StepLimits       = "limits"
	StepManifest     = "manifest"
	StepFileMissing  = "file-missing"
	StepChecksum     = "checksum"
	StepRecordCount  = "record-count"
)

// Read validates a bundle and returns it.
//
// Every check runs before any data is returned, so a failure leaves the caller
// with nothing to act on and the existing library untouched.
func Read(r io.ReaderAt, size int64, limits Limits) (*Bundle, error) {
	archive, err := zip.NewReader(r, size)
	if err != nil {
		return nil, invalid(StepZipStructure, "not a readable ZIP: %v", err)
	}

	if err := checkArchiveEntries(archive, limits); err != nil {
		return nil, err
	}

	// --- 1. structure: names are never used as paths ---
	//
	// Reading is driven by the whitelist, so a name is only ever used as a map
	// key. Nothing is written to disk from this package at all, which is what
	// makes traversal impossible rather than merely filtered.
	byName := make(map[string]*zip.File, len(archive.File))
	for _, entry := range archive.File {
		if err := checkEntryName(entry.Name); err != nil {
			return nil, err
		}
		if isKnownFile(entry.Name) {
			byName[entry.Name] = entry
		}
	}

	manifestFile, present := byName[FileManifest]
	if !present {
		return nil, invalid(StepManifest, "%s is missing", FileManifest)
	}

	// --- 3. manifest ---
	manifestBytes, err := readEntry(manifestFile, limits)
	if err != nil {
		return nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, invalid(StepManifest, "%s is not valid JSON: %v", FileManifest, err)
	}
	if manifest.Format != Format {
		return nil, invalid(StepManifest, "format is %q, want %q", manifest.Format, Format)
	}
	if manifest.FormatVersion > FormatVersion {
		return nil, invalid(StepManifest,
			"the bundle is version %d and this app reads up to %d; update the app",
			manifest.FormatVersion, FormatVersion)
	}
	if manifest.FormatVersion < 1 {
		return nil, invalid(StepManifest, "formatVersion %d is not a version", manifest.FormatVersion)
	}

	bundle := &Bundle{Manifest: manifest, Files: make(map[string][]byte, len(byName))}
	bundle.Files[FileManifest] = manifestBytes
	bundle.TotalBytes = int64(len(manifestBytes))

	// --- 4. each declared file exists and matches its hash ---
	totalRecords := 0
	for _, declared := range manifest.Files {
		if err := checkEntryName(declared.Path); err != nil {
			return nil, invalid(StepManifest, "declared path %q is not usable: %v", declared.Path, err)
		}
		if !isKnownFile(declared.Path) {
			return nil, invalid(StepManifest, "declared path %q is not a bundle file", declared.Path)
		}

		entry, present := byName[declared.Path]
		if !present {
			return nil, invalid(StepFileMissing, "%s is declared but absent", declared.Path)
		}

		contents, err := readEntry(entry, limits)
		if err != nil {
			return nil, err
		}
		bundle.TotalBytes += int64(len(contents))

		if int64(len(contents)) != declared.Bytes {
			return nil, invalid(StepChecksum,
				"%s is %d bytes but the manifest declares %d", declared.Path, len(contents), declared.Bytes)
		}

		// The hash is checked before the contents are parsed, so a corrupt file is
		// rejected on its bytes rather than on whatever the JSON happens to do.
		if err := verifySHA256(contents, declared.SHA256); err != nil {
			return nil, invalid(StepChecksum, "%s: %v", declared.Path, err)
		}

		// --- 5. the declared record count must match ---
		if declared.Records > 0 || declared.Path != FileSettings {
			var records []json.RawMessage
			if err := json.Unmarshal(contents, &records); err != nil {
				return nil, invalid(StepRecordCount, "%s is not a JSON array: %v", declared.Path, err)
			}
			if len(records) != declared.Records {
				return nil, invalid(StepRecordCount,
					"%s holds %d records but the manifest declares %d",
					declared.Path, len(records), declared.Records)
			}
			totalRecords += len(records)
		}

		if limits.MaxRecords > 0 && totalRecords > limits.MaxRecords {
			return nil, invalid(StepLimits, "the bundle holds more than %d records", limits.MaxRecords)
		}

		bundle.Files[declared.Path] = contents
	}

	// A file present in the archive but not declared in the manifest is ignored
	// rather than rejected: the manifest is what the reader acts on.
	return bundle, nil
}

// checkArchiveEntries applies the structural and limit checks.
func checkArchiveEntries(archive *zip.Reader, limits Limits) error {
	if limits.MaxEntries > 0 && len(archive.File) > limits.MaxEntries {
		return invalid(StepLimits, "the archive holds %d entries, more than the %d allowed",
			len(archive.File), limits.MaxEntries)
	}

	var declaredTotal int64
	for _, entry := range archive.File {
		// A directory entry is fine as long as it is not used as a path.
		if entry.FileInfo().IsDir() {
			continue
		}

		size := int64(entry.UncompressedSize64)
		if limits.MaxFileBytes > 0 && size > limits.MaxFileBytes {
			return invalid(StepLimits, "%s declares %d bytes, more than the %d allowed",
				entry.Name, size, limits.MaxFileBytes)
		}

		declaredTotal += size
		if limits.MaxTotalBytes > 0 && declaredTotal > limits.MaxTotalBytes {
			return invalid(StepLimits, "the archive declares more than %d bytes uncompressed", limits.MaxTotalBytes)
		}

		// The ratio is computed from the declared sizes, before any decompression:
		// waiting until after would already have spent the memory.
		if limits.MaxCompressionRatio > 0 && entry.CompressedSize64 > 0 {
			ratio := size / int64(entry.CompressedSize64)
			if ratio > limits.MaxCompressionRatio {
				return invalid(StepLimits, "%s compresses %d:1, more than the %d:1 allowed",
					entry.Name, ratio, limits.MaxCompressionRatio)
			}
		}
	}
	return nil
}

// checkEntryName rejects the shapes a traversal needs.
//
// Nothing is written to disk, so this is defence in depth rather than the only
// barrier: if a future caller ever uses an entry name as a path, the name has
// already been refused here.
func checkEntryName(name string) error {
	if name == "" {
		return invalid(StepZipStructure, "an entry has an empty name")
	}
	if strings.ContainsRune(name, 0) {
		return invalid(StepZipStructure, "entry %q contains a NUL byte", name)
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return invalid(StepZipStructure, "entry %q is an absolute path", name)
	}
	// A Windows drive letter is absolute too, even without a leading slash.
	if len(name) >= 2 && name[1] == ':' {
		return invalid(StepZipStructure, "entry %q is an absolute path", name)
	}
	if strings.Contains(name, `\`) {
		return invalid(StepZipStructure, "entry %q uses a Windows separator", name)
	}

	// Clean resolves "a/../b" to "b" and ".." to ".", so comparing against the
	// original catches every traversal spelling at once.
	cleaned := path.Clean(name)
	if cleaned != name {
		return invalid(StepZipStructure, "entry %q is not a clean path", name)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return invalid(StepZipStructure, "entry %q escapes the archive", name)
	}
	if strings.Contains(cleaned, "/") {
		// Files live at the archive root; a nested path is either a mistake or an
		// attempt to slip past a check that only looks at the base name.
		return invalid(StepZipStructure, "entry %q is nested, but bundle files live at the root", name)
	}
	return nil
}

// isKnownFile reports whether a name is one a bundle may contain.
func isKnownFile(name string) bool {
	for _, known := range KnownFiles {
		if name == known {
			return true
		}
	}
	return false
}

// readEntry reads one entry with the per-file limit applied.
//
// The limit is enforced while reading, not after: a declared size is a claim, and
// a lying archive would otherwise be fully materialised before the check.
func readEntry(entry *zip.File, limits Limits) ([]byte, error) {
	reader, err := entry.Open()
	if err != nil {
		return nil, invalid(StepZipStructure, "%s could not be opened: %v", entry.Name, err)
	}
	defer reader.Close()

	limit := limits.MaxFileBytes
	if limit <= 0 {
		limit = DefaultLimits().MaxFileBytes
	}

	// One byte over the limit, so an oversized file is detected rather than
	// silently truncated into something that happens to parse.
	contents, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, invalid(StepZipStructure, "%s could not be read: %v", entry.Name, err)
	}
	if int64(len(contents)) > limit {
		return nil, invalid(StepLimits, "%s is larger than the %d bytes allowed", entry.Name, limit)
	}
	return contents, nil
}

// verifySHA256 compares a file's digest against the manifest's.
func verifySHA256(contents []byte, want string) error {
	if strings.TrimSpace(want) == "" {
		return fmt.Errorf("the manifest declares no sha256")
	}

	sum := sha256.Sum256(contents)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, strings.TrimSpace(want)) {
		return fmt.Errorf("sha256 is %s, but the manifest declares %s", got, want)
	}
	return nil
}

// FileEntryFor builds a manifest entry for a file's contents.
func FileEntryFor(name string, contents []byte, records int) FileEntry {
	sum := sha256.Sum256(contents)
	return FileEntry{
		Path:    name,
		Records: records,
		Bytes:   int64(len(contents)),
		SHA256:  hex.EncodeToString(sum[:]),
	}
}

// Now stamps a manifest's export time the way the extension did.
func Now(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
