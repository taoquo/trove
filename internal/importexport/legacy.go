package importexport

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/taoquo/trove/internal/bundle"
	"github.com/taoquo/trove/internal/domain"
)

// Legacy flat-JSON backups.
//
// The legacy format is a single JSON object rather than a ZIP: exportVersion 1 or 2,
// with bookmarks, tags, bookmarkTags, settings and a sync summary at the top level.
// There is no manifest and no checksum, so this path is inherently less safe than a
// bundle — but refusing it would strand every user who has one.
const (
	legacyExportVersion1 = 1
	legacyExportVersion2 = 2
)

// LegacySupportedVersions are the versions this reader accepts.
//
// A number outside this set is rejected rather than guessed at, matching the legacy
// format's own supported-version list.
var LegacySupportedVersions = []int{legacyExportVersion1, legacyExportVersion2}

// ErrNotLegacy reports that the bytes are not a flat JSON backup.
var ErrNotLegacy = errors.New("importexport: not a legacy flat-JSON backup")

// LegacyExport is a flat backup.
type LegacyExport struct {
	ExportVersion int              `json:"exportVersion"`
	SchemaVersion int              `json:"schemaVersion"`
	ExportedAt    string           `json:"exportedAt"`
	Bookmarks     []BookmarkDTO    `json:"bookmarks"`
	Tags          []TagDTO         `json:"tags"`
	BookmarkTags  []BookmarkTagDTO `json:"bookmarkTags"`
	// Settings carries the saved views, which the legacy format kept inline.
	Settings LegacySettings `json:"settings"`
	// Summary and LatestSyncRun are recorded but not imported: the new schema's
	// sync_runs row has fields the legacy summary lacks.
	Summary       json.RawMessage `json:"summary"`
	LatestSyncRun json.RawMessage `json:"latestSyncRun"`
	// Folders appear in exports predating the lists removal.
	Folders         []LegacyFolder         `json:"folders"`
	BookmarkFolders []LegacyBookmarkFolder `json:"bookmarkFolders"`
}

// LegacySettings is the legacy settings blob.
type LegacySettings struct {
	SavedViews []SavedViewDTO `json:"savedViews"`
}

// LegacyFolder is a pre-lists-removal folder.
type LegacyFolder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// CreatedAt and UpdatedAt were not always present.
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// LegacyBookmarkFolder is a pre-lists-removal membership.
type LegacyBookmarkFolder struct {
	BookmarkID string `json:"bookmarkId"`
	FolderID   string `json:"folderId"`
}

// LegacyWarning is a consequence the user must be told about.
//
// It exists because one of them costs real money: an old backup carries no
// analysis ledger, so importing it means the first auto-tagging run re-analyses
// every bookmark and charges the user's provider account for it.
//
// It aliases the bundle package's type so the preview and the report cannot
// disagree about what a warning is.
type LegacyWarning = bundle.LegacyWarning

// Warning kinds.
const (
	WarningNoAnalysisLedger = "no-analysis-ledger"
	WarningNoSuggestions    = "no-suggestions"
	WarningNoChecksum       = "no-checksum"
)

// LooksLikeLegacy reports whether the bytes could be a flat backup.
//
// It is a cheap prefix test, so a ZIP is never handed to the JSON reader and a
// JSON file is never handed to the ZIP reader.
func LooksLikeLegacy(data []byte) bool {
	trimmed := strings.TrimLeft(string(data), " \t\r\n")
	return strings.HasPrefix(trimmed, "{")
}

// ParseLegacy reads a flat backup, with the warnings it implies.
func ParseLegacy(data []byte) (LegacyExport, []LegacyWarning, error) {
	if !LooksLikeLegacy(data) {
		return LegacyExport{}, nil, ErrNotLegacy
	}

	var legacy LegacyExport
	if err := json.Unmarshal(data, &legacy); err != nil {
		return LegacyExport{}, nil, fmt.Errorf("%w: %v", ErrNotLegacy, err)
	}

	// A missing exportVersion is not a version 1 backup: it is some other JSON
	// file, and guessing would risk importing something unrelated.
	if legacy.ExportVersion == 0 {
		return LegacyExport{}, nil, fmt.Errorf("%w: no exportVersion field", ErrNotLegacy)
	}

	supported := false
	for _, version := range LegacySupportedVersions {
		if legacy.ExportVersion == version {
			supported = true
			break
		}
	}
	if !supported {
		return LegacyExport{}, nil, fmt.Errorf(
			"%w: export version %d is not supported (this build reads %v)",
			ErrValidation, legacy.ExportVersion, LegacySupportedVersions)
	}

	var warnings []LegacyWarning
	// The legacy format has no checksum at all, so the user cannot be told the file
	// is intact — only that it parsed.
	warnings = append(warnings, LegacyWarning{
		Kind:   WarningNoChecksum,
		Detail: "this backup has no checksum, so its contents cannot be verified as intact",
	})
	warnings = append(warnings, LegacyWarning{
		Kind:   WarningNoSuggestions,
		Detail: "this backup has no tag suggestions, so previously reviewed ones are not restored",
	})
	warnings = append(warnings, LegacyWarning{
		Kind: WarningNoAnalysisLedger,
		Detail: "this backup has no analysis history, so the first auto-tagging run will " +
			"analyse every bookmark again and use your provider quota",
	})

	return legacy, warnings, nil
}

