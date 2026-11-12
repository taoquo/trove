package importexport

import (
	"context"
	"encoding/json"
	"time"
)

// extensionExportPayload is the extension's flat JSON backup.
//
// The native export is a bundle, and a user who moves back to the extension could not read
// it: the extension only knows this shape (src/lib/export/exportBookmarks.ts:26-35), and it
// validates every record before restoring anything (src/lib/export/workspaceBackup.ts:344).
// Writing a file the extension rejects would make the button decorative, so this file's
// job is to meet that reader's contract, not merely to be valid JSON:
//
//   - settings.locale and settings.themePreference are required, not optional;
//   - summary must be a SyncSummary with a known status and four counts;
//   - every bookmarkTag needs a unique id (the extension's own is "<bookmarkId>:<tagId>");
//   - a saved view's activeTagIds and activeAuthorHandles must be arrays, empty or not;
//   - metrics, when present, needs all three counts as numbers.
//
// The records are written as the raw JSON the DTO layer already produced, so the field
// names are the extension's own rather than a second mapping.
type extensionExportPayload struct {
	ExportVersion int               `json:"exportVersion"`
	SchemaVersion int               `json:"schemaVersion"`
	ExportedAt    string            `json:"exportedAt"`
	Bookmarks     []json.RawMessage `json:"bookmarks"`
	Tags          []json.RawMessage `json:"tags"`
	BookmarkTags  []json.RawMessage `json:"bookmarkTags"`
	Settings      extensionSettings `json:"settings"`
	Summary       extensionSummary  `json:"summary"`
	LatestSyncRun *extensionSyncRun `json:"latestSyncRun"`
}

// extensionSettings is the settings block the extension writes and validates.
//
// Only what the native app actually has is emitted: the endpoint and every credential stay
// behind, which is the same boundary Export keeps. The provider id and model are optional
// in the extension's validator and are not carried, because the native app stores them in
// the auto-tag blob beside a private endpoint.
type extensionSettings struct {
	SchemaVersion   int               `json:"schemaVersion"`
	Locale          string            `json:"locale"`
	ThemePreference string            `json:"themePreference"`
	SavedViews      []json.RawMessage `json:"savedViews,omitempty"`
	LastSyncSummary *extensionSummary `json:"lastSyncSummary,omitempty"`
}

// extensionSummary is the extension's SyncSummary (src/lib/types.ts). The status vocabulary
// has no "cancelled", which the native app records, so that one is written as an error.
type extensionSummary struct {
	Status        string `json:"status"`
	FetchedCount  int    `json:"fetchedCount"`
	InsertedCount int    `json:"insertedCount"`
	UpdatedCount  int    `json:"updatedCount"`
	FailedCount   int    `json:"failedCount"`
	LastSyncedAt  string `json:"lastSyncedAt,omitempty"`
	ErrorKind     string `json:"errorKind,omitempty"`
	ErrorSummary  string `json:"errorSummary,omitempty"`
}

// extensionSyncRun is the extension's SyncRunRecord.
type extensionSyncRun struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	StartedAt     string `json:"startedAt,omitempty"`
	FinishedAt    string `json:"finishedAt,omitempty"`
	FetchedCount  int    `json:"fetchedCount"`
	InsertedCount int    `json:"insertedCount"`
	UpdatedCount  int    `json:"updatedCount"`
	FailedCount   int    `json:"failedCount"`
	ErrorKind     string `json:"errorKind,omitempty"`
	ErrorSummary  string `json:"errorSummary,omitempty"`
}

// extensionBookmarkTag is one relation with the id the extension's store expects.
type extensionBookmarkTag struct {
	ID         string `json:"id"`
	BookmarkID string `json:"bookmarkId"`
	TagID      string `json:"tagId"`
	CreatedAt  string `json:"createdAt"`
}

