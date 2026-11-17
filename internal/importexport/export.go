package importexport

// Export is the read half of the package: what a backup carries, and the options that
// choose how much of it. It is split from import.go and restore.go because the halves are
// different jobs -- this one reads the library out, import.go writes a bundle in, and
// restore.go replaces the library and rolls a failed restore back.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/taoquo/trove/internal/bundle"
	"github.com/taoquo/trove/internal/domain"
)

// ExportOptions chooses what an export includes.
type ExportOptions struct {
	IncludeSyncRuns    bool
	IncludeRawPayloads bool
}

// DefaultExportOptions are the legacy export defaults: sync history yes, raw payloads no.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{IncludeSyncRuns: true, IncludeRawPayloads: false}
}

// SnapshotExportOptions are the options a restore's in-memory snapshot uses.
//
// A snapshot is not a user-facing backup, it is the state a failed restore must put
// back, so it includes everything the library holds: raw payloads included. The
// user-facing default leaves them out to keep a backup small, which is the one thing
// a rollback must not do -- restoring the snapshot would then drop diagnostic data
// the failed restore never touched.
func SnapshotExportOptions() ExportOptions {
	return ExportOptions{IncludeSyncRuns: true, IncludeRawPayloads: true}
}

// Export writes a bundle into memory.
//
// Callers that have a file to write should prefer ExportTo, which streams the
// bundle into it. This shape is kept for the restore snapshot and for tests that
// need the bytes themselves.
func (s *Service) Export(ctx context.Context, options ExportOptions) ([]byte, error) {
	var buffer bytes.Buffer
	if err := s.ExportTo(ctx, &buffer, options); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// ExportTo writes a bundle to w without holding it in memory.
//
// The records are still read in one pass, but each file is encoded straight into
// the archive as it goes, so the peak memory is the library plus the compression
// window rather than the library plus every encoded array.
//
// The settings are taken from the injected allowlist, so a credential or a private
// endpoint cannot reach the file even if it is stored in the same table.
func (s *Service) ExportTo(ctx context.Context, w io.Writer, options ExportOptions) error {
	data, err := s.collect(ctx, options)
	if err != nil {
		return err
	}

	if err := bundle.Write(w, data, bundle.WriteOptions{
		SourceAppVersion:   s.AppVersion,
		IncludeSyncRuns:    options.IncludeSyncRuns,
		IncludeRawPayloads: options.IncludeRawPayloads,
		Baseline: bundle.SourceBaseline{
			// The native app's data descends from the legacy schema; recording it
			// lets an importer apply the right mappings.
			LegacyDBVersion:       9,
			SettingsSchemaVersion: 7,
			ExportVersion:         2,
		},
		Now: s.clock.Now(),
	}); err != nil {
		return err
	}
	return nil
}

// ErrIncomplete reports a service built without the stores an export needs.
//
// Export touches six tables, so a partial set of repositories would panic on a nil
// interface. Failing with a message is the difference between a clear
// configuration error and a crash with a stack trace.
var ErrIncomplete = errors.New("importexport: the service is missing a repository")

// require checks the stores an export reads.
func (s *Service) require() error {
	switch {
	case s.repos.Bookmarks == nil:
		return fmt.Errorf("%w: bookmarks", ErrIncomplete)
	case s.repos.Tags == nil:
		return fmt.Errorf("%w: tags", ErrIncomplete)
	case s.repos.Lists == nil:
		return fmt.Errorf("%w: lists", ErrIncomplete)
	case s.repos.Suggestions == nil:
		return fmt.Errorf("%w: suggestions", ErrIncomplete)
	case s.repos.Analysis == nil:
		return fmt.Errorf("%w: analysis", ErrIncomplete)
	case s.repos.SavedViews == nil:
		return fmt.Errorf("%w: saved views", ErrIncomplete)
	}
	return nil
}

// collect reads every table into the bundle's wire shape.
func (s *Service) collect(ctx context.Context, options ExportOptions) (bundle.ExportData, error) {
	data := bundle.ExportData{}

	if err := s.require(); err != nil {
		return data, err
	}

	bookmarks, err := s.repos.Bookmarks.All(ctx)
	if err != nil {
		return data, err
	}
	dtos := make([]BookmarkDTO, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		dtos = append(dtos, NewBookmarkDTO(bookmark))
	}
	if data.Bookmarks, err = marshalRecords(dtos); err != nil {
		return data, fmt.Errorf("importexport: encode bookmarks: %w", err)
	}

	tags, err := s.repos.Tags.List(ctx)
	if err != nil {
		return data, err
	}
	tagDTOs := make([]TagDTO, 0, len(tags))
	for _, tag := range tags {
		tagDTOs = append(tagDTOs, NewTagDTO(tag))
	}
	if data.Tags, err = marshalRecords(tagDTOs); err != nil {
		return data, fmt.Errorf("importexport: encode tags: %w", err)
	}

	relations, err := s.repos.Tags.AllRelations(ctx)
	if err != nil {
		return data, err
	}
	relationDTOs := make([]BookmarkTagDTO, 0, len(relations))
	for _, relation := range relations {
		relationDTOs = append(relationDTOs, BookmarkTagDTO{
			BookmarkID: relation.BookmarkID,
			TagID:      relation.TagID,
			CreatedAt:  domain.FormatTimestamp(relation.CreatedAt),
			Source:     relation.Source,
		})
	}
	if data.BookmarkTags, err = marshalRecords(relationDTOs); err != nil {
		return data, fmt.Errorf("importexport: encode bookmark tags: %w", err)
	}

	lists, err := s.repos.Lists.List(ctx)
	if err != nil {
		return data, err
	}
	listDTOs := make([]ListDTO, 0, len(lists))
	for _, list := range lists {
		listDTOs = append(listDTOs, NewListDTO(list))
	}
	if data.Lists, err = marshalRecords(listDTOs); err != nil {
		return data, fmt.Errorf("importexport: encode lists: %w", err)
	}

	// Bookmark-to-list membership is read per bookmark: the repository exposes it
	// that way, and a bookmark's list is what the format records.
	memberships := make([]BookmarkListDTO, 0, len(bookmarks))
	ids := make([]string, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		ids = append(ids, bookmark.TweetID)
	}
	byBookmark, err := s.repos.Bookmarks.ListByBookmark(ctx, ids)
	if err != nil {
		return data, err
	}
	// In the order the bookmarks were read, never the map's. Go visits a map in a random
	// order, so the file came out differently on every export of the same unchanged
	// library -- and the manifest carries a digest of the parts, so it changed too. Two
	// backups of one library are the same bytes now.
	for _, bookmarkID := range ids {
		list, found := byBookmark[bookmarkID]
		if !found {
			continue
		}
		memberships = append(memberships, BookmarkListDTO{
			BookmarkID: bookmarkID,
			ListID:     list.ID,
			UpdatedAt:  domain.FormatTimestamp(list.UpdatedAt),
		})
	}
	if data.BookmarkLists, err = marshalRecords(memberships); err != nil {
		return data, fmt.Errorf("importexport: encode bookmark lists: %w", err)
	}

	suggestions, err := s.repos.Suggestions.All(ctx)
	if err != nil {
		return data, err
	}
	suggestionDTOs := make([]SuggestionDTO, 0, len(suggestions))
	for _, suggestion := range suggestions {
		suggestionDTOs = append(suggestionDTOs, NewSuggestionDTO(suggestion))
	}
	if data.TagSuggestions, err = marshalRecords(suggestionDTOs); err != nil {
		return data, fmt.Errorf("importexport: encode suggestions: %w", err)
	}

	analysis, err := s.repos.Analysis.List(ctx)
	if err != nil {
		return data, err
	}
	analysisDTOs := make([]AnalysisDTO, 0, len(analysis))
	for _, record := range analysis {
		analysisDTOs = append(analysisDTOs, NewAnalysisDTO(record))
	}
	if data.LLMAnalysis, err = marshalRecords(analysisDTOs); err != nil {
		return data, fmt.Errorf("importexport: encode analysis: %w", err)
	}

	views, err := s.repos.SavedViews.List(ctx)
	if err != nil {
		return data, err
	}
	viewDTOs := make([]SavedViewDTO, 0, len(views))
	for _, view := range views {
		viewDTOs = append(viewDTOs, NewSavedViewDTO(view))
	}
	if data.SavedViews, err = marshalRecords(viewDTOs); err != nil {
		return data, fmt.Errorf("importexport: encode saved views: %w", err)
	}

	// Sync history is optional: it is diagnostic, and a user restoring a backup
	// rarely wants another machine's sync log.
	if options.IncludeSyncRuns && s.repos.SyncRuns != nil {
		runs, err := s.repos.SyncRuns.Recent(ctx, syncRunsExported)
		if err != nil {
			return data, err
		}
		runDTOs := make([]SyncRunDTO, 0, len(runs))
		for _, run := range runs {
			runDTOs = append(runDTOs, NewSyncRunDTO(run))
		}
		if data.SyncRuns, err = marshalRecords(runDTOs); err != nil {
			return data, fmt.Errorf("importexport: encode sync runs: %w", err)
		}
	}

	// Raw payloads are off by default: they duplicate the parsed fields and are
	// the largest thing in the database.
	if options.IncludeRawPayloads && s.repos.Bookmarks != nil {
		payloads, err := s.repos.Bookmarks.RawPayloads(ctx, ids)
		if err != nil {
			return data, err
		}
		payloadDTOs := make([]RawPayloadDTO, 0, len(payloads))
		for _, payload := range payloads {
			payloadDTOs = append(payloadDTOs, NewRawPayloadDTO(payload))
		}
		if data.RawPayloads, err = marshalRecords(payloadDTOs); err != nil {
			return data, fmt.Errorf("importexport: encode raw payloads: %w", err)
		}
	}

	// Tombstones are always written, whatever the options say: they are small, and a
	// backup without them lets a restore resurrect every post the user deleted as soon
	// as the next sync sees it. A version 1 bundle that lacks the file simply means
	// the exporting device had no local deletes to record.
	if s.repos.Bookmarks != nil {
		tombstones, err := s.repos.Bookmarks.Tombstones(ctx)
		if err != nil {
			return data, err
		}
		tombstoneDTOs := make([]TombstoneDTO, 0, len(tombstones))
		for _, tombstone := range tombstones {
			tombstoneDTOs = append(tombstoneDTOs, NewTombstoneDTO(tombstone))
		}
		if data.Tombstones, err = marshalRecords(tombstoneDTOs); err != nil {
			return data, fmt.Errorf("importexport: encode tombstones: %w", err)
		}
	}

	// The settings come from the allowlist, never from a raw table dump.
	if s.ExportableSettings != nil {
		if data.Settings, err = s.ExportableSettings(ctx); err != nil {
			return data, err
		}
	}

	return data, nil
}