// toBundle converts a flat backup into the bundle shape.
//
// It is a real conversion rather than a parallel import path: one applier means one
// place where merge rules live, so a legacy file and a bundle cannot behave
// differently.
func (l LegacyExport) toBundle() (*bundle.Bundle, error) {
	files := map[string][]byte{}

	encode := func(name string, value any) error {
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("importexport: encode %s: %w", name, err)
		}
		files[name] = encoded
		return nil
	}

	if err := encode(bundle.FileBookmarks, l.Bookmarks); err != nil {
		return nil, err
	}
	if err := encode(bundle.FileTags, l.Tags); err != nil {
		return nil, err
	}
	if err := encode(bundle.FileBookmarkTags, l.BookmarkTags); err != nil {
		return nil, err
	}
	// The legacy format had no separate files for these, so they are empty rather
	// than absent: an empty array is unambiguous, and the warnings already say why.
	if err := encode(bundle.FileTagSuggestions, []SuggestionDTO{}); err != nil {
		return nil, err
	}
	if err := encode(bundle.FileLLMAnalysis, []AnalysisDTO{}); err != nil {
		return nil, err
	}
	if err := encode(bundle.FileLists, l.lists()); err != nil {
		return nil, err
	}
	if err := encode(bundle.FileBookmarkLists, l.memberships()); err != nil {
		return nil, err
	}
	// The saved views lived inside the settings blob.
	views := l.Settings.SavedViews
	if views == nil {
		views = []SavedViewDTO{}
	}
	if err := encode(bundle.FileSavedViews, views); err != nil {
		return nil, err
	}

	manifest := bundle.Manifest{
		Format:        bundle.Format,
		FormatVersion: bundle.FormatVersion,
		SourceBaseline: bundle.SourceBaseline{
			// The flat format is the legacy one, so its schema applies.
			LegacyDBVersion: l.SchemaVersion,
			ExportVersion:   l.ExportVersion,
		},
		ExportedAt: l.ExportedAt,
	}
	for name, contents := range files {
		manifest.Files = append(manifest.Files, bundle.FileEntryFor(name, contents, countRecords(contents)))
	}
	manifest.Counts = bundle.Counts{
		Bookmarks:     len(l.Bookmarks),
		Tags:          len(l.Tags),
		BookmarkTags:  len(l.BookmarkTags),
		Lists:         len(l.lists()),
		BookmarkLists: len(l.memberships()),
		SavedViews:    len(views),
	}

	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("importexport: encode manifest: %w", err)
	}
	files[bundle.FileManifest] = manifestBody

	return &bundle.Bundle{Manifest: manifest, Files: files}, nil
}

// countRecords reads back how many records an encoded array holds, so the
// manifest describes the bytes rather than a guess.
func countRecords(contents []byte) int {
	var records []json.RawMessage
	if err := json.Unmarshal(contents, &records); err != nil {
		return 0
	}
	return len(records)
}

// lists converts the legacy folders, if any.
//
// The legacy format dropped lists at one point, so a version 2 backup has no folders
// and a version 1 backup may. The Inbox is never taken from a file: it is a system
// row the schema creates.
func (l LegacyExport) lists() []ListDTO {
	lists := make([]ListDTO, 0, len(l.Folders))
	for _, folder := range l.Folders {
		if folder.ID == "" || folder.ID == domain.InboxListID {
			continue
		}
		created := folder.CreatedAt
		if created == "" {
			created = l.ExportedAt
		}
		updated := folder.UpdatedAt
		if updated == "" {
			updated = created
		}
		lists = append(lists, ListDTO{
			ID: folder.ID, Name: folder.Name,
			NormalizedName: domain.NormalizeListName(folder.Name),
			CreatedAt:      created, UpdatedAt: updated,
		})
	}
	return lists
}

// memberships converts the legacy folder assignments.
func (l LegacyExport) memberships() []BookmarkListDTO {
	memberships := make([]BookmarkListDTO, 0, len(l.BookmarkFolders))
	for _, membership := range l.BookmarkFolders {
		if membership.BookmarkID == "" || membership.FolderID == "" {
			continue
		}
		memberships = append(memberships, BookmarkListDTO{
			BookmarkID: membership.BookmarkID,
			ListID:     membership.FolderID,
			UpdatedAt:  l.ExportedAt,
		})
	}
	return memberships
}
