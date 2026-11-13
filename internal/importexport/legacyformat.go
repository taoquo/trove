package importexport

import (
	"context"
	"encoding/json"
	"time"
)

// legacyExportPayload is the flat JSON backup used by the legacy exportVersion 2 format.
//
// The native export is a bundle, which a reader of the legacy format could not read: that
// format only knows this flat shape, and it validates every record before restoring
// anything. Writing a file the legacy reader rejects would make the button decorative, so
// this file's job is to meet that reader's contract, not merely to be valid JSON:
//
//   - settings.locale and settings.themePreference are required, not optional;
//   - summary must be a SyncSummary with a known status and four counts;
//   - every bookmarkTag needs a unique id ("<bookmarkId>:<tagId>");
//   - a saved view's activeTagIds and activeAuthorHandles must be arrays, empty or not;
//   - metrics, when present, needs all three counts as numbers.
//
// The records are written as the raw JSON the DTO layer already produced, so the field
// names are the format's own rather than a second mapping.
type legacyExportPayload struct {
	ExportVersion int               `json:"exportVersion"`
	SchemaVersion int               `json:"schemaVersion"`
	ExportedAt    string            `json:"exportedAt"`
	Bookmarks     []json.RawMessage `json:"bookmarks"`
	Tags          []json.RawMessage `json:"tags"`
	BookmarkTags  []json.RawMessage `json:"bookmarkTags"`
	Settings      legacySettings    `json:"settings"`
	Summary       legacySummary     `json:"summary"`
	LatestSyncRun *legacySyncRun    `json:"latestSyncRun"`
}

// legacySettings is the settings block the legacy format writes and validates.
//
// Only what the native app actually has is emitted: the endpoint and every credential stay
// behind, which is the same boundary Export keeps. The provider id and model are optional
// in the legacy validator and are not carried, because the native app stores them in the
// auto-tag blob beside a private endpoint.
type legacySettings struct {
	SchemaVersion   int               `json:"schemaVersion"`
	Locale          string            `json:"locale"`
	ThemePreference string            `json:"themePreference"`
	SavedViews      []json.RawMessage `json:"savedViews,omitempty"`
	LastSyncSummary *legacySummary    `json:"lastSyncSummary,omitempty"`
}

// legacySummary is the legacy SyncSummary. The status vocabulary has no "cancelled",
// which the native app records, so that one is written as an error.
type legacySummary struct {
	Status        string `json:"status"`
	FetchedCount  int    `json:"fetchedCount"`
	InsertedCount int    `json:"insertedCount"`
	UpdatedCount  int    `json:"updatedCount"`
	FailedCount   int    `json:"failedCount"`
	LastSyncedAt  string `json:"lastSyncedAt,omitempty"`
	ErrorKind     string `json:"errorKind,omitempty"`
	ErrorSummary  string `json:"errorSummary,omitempty"`
}