// extensionSavedView is a saved view with the arrays the validator requires.
type extensionSavedView struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Query               string   `json:"query"`
	ActiveTagIDs        []string `json:"activeTagIds"`
	ActiveAuthorHandles []string `json:"activeAuthorHandles"`
	OnlyWithMedia       bool     `json:"onlyWithMedia"`
	OnlyLongform        bool     `json:"onlyLongform"`
	SortOrder           string   `json:"sortOrder"`
	ViewMode            string   `json:"viewMode"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

// extensionErrorKinds maps the native vocabulary onto the extension's six kinds
// (src/lib/x/syncErrors.ts:1-10). The finer native kinds did not exist there, and an
// unknown string makes the extension refuse the whole backup, so nothing is passed through
// unmapped.
var extensionErrorKinds = map[string]string{
	"auth_expired":            "auth_expired",
	"unauthorized":            "auth_expired",
	"csrf_invalid":            "auth_expired",
	"rate_limited":            "rate_limited",
	"network_error":           "network_error",
	"network":                 "network_error",
	"x_schema_changed":        "x_schema_changed",
	"response_schema_changed": "x_schema_changed",
	"query_id_stale":          "x_schema_changed",
	"interrupted":             "interrupted",
	"unknown":                 "unknown",
}

// extensionStatus maps a native sync status onto the extension's vocabulary.
func extensionStatus(status string) string {
	switch status {
	case "idle", "running", "success", "partial_success", "error":
		return status
	default:
		// "cancelled" is the one native status the extension has no word for. The run did
		// not complete, so saying "error" is closer than claiming success or idleness.
		return "error"
	}
}

func extensionErrorKind(kind string) string { return extensionErrorKinds[kind] }

// extensionRecords re-renders each DTO so the extension's required fields are present.
func extensionRecords[T any, R any](records []json.RawMessage, convert func(T) (R, error)) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(records))
	for _, raw := range records {
		var record T
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		converted, err := convert(record)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(converted)
		if err != nil {
			return nil, err
		}
		out = append(out, encoded)
	}
	return out, nil
}

// extensionBookmarks fills the counts a missing metrics field would otherwise omit.
//
// The native bundle deliberately keeps an absent count absent (dto.go, MetricsDTO), but the
// extension requires all three numbers whenever a metrics object is present. The zero is
// written here rather than in the DTO so the bundle keeps its meaning.
func extensionBookmarks(records []json.RawMessage) ([]json.RawMessage, error) {
	return extensionRecords(records, func(dto BookmarkDTO) (BookmarkDTO, error) {
		if dto.Metrics != nil {
			zero := 0
			dto.Metrics = &MetricsDTO{
				Likes:    firstInt(dto.Metrics.Likes, &zero),
				Retweets: firstInt(dto.Metrics.Retweets, &zero),
				Replies:  firstInt(dto.Metrics.Replies, &zero),
			}
		}
		return dto, nil
	})
}

func firstInt(value, fallback *int) *int {
	if value != nil {
		return value
	}
	return fallback
}

// extensionBookmarkTags adds the id the extension's store keys relations by.
func extensionBookmarkTags(records []json.RawMessage) ([]json.RawMessage, error) {
	return extensionRecords(records, func(dto BookmarkTagDTO) (extensionBookmarkTag, error) {
		return extensionBookmarkTag{
			// src/lib/storage/tagsStore.ts:13-15, the same id the extension creates.
			ID:         dto.BookmarkID + ":" + dto.TagID,
			BookmarkID: dto.BookmarkID,
			TagID:      dto.TagID,
			CreatedAt:  dto.CreatedAt,
		}, nil
	})
}

// extensionSavedViews makes the two arrays unconditional.
func extensionSavedViews(records []json.RawMessage) ([]json.RawMessage, error) {
	return extensionRecords(records, func(dto SavedViewDTO) (extensionSavedView, error) {
		return extensionSavedView{
			ID:                  dto.ID,
			Name:                dto.Name,
			Query:               dto.Query,
			ActiveTagIDs:        nonNilStrings(dto.ActiveTagIDs),
			ActiveAuthorHandles: nonNilStrings(dto.ActiveAuthorHandles),
			OnlyWithMedia:       dto.OnlyWithMedia,
			OnlyLongform:        dto.OnlyLongform,
			SortOrder:           dto.SortOrder,
			ViewMode:            dto.ViewMode,
			CreatedAt:           dto.CreatedAt,
			UpdatedAt:           dto.UpdatedAt,
		}, nil
	})
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// ExportExtensionJSON writes the library in the extension's own backup format.
//
// The extension is the reference implementation and the format it reads is also the one it
// writes; a native backup it cannot read makes the native app a one-way door.
func (s *Service) ExportExtensionJSON(ctx context.Context) ([]byte, error) {
	data, err := s.collect(ctx, DefaultExportOptions())
	if err != nil {
		return nil, err
	}

	bookmarks, err := extensionBookmarks(data.Bookmarks)
	if err != nil {
		return nil, err
	}
	bookmarkTags, err := extensionBookmarkTags(data.BookmarkTags)
	if err != nil {
		return nil, err
	}
	views, err := extensionSavedViews(data.SavedViews)
	if err != nil {
		return nil, err
	}

	settings := extensionSettings{SchemaVersion: 7, Locale: "en", ThemePreference: "system", SavedViews: views}
	if s.ExportableSettings != nil {
		if raw, err := s.ExportableSettings(ctx); err == nil {
			// The settings are stored as one preferences blob, not one key per field.
			for _, value := range raw {
				var preferences struct {
					Locale     string `json:"locale"`
					Appearance string `json:"appearance"`
				}
				if json.Unmarshal(value, &preferences) != nil {
					continue
				}
				// The extension accepts only its two locales, and the native app stores an
				// empty locale for "follow the system"; "en" is the honest fallback because
				// the extension has no third choice.
				if preferences.Locale == "zh-CN" {
					settings.Locale = "zh-CN"
				}
				switch preferences.Appearance {
				case "light", "dark":
					settings.ThemePreference = preferences.Appearance
				}
				break
			}
		}
	}

	summary, latest := extensionLastRun(data.SyncRuns)
	settings.LastSyncSummary = &summary

	payload := extensionExportPayload{
		ExportVersion: 2,
		SchemaVersion: 7,
		ExportedAt:    s.clock.Now().UTC().Format(time.RFC3339Nano),
		Bookmarks:     bookmarks,
		Tags:          data.Tags,
		BookmarkTags:  bookmarkTags,
		Settings:      settings,
		Summary:       summary,
		LatestSyncRun: latest,
	}
	if payload.Bookmarks == nil {
		payload.Bookmarks = []json.RawMessage{}
	}
	if payload.Tags == nil {
		payload.Tags = []json.RawMessage{}
	}
	if payload.BookmarkTags == nil {
		payload.BookmarkTags = []json.RawMessage{}
	}

	// Indented like the extension's JSON.stringify(..., null, 2), so a diff of two backups
	// reads the same whichever app wrote them.
	return json.MarshalIndent(payload, "", "  ")
}

// extensionLastRun renders the newest recorded run as both the summary and the run record.
//
// The extension restores lastSyncSummary from payload.summary, so a truthful run is better
// than an idle placeholder: a user returning to the extension should not be told they have
// never synced. With no run at all, the idle summary with zero counts is the honest value.
func extensionLastRun(runs []json.RawMessage) (extensionSummary, *extensionSyncRun) {
	idle := extensionSummary{Status: "idle"}
	if len(runs) == 0 {
		return idle, nil
	}

	var dto SyncRunDTO
	if err := json.Unmarshal(runs[0], &dto); err != nil {
		return idle, nil
	}

	finished := dto.FinishedAt
	if finished == "" {
		finished = dto.StartedAt
	}
	kind := extensionErrorKind(dto.ErrorKind)
	summary := extensionSummary{
		Status:        extensionStatus(dto.Status),
		FetchedCount:  dto.FetchedCount,
		InsertedCount: dto.InsertedCount,
		UpdatedCount:  dto.UpdatedCount,
		FailedCount:   dto.FailedCount,
		LastSyncedAt:  finished,
		ErrorKind:     kind,
		ErrorSummary:  dto.ErrorSummary,
	}
	run := &extensionSyncRun{
		ID:            dto.ID,
		Status:        summary.Status,
		StartedAt:     dto.StartedAt,
		FinishedAt:    dto.FinishedAt,
		FetchedCount:  dto.FetchedCount,
		InsertedCount: dto.InsertedCount,
		UpdatedCount:  dto.UpdatedCount,
		FailedCount:   dto.FailedCount,
		ErrorKind:     kind,
		ErrorSummary:  dto.ErrorSummary,
	}
	return summary, run
}
