package importexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/taoquo/trove/internal/bundle"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
)

// ErrValidation reports a bundle that failed a check. The wrapped error names the
// step, so a caller can tell corruption from a version mismatch.
var ErrValidation = errors.New("importexport: the bundle was rejected")

// Service exports, imports and restores libraries.
type Service struct {
	repos ports.Repositories
	clock ports.Clock
	// AppVersion is recorded in the manifest, so a bundle says what produced it.
	AppVersion string
	// ExportableSettings returns the settings a backup may carry.
	//
	// It is injected rather than read here because which keys may leave the machine
	// is a policy the settings package owns; duplicating the allowlist would let
	// the two drift apart.
	ExportableSettings func(ctx context.Context) (map[string]json.RawMessage, error)
	// clearSecrets forgets stored secrets. It is installed by the composition root,
	// which holds both this service and the credential store; it stays nil in tests that
	// do not care, and the reset still runs.
	clearSecrets func(context.Context) error
	// RestoreSettings applies the settings from a bundle.
	RestoreSettings func(ctx context.Context, settings map[string]json.RawMessage) error
}

// New builds a service.
func New(repos ports.Repositories, clock ports.Clock, appVersion string) *Service {
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Service{repos: repos, clock: clock, AppVersion: appVersion}
}

// ExportOptions chooses what an export includes.
type ExportOptions struct {
	IncludeSyncRuns    bool
	IncludeRawPayloads bool
}

// DefaultExportOptions are the legacy export defaults: sync history yes, raw payloads no.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{IncludeSyncRuns: true, IncludeRawPayloads: false}
}