// legacySyncRun is the legacy SyncRunRecord.
type legacySyncRun struct {
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

// legacyBookmarkTag is one relation with the id the legacy store expects.
type legacyBookmarkTag struct {
	ID         string `json:"id"`
	BookmarkID string `json:"bookmarkId"`
	TagID      string `json:"tagId"`
	CreatedAt  string `json:"createdAt"`
}

// legacySavedView is a saved view with the arrays the validator requires.
type legacySavedView struct {
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

// legacyErrorKinds maps the native vocabulary onto the legacy format's six kinds. The finer
// native kinds are not part of that vocabulary, and an unknown string makes the legacy
// reader refuse the whole backup, so nothing is passed through unmapped.
var legacyErrorKinds = map[string]string{
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

// legacyStatus maps a native sync status onto the legacy vocabulary.
func legacyStatus(status string) string {
	switch status {
	case "idle", "running", "success", "partial_success", "error":
		return status
	default:
		// "cancelled" is the one native status the legacy vocabulary has no word for. The run did
		// not complete, so saying "error" is closer than claiming success or idleness.
		return "error"
	}
}

func legacyErrorKind(kind string) string { return legacyErrorKinds[kind] }

// legacyRecords re-renders each DTO so the legacy format's required fields are present.
func legacyRecords[T any, R any](records []json.RawMessage, convert func(T) (R, error)) ([]json.RawMessage, error) {
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

// legacyBookmarks fills the counts a missing metrics field would otherwise omit.
//
// The native bundle deliberately keeps an absent count absent (dto.go, MetricsDTO), but the
// legacy format requires all three numbers whenever a metrics object is present. The zero is
// written here rather than in the DTO so the bundle keeps its meaning.
func legacyBookmarks(records []json.RawMessage) ([]json.RawMessage, error) {
	return legacyRecords(records, func(dto BookmarkDTO) (BookmarkDTO, error) {
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

// legacyBookmarkTags adds the id the legacy store keys relations by.
func legacyBookmarkTags(records []json.RawMessage) ([]json.RawMessage, error) {
	return legacyRecords(records, func(dto BookmarkTagDTO) (legacyBookmarkTag, error) {
		return legacyBookmarkTag{
			// The same relation id the legacy store creates.
			ID:         dto.BookmarkID + ":" + dto.TagID,
			BookmarkID: dto.BookmarkID,
			TagID:      dto.TagID,
			CreatedAt:  dto.CreatedAt,
		}, nil
	})
}

// legacySavedViews makes the two arrays unconditional.
func legacySavedViews(records []json.RawMessage) ([]json.RawMessage, error) {
	return legacyRecords(records, func(dto SavedViewDTO) (legacySavedView, error) {
		return legacySavedView{
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

// ExportLegacyJSON writes the library in the legacy exportVersion 2 format.
//
// That format reads and writes the same shape; a native backup it cannot read makes the
// native app a one-way door.
func (s *Service) ExportLegacyJSON(ctx context.Context) ([]byte, error) {
	data, err := s.collect(ctx, DefaultExportOptions())
	if err != nil {
		return nil, err
	}

	bookmarks, err := legacyBookmarks(data.Bookmarks)
	if err != nil {
		return nil, err
	}
	bookmarkTags, err := legacyBookmarkTags(data.BookmarkTags)
	if err != nil {
		return nil, err
	}
	views, err := legacySavedViews(data.SavedViews)
	if err != nil {
		return nil, err
	}

	settings := legacySettings{SchemaVersion: 7, Locale: "en", ThemePreference: "system", SavedViews: views}
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
				// The legacy format accepts only its two locales, and the native app stores
				// an empty locale for "follow the system"; "en" is the honest fallback
				// because the legacy format has no third choice.
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

	summary, latest := legacyLastRun(data.SyncRuns)
	settings.LastSyncSummary = &summary

	payload := legacyExportPayload{
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

	// Indented by two spaces, so a diff of two backups reads the same whichever app wrote
	// them.
	return json.MarshalIndent(payload, "", "  ")
}

// legacyLastRun renders the newest recorded run as both the summary and the run record.
//
// The legacy format restores lastSyncSummary from payload.summary, so a truthful run is
// better than an idle placeholder: a user returning to the legacy format should not be told
// they have never synced. With no run at all, the idle summary with zero counts is the
// honest value.
func legacyLastRun(runs []json.RawMessage) (legacySummary, *legacySyncRun) {
	idle := legacySummary{Status: "idle"}
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
	kind := legacyErrorKind(dto.ErrorKind)
	summary := legacySummary{
		Status:        legacyStatus(dto.Status),
		FetchedCount:  dto.FetchedCount,
		InsertedCount: dto.InsertedCount,
		UpdatedCount:  dto.UpdatedCount,
		FailedCount:   dto.FailedCount,
		LastSyncedAt:  finished,
		ErrorKind:     kind,
		ErrorSummary:  dto.ErrorSummary,
	}
	run := &legacySyncRun{
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
