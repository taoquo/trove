package importexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/taoquo/trove/internal/bundle"
	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/ports"
	"sort"
	"time"
)

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

// ErrValidation reports a bundle that failed a check. The wrapped error names the
// step, so a caller can tell corruption from a version mismatch.
var ErrValidation = errors.New("importexport: the bundle was rejected")

// ErrUnsupportedBundle reports a file whose shape this build cannot read.
var ErrUnsupportedBundle = errors.New("importexport: the bundle holds data this build cannot read")

// Clock returns the service's clock reading, so a caller stamping alongside it
// uses one clock.
func (s *Service) Now() time.Time { return s.clock.Now() }

// UseSecretClearer installs the hook that forgets stored secrets.
func (s *Service) UseSecretClearer(clear func(context.Context) error) { s.clearSecrets = clear }

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
	BookmarksInserted  int
	BookmarksUpdated   int
	TagsInserted       int
	ListsInserted      int
	RelationsAdded     int
	SuggestionsAdded   int
	ViewsWritten       int
	SyncRunsAdded      int
	AnalysisWritten    int
	RawPayloadsWritten int
	TombstonesWritten  int
	// Warnings are the consequences the user must be told about, such as a legacy
	// backup that will cause every bookmark to be analysed again.
	Warnings []LegacyWarning
	Legacy   bool
}

// syncRunsExported caps how much history a bundle carries. A long-lived install
// accumulates thousands of runs, and a backup does not need all of them.
const syncRunsExported = 200

// importBatch is how many rows one transaction of an import writes.
//
// The relations and the memberships used to be written one row at a time, each its
// own transaction: a hundred thousand relations meant a hundred thousand commits.
// A thousand rows per commit is small enough that a failure loses little and large
// enough that the commit count stays in the hundreds.
const importBatch = 1000

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

// ParseResult is a validated input, whatever format it arrived in.
type ParseResult struct {
	Bundle *bundle.Bundle
	// Warnings are consequences the user must be told about. They are not errors:
	// a legacy backup is perfectly importable, it just costs something.
	Warnings []LegacyWarning
	// Legacy is true when the input was a flat JSON backup.
	Legacy bool
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
	pendingRelations := make([]domain.BookmarkTag, 0, len(relations))
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
		pendingRelations = append(pendingRelations, domain.BookmarkTag{
			BookmarkID: dto.BookmarkID, TagID: tagID,
			CreatedAt: domain.MustParseTimestamp(dto.CreatedAt), Source: source,
		})
		existing[key] = true
		report.RelationsAdded++
	}
	// In batches, not one attach at a time. Every Attach was its own transaction,
	// so importing a library with fifty thousand relations committed fifty thousand
	// times -- minutes of fsync for a file that was already in memory.
	for start := 0; start < len(pendingRelations); start += importBatch {
		end := min(start+importBatch, len(pendingRelations))
		if err := s.repos.Tags.AttachMany(ctx, pendingRelations[start:end]); err != nil {
			return report, err
		}
	}

	// Tombstones. The file records what the exporting device deleted, and a restore
	// must keep that record: wipe has just cleared the local one, and a sync would
	// otherwise bring every deleted post back. An id the file itself carries is not a
	// delete, and a merge must not tombstone a post this library still holds.
	tombstones, err := decodeRecords[TombstoneDTO](parsed.Files[bundle.FileTombstones])
	if err != nil {
		return report, fmt.Errorf("importexport: decode tombstones: %w", err)
	}
	if len(tombstones) > 0 {
		records := make([]ports.Tombstone, 0, len(tombstones))
		for _, dto := range tombstones {
			if dto.TweetID == "" || knownBookmarks[dto.TweetID] {
				continue
			}
			records = append(records, dto.ToDomain())
		}
		if err := s.repos.Bookmarks.AddTombstones(ctx, records); err != nil {
			return report, err
		}
		report.TombstonesWritten = len(records)
	}

	// Raw payloads, after the bookmarks they belong to: the foreign key would
	// reject one that arrives first. A capture only exists when the export asked
	// for it, so an absent file is the normal case rather than a loss.
	payloads, err := decodeRecords[RawPayloadDTO](parsed.Files[bundle.FileRawPayloads])
	if err != nil {
		return report, fmt.Errorf("importexport: decode raw payloads: %w", err)
	}
	if len(payloads) > 0 {
		stored := make([]ports.RawPayload, 0, len(payloads))
		for _, dto := range payloads {
			// The same rule as relations: a payload for a bookmark neither the file
			// nor the library holds cannot be written, and skipping it keeps one bad
			// row from aborting a restore.
			if !knownBookmarks[dto.TweetID] {
				continue
			}
			stored = append(stored, dto.ToDomain())
		}
		if err := s.repos.Bookmarks.SaveRawPayloads(ctx, stored); err != nil {
			return report, err
		}
		report.RawPayloadsWritten = len(stored)
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
		// Grouped by list, because the repository assigns a list to a set of
		// bookmarks: one row at a time made the import commit per bookmark. The
		// list order is the file's, so the same file commits in the same order.
		byList := make(map[string][]string, 8)
		var listOrder []string
		for _, dto := range memberships {
			if _, present := current[dto.BookmarkID]; present && !replacing {
				continue
			}
			if _, seen := byList[dto.ListID]; !seen {
				listOrder = append(listOrder, dto.ListID)
			}
			byList[dto.ListID] = append(byList[dto.ListID], dto.BookmarkID)
		}
		for _, listID := range listOrder {
			bookmarkIDs := byList[listID]
			for start := 0; start < len(bookmarkIDs); start += importBatch {
				end := min(start+importBatch, len(bookmarkIDs))
				if err := s.repos.Bookmarks.AssignToList(ctx, bookmarkIDs[start:end], listID, now); err != nil {
					return report, err
				}
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