// Export writes a bundle.
//
// The settings are taken from the injected allowlist, so a credential or a private
// endpoint cannot reach the file even if it is stored in the same table.
func (s *Service) Export(ctx context.Context, options ExportOptions) ([]byte, error) {
	data, err := s.collect(ctx, options)
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	if err := bundle.Write(&buffer, data, bundle.WriteOptions{
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
		return nil, err
	}
	return buffer.Bytes(), nil
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
	for bookmarkID, list := range byBookmark {
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

	// The settings come from the allowlist, never from a raw table dump.
	if s.ExportableSettings != nil {
		if data.Settings, err = s.ExportableSettings(ctx); err != nil {
			return data, err
		}
	}

	return data, nil
}

// Preview inspects a bundle without applying it.
//
// It exists so a user can see the shape of a merge before agreeing to it, which is
// the difference between Import and Restore.
func (s *Service) Preview(ctx context.Context, raw []byte) (bundle.Preview, error) {
	result, err := s.parse(raw)
	if err != nil {
		return bundle.Preview{}, err
	}
	parsed := result.Bundle

	existing, err := s.repos.Bookmarks.AllIDs(ctx)
	if err != nil {
		return bundle.Preview{}, err
	}
	known := make(map[string]bool, len(existing))
	for _, id := range existing {
		known[id] = true
	}

	bookmarks, err := decodeRecords[BookmarkDTO](parsed.Files[bundle.FileBookmarks])
	if err != nil {
		return bundle.Preview{}, err
	}

	preview := bundle.Preview{
		Manifest: parsed.Manifest, TotalBytes: parsed.TotalBytes,
		Warnings: result.Warnings, Legacy: result.Legacy,
	}
	for _, bookmark := range bookmarks {
		if known[bookmark.TweetID] {
			preview.ExistingRecords++
		} else {
			preview.NewRecords++
		}
	}
	return preview, nil
}

// ImportReport says what a merge changed.
type ImportReport struct {
	BookmarksInserted int
	BookmarksUpdated  int
	TagsInserted      int
	ListsInserted     int
	RelationsAdded    int
	SuggestionsAdded  int
	ViewsWritten      int
	SyncRunsAdded     int
	AnalysisWritten   int
	// Warnings are the consequences the user must be told about, such as a legacy
	// backup that will cause every bookmark to be analysed again.
	Warnings []LegacyWarning
	Legacy   bool
}

// syncRunsExported caps how much history a bundle carries. A long-lived install
// accumulates thousands of runs, and a backup does not need all of them.
const syncRunsExported = 200

// Import merges a bundle into the library.
//
// Merging is idempotent: every write is an upsert or an insert-if-absent, so
// importing the same bundle twice leaves the library exactly as one import did.
// Existing data is never removed, which is what distinguishes Import from Restore.
func (s *Service) Import(ctx context.Context, raw []byte) (ImportReport, error) {
	result, err := s.parse(raw)
	if err != nil {
		return ImportReport{}, err
	}
	return s.apply(ctx, result.Bundle, false, result.Warnings...)
}

// Restore replaces the library with a bundle.
//
// This is the legacy restore semantics: the bundle becomes the library. It is
// destructive, so the caller is expected to have confirmed it — and unlike Import
// it does not merge, because merging into data the user is trying to get rid of
// would defeat the point.
func (s *Service) Restore(ctx context.Context, raw []byte) (ImportReport, error) {
	result, err := s.parse(raw)
	if err != nil {
		return ImportReport{}, err
	}

	// Snapshot before deleting anything. The apply path is not transactional --
	// it writes table by table through the repositories -- so a failure part way
	// through used to leave the library half replaced, with the original already
	// gone. The snapshot is in memory and is only kept while this call runs; the
	// guarantee is "a failed restore puts the library back", not "a crash is
	// survivable", which is what the transactional rewrite would add.
	snapshot, snapshotErr := s.Export(ctx, DefaultExportOptions())

	// Validation happens above, before anything is deleted: a rejected bundle must
	// leave the existing library untouched.
	if err := s.wipe(ctx); err != nil {
		return ImportReport{}, err
	}

	report, applyErr := s.apply(ctx, result.Bundle, true, result.Warnings...)
	if applyErr == nil {
		return report, nil
	}

	// Put the old library back. The rollback itself can fail (the disk is the
	// disk), and when it does the honest report is both failures rather than a
	// single error that hides which data is still there.
	if snapshotErr != nil {
		return report, fmt.Errorf("importexport: restore failed (%v) and no snapshot could be taken: %w", applyErr, snapshotErr)
	}
	parsedSnapshot, parseErr := s.parse(snapshot)
	if parseErr != nil {
		return report, fmt.Errorf("importexport: restore failed (%v) and its snapshot could not be read: %w", applyErr, parseErr)
	}
	if wipeErr := s.wipe(ctx); wipeErr != nil {
		return report, fmt.Errorf("importexport: restore failed (%v); rollback could not clear the partial data: %w", applyErr, wipeErr)
	}
	if _, rollbackErr := s.apply(ctx, parsedSnapshot.Bundle, true); rollbackErr != nil {
		return report, fmt.Errorf("importexport: restore failed (%v) and rollback failed: %w", applyErr, rollbackErr)
	}
	return report, fmt.Errorf("importexport: restore failed; the previous library was put back: %w", applyErr)
}

// ParseResult is a validated input, whatever format it arrived in.
type ParseResult struct {
	Bundle *bundle.Bundle
	// Warnings are consequences the user must be told about. They are not errors:
	// a legacy backup is perfectly importable, it just costs something.
	Warnings []LegacyWarning
	// Legacy is true when the input was a flat JSON backup.
	Legacy bool
}

// parse validates an input, recognising both the bundle and the legacy format.
//
// A legacy backup is converted into the bundle shape rather than given its own
// applier, so the merge rules exist in exactly one place and a legacy file cannot
// behave differently from a bundle.
func (s *Service) parse(raw []byte) (*ParseResult, error) {
	if LooksLikeLegacy(raw) {
		legacy, warnings, err := ParseLegacy(raw)
		if err != nil {
			return nil, err
		}
		converted, err := legacy.toBundle()
		if err != nil {
			return nil, err
		}
		return &ParseResult{Bundle: converted, Warnings: warnings, Legacy: true}, nil
	}

	parsed, err := bundle.Read(bytes.NewReader(raw), int64(len(raw)), bundle.DefaultLimits())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return &ParseResult{Bundle: parsed}, nil
}

// normalizeTagParents reduces an imported hierarchy to the two levels the app supports.
//
// It reads the file's original links rather than the values as it rewrites them, so the
// outcome does not depend on the order the records happen to be in. Every bad shape maps to
// a root: a self-parent, a parent that is itself a child (a third level, and also what
// breaks a cycle of any length).
func normalizeTagParents(tags []TagDTO) {
	original := make(map[string]string, len(tags))
	for _, tag := range tags {
		original[tag.ID] = tag.ParentID
	}
	for i := range tags {
		parent := original[tags[i].ID]
		if parent == "" {
			continue
		}
		if parent == tags[i].ID {
			tags[i].ParentID = ""
			continue
		}
		if grand, known := original[parent]; known && grand != "" {
			tags[i].ParentID = ""
		}
	}
}

// apply writes a validated bundle, merging or replacing.
func (s *Service) apply(ctx context.Context, parsed *bundle.Bundle, replacing bool, warnings ...LegacyWarning) (ImportReport, error) {
	report := ImportReport{Warnings: warnings, Legacy: len(warnings) > 0}
	now := s.clock.Now()

	// Tags first: relations reference them, and a relation to a tag that does not
	// exist yet would be rejected by the foreign key.
	tags, err := decodeRecords[TagDTO](parsed.Files[bundle.FileTags])
	if err != nil {
		return report, fmt.Errorf("importexport: decode tags: %w", err)
	}
	// A bundle is untrusted input, and the hierarchy is capped at two levels. Only this
	// app writes bundles, but a hand-edited or third-party one could carry a self-parent,
	// a cycle or a third level; the legacy reader rejects all three when it reads a backup,
	// and importing one here produced a library whose parent
	// picker was locked or whose rows the store then refused. Reducing the bad edge to a
	// root is what the loop below already does for a parent outside the bundle.
	normalizeTagParents(tags)

	// Parents first. The schema's trigger refuses a child whose parent does not
	// exist yet, and the export orders tags by name -- so "Go" under
	// "Programming" was written first, the trigger aborted, and a restore had
	// already wiped the library by then.
	sort.SliceStable(tags, func(i, j int) bool {
		return tags[i].ParentID == "" && tags[j].ParentID != ""
	})

	// A bundle tag may share its name with a local tag that has another id.
	// canonical_key is UNIQUE, so the insert would fail; the map lets relations
	// and child tags that point at the bundle's id find the local row instead.
	tagIDMap := make(map[string]string, len(tags))
	for _, dto := range tags {
		tag := dto.ToDomain()
		if parent, ok := tagIDMap[tag.ParentID]; ok {
			tag.ParentID = parent
		} else if tag.ParentID != "" {
			// The parent was not in the bundle (or could not be mapped), and a
			// dangling id would trip the same trigger. A root is the safe shape.
			tag.ParentID = ""
		}

		existing, found, err := s.repos.Tags.Get(ctx, tag.ID)
		if err != nil {
			return report, err
		}
		if found {
			tagIDMap[dto.ID] = existing.ID
			if !replacing {
				// Merging keeps what is already there: the local tag may carry
				// aliases an older bundle does not know about.
				continue
			}
			tag.CreatedAt = existing.CreatedAt
			if err := s.repos.Tags.Update(ctx, tag); err != nil {
				return report, err
			}
			continue
		}
		if sameName, ok, err := s.repos.Tags.FindByName(ctx, tag.Name); err != nil {
			return report, err
		} else if ok {
			tagIDMap[dto.ID] = sameName.ID
			continue
		}
		if err := s.repos.Tags.Create(ctx, tag); err != nil {
			return report, err
		}
		tagIDMap[dto.ID] = tag.ID
		report.TagsInserted++
	}

	// Lists, with the Inbox always present.
	inbox := domain.List{
		ID: domain.InboxListID, Name: domain.InboxListName,
		Normalized: domain.NormalizeListName(domain.InboxListName),
		CreatedAt:  now, UpdatedAt: now,
	}
	if _, found, err := s.repos.Lists.Get(ctx, inbox.ID); err != nil {
		return report, err
	} else if !found {
		if err := s.repos.Lists.Create(ctx, inbox); err != nil {
			return report, err
		}
		report.ListsInserted++
	}

	lists, err := decodeRecords[ListDTO](parsed.Files[bundle.FileLists])
	if err != nil {
		return report, fmt.Errorf("importexport: decode lists: %w", err)
	}
	for _, dto := range lists {
		list := dto.ToDomain()
		if list.ID == domain.InboxListID {
			continue
		}
		if _, found, err := s.repos.Lists.Get(ctx, list.ID); err != nil {
			return report, err
		} else if found {
			continue
		}
		if err := s.repos.Lists.Create(ctx, list); err != nil {
			return report, err
		}
		report.ListsInserted++
	}

	// Bookmarks.
	bookmarks, err := decodeRecords[BookmarkDTO](parsed.Files[bundle.FileBookmarks])
	if err != nil {
		return report, fmt.Errorf("importexport: decode bookmarks: %w", err)
	}
	if len(bookmarks) > 0 {
		ids := make([]string, 0, len(bookmarks))
		for _, dto := range bookmarks {
			ids = append(ids, dto.TweetID)
		}
		// The file is authoritative for the ids it carries, so an explicit import
		// revives them; a restore has already cleared every tombstone in wipe.
		if err := s.repos.Bookmarks.ClearTombstones(ctx, ids); err != nil {
			return report, err
		}

		domainBookmarks := make([]domain.Bookmark, 0, len(bookmarks))
		for _, dto := range bookmarks {
			domainBookmarks = append(domainBookmarks, dto.ToDomain())
		}
		upserted, err := s.repos.Bookmarks.UpsertMany(ctx, domainBookmarks)
		if err != nil {
			return report, err
		}
		report.BookmarksInserted = upserted.Inserted
		report.BookmarksUpdated = upserted.Updated
	}

	// Relations.
	relations, err := decodeRecords[BookmarkTagDTO](parsed.Files[bundle.FileBookmarkTags])
	if err != nil {
		return report, fmt.Errorf("importexport: decode bookmark tags: %w", err)
	}
	// The existing set, read once. Attach leaves an existing relation alone, so counting
	// every row in the file reported all 4,650 of them as "added" on a second import of a
	// library where nothing changed -- the merge was idempotent and the report said otherwise.
	// The map also skips the no-op writes.
	existing := make(map[[2]string]bool)
	currentRelations, err := s.repos.Tags.AllRelations(ctx)
	if err != nil {
		return report, err
	}
	for _, relation := range currentRelations {
		existing[[2]string{relation.BookmarkID, relation.TagID}] = true
	}
	// The bookmarks a relation may point at: this file's, plus whatever the library
	// already held when merging. A dangling reference used to reach the foreign key,
	// which aborted the import after the tags and bookmarks had been written -- a merge
	// with no rollback. The legacy reader rejects such a file before restoring anything.
	knownBookmarks := make(map[string]bool, len(bookmarks))
	for _, dto := range bookmarks {
		knownBookmarks[dto.TweetID] = true
	}
	if !replacing {
		all, err := s.repos.Bookmarks.All(ctx)
		if err != nil {
			return report, err
		}
		for _, bookmark := range all {
			knownBookmarks[bookmark.TweetID] = true
		}
	}
	for _, dto := range relations {
		tagID := dto.TagID
		if mapped, ok := tagIDMap[tagID]; ok {
			tagID = mapped
		} else {
			// A relation to a tag the bundle did not carry cannot be attached:
			// the foreign key would reject it and abort the whole import.
			continue
		}
		if !knownBookmarks[dto.BookmarkID] {
			continue
		}
		key := [2]string{dto.BookmarkID, tagID}
		if existing[key] {
			continue
		}
		// An unrecognised source is recorded as manual, which is the safe
		// direction: a manual tag is never removed by an automatic prune.
		source := dto.Source
		if !domain.ValidTagSource(source) {
			source = domain.TagSourceManual
		}
		if err := s.repos.Tags.Attach(ctx, domain.BookmarkTag{
			BookmarkID: dto.BookmarkID, TagID: tagID,
			CreatedAt: domain.MustParseTimestamp(dto.CreatedAt), Source: source,
		}); err != nil {
			return report, err
		}
		existing[key] = true
		report.RelationsAdded++
	}

	// Memberships. A bookmark keeps the list it already has when merging, so an
	// import cannot silently move a user's bookmarks.
	memberships, err := decodeRecords[BookmarkListDTO](parsed.Files[bundle.FileBookmarkLists])
	if err != nil {
		return report, fmt.Errorf("importexport: decode bookmark lists: %w", err)
	}
	if len(memberships) > 0 {
		ids := make([]string, 0, len(memberships))
		for _, dto := range memberships {
			ids = append(ids, dto.BookmarkID)
		}
		current, err := s.repos.Bookmarks.ListByBookmark(ctx, ids)
		if err != nil {
			return report, err
		}
		for _, dto := range memberships {
			if _, present := current[dto.BookmarkID]; present && !replacing {
				continue
			}
			if err := s.repos.Bookmarks.AssignToList(ctx, []string{dto.BookmarkID}, dto.ListID, now); err != nil {
				return report, err
			}
		}
	}

	// Every bookmark without a list belongs to the Inbox, or it would be invisible
	// in every list view.
	if _, err := s.repos.Bookmarks.AssignMissingToInbox(ctx, now); err != nil {
		return report, err
	}

	// Suggestions. Resolved rows are the negative cache, so a merge must not lose
	// them; an insert-if-absent is exactly right for both cases.
	suggestions, err := decodeRecords[SuggestionDTO](parsed.Files[bundle.FileTagSuggestions])
	if err != nil {
		return report, fmt.Errorf("importexport: decode suggestions: %w", err)
	}
	if len(suggestions) > 0 {
		domainSuggestions := make([]domain.TagSuggestion, 0, len(suggestions))
		for _, dto := range suggestions {
			if !domain.ValidSuggestionSource(dto.Source) || !domain.ValidSuggestionStatus(dto.Status) {
				// A row this build does not understand is skipped rather than
				// guessed at: the status drives whether it is offered again.
				continue
			}
			domainSuggestions = append(domainSuggestions, dto.ToDomain())
		}
		inserted, err := s.repos.Suggestions.InsertMany(ctx, domainSuggestions)
		if err != nil {
			return report, err
		}
		report.SuggestionsAdded = inserted
	}

	// Saved views.
	views, err := decodeRecords[SavedViewDTO](parsed.Files[bundle.FileSavedViews])
	if err != nil {
		return report, fmt.Errorf("importexport: decode saved views: %w", err)
	}
	for _, dto := range views {
		view := dto.ToDomain()
		if !domain.ValidSortOrder(view.SortOrder) || !domain.ValidViewMode(view.ViewMode) {
			// A view whose sort order this build does not know is skipped: the
			// schema would reject it anyway, and defaulting would silently change
			// what the user saved.
			continue
		}
		if err := s.repos.SavedViews.Upsert(ctx, view); err != nil {
			return report, err
		}
		report.ViewsWritten++
	}

	// Sync history, when the bundle carries it.
	runs, err := decodeRecords[SyncRunDTO](parsed.Files[bundle.FileSyncRuns])
	if err != nil {
		return report, fmt.Errorf("importexport: decode sync runs: %w", err)
	}
	if s.repos.SyncRuns != nil {
		for _, dto := range runs {
			run := dto.ToDomain()
			if _, found, err := s.repos.SyncRuns.Get(ctx, run.ID); err != nil {
				return report, err
			} else if found {
				continue
			}
			if err := s.repos.SyncRuns.Start(ctx, run); err != nil {
				return report, err
			}
			if err := s.repos.SyncRuns.Finish(ctx, run); err != nil {
				return report, err
			}
			report.SyncRunsAdded++
		}
	}

	// The verdict ledger, so an imported library is not re-analysed from scratch
	// and does not spend the user's quota again.
	ledger, err := decodeRecords[AnalysisDTO](parsed.Files[bundle.FileLLMAnalysis])
	if err != nil {
		return report, fmt.Errorf("importexport: decode analysis: %w", err)
	}
	if len(ledger) > 0 && s.repos.Analysis != nil {
		records := make([]domain.LlmAnalysis, 0, len(ledger))
		for _, dto := range ledger {
			records = append(records, dto.ToDomain())
		}
		if _, err := s.repos.Analysis.UpsertMany(ctx, records); err != nil {
			return report, err
		}
		report.AnalysisWritten = len(records)
	}

	// Settings, through the injected applier so the allowlist is enforced in one
	// place.
	if s.RestoreSettings != nil {
		if raw, present := parsed.Files[bundle.FileSettings]; present {
			var settings map[string]json.RawMessage
			if err := json.Unmarshal(raw, &settings); err == nil {
				if err := s.RestoreSettings(ctx, settings); err != nil {
					return report, err
				}
			}
		}
	}

	return report, nil
}

// clearSecrets is installed by the composition root, which holds both this service and
// the credential store. It is nil in tests that do not care, and the reset still runs.
//
// UseSecretClearer installs it.
//
// This action belongs beside "Import restore", behind a confirmation. It was missing here
// in a way nothing caught: the cascade existed, but only as the first half of a restore,
// and the checker that finds unreached methods walks internal/usecase -- this is
// internal/importexport, so it was outside that sweep.
func (s *Service) ResetLocalData(ctx context.Context) error {
	if err := s.require(); err != nil {
		return err
	}

	// The stored secrets go first, and a failure stops the reset before anything is
	// wiped. The confirmation promises that local settings are cleared, and it would be
	// false if the Keychain accounts survived: the API key lives under its own account,
	// so emptying the tables would never reach it. The secrets store has to be cleared as
	// well for the promise to hold.
	//
	// The clearer is injected, not reached for: the accounts belong to the credentials
	// package, and importing it here would tie this service to the Keychain for one
	// call. A nil clearer means there is nothing to clear, as in the tests that build
	// this service without one.
	if s.clearSecrets != nil {
		if err := s.clearSecrets(ctx); err != nil {
			return err
		}
	}

	// The confirmation names bookmarks, tags and settings, so all three go.
	if err := s.wipe(ctx); err != nil {
		return err
	}
	if _, err := s.repos.Tags.DeleteAll(ctx); err != nil {
		return err
	}
	if _, err := s.repos.Settings.DeleteAll(ctx); err != nil {
		return err
	}
	return nil
}

// wipe removes the library's data before a restore.
//
// The order matters: relations and memberships cascade from bookmarks, and a tag
// still referenced by a relation cannot be removed first.
func (s *Service) wipe(ctx context.Context) error {
	if _, err := s.repos.Bookmarks.DeleteAll(ctx); err != nil {
		return err
	}
	if _, err := s.repos.Suggestions.DeleteAll(ctx); err != nil {
		return err
	}
	if _, err := s.repos.Analysis.DeleteAll(ctx); err != nil {
		return err
	}

	tags, err := s.repos.Tags.List(ctx)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(tags))
	for _, tag := range tags {
		ids = append(ids, tag.ID)
	}
	if _, err := s.repos.Tags.DeleteMany(ctx, ids); err != nil {
		return err
	}

	// Lists are removed except the Inbox, which is a system row the schema and the
	// UI both assume exists.
	lists, err := s.repos.Lists.List(ctx)
	if err != nil {
		return err
	}
	for _, list := range lists {
		if list.ID == domain.InboxListID {
			continue
		}
		if err := s.repos.Lists.Delete(ctx, list.ID); err != nil {
			return err
		}
	}

	// Saved views belong to the library, not to the app: the bundle is the new
	// library, and a view the bundle does not contain must not survive it. The
	// repository has no DeleteAll, and the count is small.
	if s.repos.SavedViews != nil {
		views, err := s.repos.SavedViews.List(ctx)
		if err != nil {
			return err
		}
		for _, view := range views {
			if err := s.repos.SavedViews.Delete(ctx, view.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ErrUnsupportedBundle reports a file whose shape this build cannot read.
var ErrUnsupportedBundle = errors.New("importexport: the bundle holds data this build cannot read")

// Clock returns the service's clock reading, so a caller stamping alongside it
// uses one clock.
func (s *Service) Now() time.Time { return s.clock.Now() }

// UseSecretClearer installs the hook that forgets stored secrets.
func (s *Service) UseSecretClearer(clear func(context.Context) error) { s.clearSecrets = clear }
