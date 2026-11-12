// Package importexport moves a library in and out of a .xmbundle.
//
// It is a wire format, not the internal model: the field names here are the ones
// the extension wrote and the ones a backup from it carries, so they are spelled
// out as DTOs rather than inferred from Go field names.
package importexport

import (
	"encoding/json"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
	"github.com/taoquo/x-master/apps/macos/internal/ports"
)

// The wire records. Only the fields the format defines are present, so a bundle
// round-trips through exactly the shape a user can read in a text editor.

// BookmarkDTO is one bookmark.
type BookmarkDTO struct {
	TweetID         string          `json:"tweetId"`
	TweetURL        string          `json:"tweetUrl"`
	AuthorName      string          `json:"authorName"`
	AuthorHandle    string          `json:"authorHandle"`
	AuthorAvatarURL string          `json:"authorAvatarUrl,omitempty"`
	Text            string          `json:"text"`
	CreatedAtOnX    string          `json:"createdAtOnX"`
	SavedAt         string          `json:"savedAt"`
	LastSeenAt      string          `json:"lastSeenAt,omitempty"`
	TimelineRank    *int            `json:"bookmarkTimelineRank,omitempty"`
	Hashtags        []string        `json:"hashtags,omitempty"`
	SourceKind      string          `json:"sourceKind,omitempty"`
	Media           []MediaDTO      `json:"media,omitempty"`
	Metrics         *MetricsDTO     `json:"metrics,omitempty"`
	UpdatedAt       string          `json:"updatedAt,omitempty"`
	RawPayload      json.RawMessage `json:"rawPayload,omitempty"`
}

// MediaDTO is one attachment.
type MediaDTO struct {
	Type      string  `json:"type"`
	URL       string  `json:"url"`
	PosterURL *string `json:"posterUrl,omitempty"`
	AltText   string  `json:"altText,omitempty"`
}

// MetricsDTO is a post's engagement counts.
//
// Every field is a pointer so an absent count stays absent; the extension wrote
// zero for a missing field, which is the defect this format does not repeat.
type MetricsDTO struct {
	Likes    *int `json:"likes,omitempty"`
	Retweets *int `json:"retweets,omitempty"`
	Replies  *int `json:"replies,omitempty"`
}

// TagDTO is one tag.
type TagDTO struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	CanonicalKey string   `json:"canonicalKey"`
	Aliases      []string `json:"aliases,omitempty"`
	ParentID     string   `json:"parentId,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

// BookmarkTagDTO is one relation.
type BookmarkTagDTO struct {
	BookmarkID string `json:"bookmarkId"`
	TagID      string `json:"tagId"`
	CreatedAt  string `json:"createdAt"`
	Source     string `json:"source"`
}

// ListDTO is one list.
type ListDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	NormalizedName string `json:"normalizedName"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// BookmarkListDTO is one bookmark's list.
type BookmarkListDTO struct {
	BookmarkID string `json:"bookmarkId"`
	ListID     string `json:"listId"`
	UpdatedAt  string `json:"updatedAt"`
}

// SuggestionDTO is one tag suggestion.
type SuggestionDTO struct {
	ID         string  `json:"id"`
	BookmarkID string  `json:"bookmarkId"`
	TagName    string  `json:"tagName"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"createdAt"`
	ResolvedAt string  `json:"resolvedAt,omitempty"`
}

// SavedViewDTO is one saved view.
type SavedViewDTO struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Query               string   `json:"query"`
	ActiveTagIDs        []string `json:"activeTagIds,omitempty"`
	ActiveAuthorHandles []string `json:"activeAuthorHandles,omitempty"`
	OnlyWithMedia       bool     `json:"onlyWithMedia"`
	OnlyLongform        bool     `json:"onlyLongform"`
	SortOrder           string   `json:"sortOrder"`
	ViewMode            string   `json:"viewMode"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

// --- domain to wire ---

// NewBookmarkDTO renders a bookmark for the bundle.
func NewBookmarkDTO(bookmark domain.Bookmark) BookmarkDTO {
	dto := BookmarkDTO{
		TweetID:         bookmark.TweetID,
		TweetURL:        bookmark.TweetURL,
		AuthorName:      bookmark.AuthorName,
		AuthorHandle:    bookmark.AuthorHandle,
		AuthorAvatarURL: bookmark.AuthorAvatarURL,
		Text:            bookmark.Text,
		CreatedAtOnX:    bookmark.CreatedAtOnX,
		SavedAt:         domain.FormatTimestamp(bookmark.SavedAt),
		TimelineRank:    bookmark.TimelineRank,
		Hashtags:        bookmark.Hashtags,
		SourceKind:      bookmark.SourceKind,
		// The raw payload is deliberately not exported: it is large, and it
		// duplicates what the parsed fields already hold.
	}

	if !bookmark.LastSeenAt.IsZero() {
		dto.LastSeenAt = domain.FormatTimestamp(bookmark.LastSeenAt)
	}
	if !bookmark.UpdatedAt.IsZero() {
		dto.UpdatedAt = domain.FormatTimestamp(bookmark.UpdatedAt)
	}

	for _, media := range bookmark.Media {
		dto.Media = append(dto.Media, MediaDTO{
			Type: media.Type, URL: media.URL, PosterURL: media.PosterURL, AltText: media.AltText,
		})
	}
	if bookmark.Metrics != nil {
		dto.Metrics = &MetricsDTO{
			Likes:    bookmark.Metrics.Likes,
			Retweets: bookmark.Metrics.Retweets,
			Replies:  bookmark.Metrics.Replies,
		}
	}
	return dto
}

// ToDomain turns a wire bookmark back into a domain one.
func (d BookmarkDTO) ToDomain() domain.Bookmark {
	bookmark := domain.Bookmark{
		TweetID:         d.TweetID,
		TweetURL:        d.TweetURL,
		AuthorName:      d.AuthorName,
		AuthorHandle:    d.AuthorHandle,
		AuthorAvatarURL: d.AuthorAvatarURL,
		Text:            d.Text,
		CreatedAtOnX:    d.CreatedAtOnX,
		SavedAt:         domain.MustParseTimestamp(d.SavedAt),
		LastSeenAt:      domain.MustParseTimestamp(d.LastSeenAt),
		TimelineRank:    d.TimelineRank,
		Hashtags:        d.Hashtags,
		SourceKind:      d.SourceKind,
		UpdatedAt:       domain.MustParseTimestamp(d.UpdatedAt),
	}

	for _, media := range d.Media {
		bookmark.Media = append(bookmark.Media, domain.Media{
			Type: media.Type, URL: media.URL, PosterURL: media.PosterURL, AltText: media.AltText,
		})
	}
	if d.Metrics != nil {
		bookmark.Metrics = &domain.Metrics{
			Likes: d.Metrics.Likes, Retweets: d.Metrics.Retweets, Replies: d.Metrics.Replies,
		}
	}
	// An absent source kind means an old record, which was a plain bookmark.
	if bookmark.SourceKind == "" {
		bookmark.SourceKind = domain.SourceBookmark
	}
	// updatedAt is not always present in an old export; savedAt is the honest
	// fallback because it is when this machine last wrote the record.
	if bookmark.UpdatedAt.IsZero() {
		bookmark.UpdatedAt = bookmark.SavedAt
	}
	return bookmark
}

// NewTagDTO renders a tag.
func NewTagDTO(tag domain.Tag) TagDTO {
	return TagDTO{
		ID: tag.ID, Name: tag.Name, CanonicalKey: tag.CanonicalKey,
		Aliases: tag.Aliases, ParentID: tag.ParentID,
		CreatedAt: tag.CreatedAt, UpdatedAt: tag.UpdatedAt,
	}
}

// ToDomain turns a wire tag back into a domain one.
func (d TagDTO) ToDomain() domain.Tag {
	tag := domain.Tag{
		ID: d.ID, Name: d.Name, CanonicalKey: d.CanonicalKey,
		Aliases: d.Aliases, ParentID: d.ParentID,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	// A bundle may predate the canonical key being stored, so it is derived when
	// absent rather than left empty: an empty key would defeat the uniqueness the
	// schema relies on.
	if tag.CanonicalKey == "" {
		tag.CanonicalKey = domain.CanonicalTagKey(tag.Name)
	}
	return tag
}

// NewListDTO renders a list.
func NewListDTO(list domain.List) ListDTO {
	return ListDTO{
		ID: list.ID, Name: list.Name, NormalizedName: list.Normalized,
		CreatedAt: domain.FormatTimestamp(list.CreatedAt),
		UpdatedAt: domain.FormatTimestamp(list.UpdatedAt),
	}
}

// ToDomain turns a wire list back into a domain one.
func (d ListDTO) ToDomain() domain.List {
	list := domain.List{
		ID: d.ID, Name: d.Name, Normalized: d.NormalizedName,
		CreatedAt: domain.MustParseTimestamp(d.CreatedAt),
		UpdatedAt: domain.MustParseTimestamp(d.UpdatedAt),
	}
	if list.Normalized == "" {
		list.Normalized = domain.NormalizeListName(list.Name)
	}
	return list
}

// NewSuggestionDTO renders a suggestion.
func NewSuggestionDTO(suggestion domain.TagSuggestion) SuggestionDTO {
	dto := SuggestionDTO{
		ID: suggestion.ID, BookmarkID: suggestion.BookmarkID, TagName: suggestion.TagName,
		Source: suggestion.Source, Confidence: suggestion.Confidence, Status: suggestion.Status,
		CreatedAt: domain.FormatTimestamp(suggestion.CreatedAt),
	}
	if suggestion.ResolvedAt != nil {
		dto.ResolvedAt = domain.FormatTimestamp(*suggestion.ResolvedAt)
	}
	return dto
}

// ToDomain turns a wire suggestion back into a domain one.
func (d SuggestionDTO) ToDomain() domain.TagSuggestion {
	suggestion := domain.TagSuggestion{
		ID: d.ID, BookmarkID: d.BookmarkID, TagName: d.TagName,
		Source: d.Source, Confidence: d.Confidence, Status: d.Status,
		CreatedAt: domain.MustParseTimestamp(d.CreatedAt),
	}
	if d.ResolvedAt != "" {
		resolved := domain.MustParseTimestamp(d.ResolvedAt)
		suggestion.ResolvedAt = &resolved
	}
	return suggestion
}

// NewSavedViewDTO renders a saved view.
func NewSavedViewDTO(view domain.SavedView) SavedViewDTO {
	return SavedViewDTO{
		ID: view.ID, Name: view.Name, Query: view.Query,
		ActiveTagIDs: view.ActiveTagIDs, ActiveAuthorHandles: view.ActiveAuthorHandles,
		OnlyWithMedia: view.OnlyWithMedia, OnlyLongform: view.OnlyLongform,
		SortOrder: view.SortOrder, ViewMode: view.ViewMode,
		CreatedAt: domain.FormatTimestamp(view.CreatedAt),
		UpdatedAt: domain.FormatTimestamp(view.UpdatedAt),
	}
}

// ToDomain turns a wire saved view back into a domain one.
func (d SavedViewDTO) ToDomain() domain.SavedView {
	return domain.SavedView{
		ID: d.ID, Name: d.Name, Query: d.Query,
		ActiveTagIDs: d.ActiveTagIDs, ActiveAuthorHandles: d.ActiveAuthorHandles,
		OnlyWithMedia: d.OnlyWithMedia, OnlyLongform: d.OnlyLongform,
		SortOrder: d.SortOrder, ViewMode: d.ViewMode,
		CreatedAt: domain.MustParseTimestamp(d.CreatedAt),
		UpdatedAt: domain.MustParseTimestamp(d.UpdatedAt),
	}
}

// marshalRecords renders a slice of DTOs as raw JSON for the bundle writer.
func marshalRecords[T any](records []T) ([]json.RawMessage, error) {
	out := make([]json.RawMessage, 0, len(records))
	for _, record := range records {
		encoded, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		out = append(out, encoded)
	}
	return out, nil
}

// decodeRecords reads a wire array out of a bundle.
func decodeRecords[T any](raw []byte) ([]T, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var records []T
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// nowStamp is the bundle's own timestamp shape.
func nowStamp(t time.Time) string { return domain.FormatTimestamp(t) }

// SyncRunDTO is one recorded sync attempt.
type SyncRunDTO struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	StartedAt       string `json:"startedAt"`
	FinishedAt      string `json:"finishedAt,omitempty"`
	FetchedCount    int    `json:"fetchedCount"`
	InsertedCount   int    `json:"insertedCount"`
	UpdatedCount    int    `json:"updatedCount"`
	FailedCount     int    `json:"failedCount"`
	PagesCompleted  int    `json:"pagesCompleted"`
	ErrorKind       string `json:"errorKind,omitempty"`
	ErrorSummary    string `json:"errorSummary,omitempty"`
	StrategyVersion int    `json:"strategyVersion"`
}

// NewSyncRunDTO renders a sync run.
func NewSyncRunDTO(run domain.SyncRun) SyncRunDTO {
	dto := SyncRunDTO{
		ID: run.ID, Status: run.Status,
		StartedAt:       domain.FormatTimestamp(run.StartedAt),
		FetchedCount:    run.FetchedCount,
		InsertedCount:   run.InsertedCount,
		UpdatedCount:    run.UpdatedCount,
		FailedCount:     run.FailedCount,
		PagesCompleted:  run.PagesCompleted,
		ErrorKind:       run.ErrorKind,
		ErrorSummary:    run.ErrorSummary,
		StrategyVersion: run.StrategyVersion,
	}
	if run.FinishedAt != nil {
		dto.FinishedAt = domain.FormatTimestamp(*run.FinishedAt)
	}
	return dto
}

// ToDomain turns a wire sync run back into a domain one.
func (d SyncRunDTO) ToDomain() domain.SyncRun {
	run := domain.SyncRun{
		ID: d.ID, Status: d.Status,
		StartedAt:       domain.MustParseTimestamp(d.StartedAt),
		FetchedCount:    d.FetchedCount,
		InsertedCount:   d.InsertedCount,
		UpdatedCount:    d.UpdatedCount,
		FailedCount:     d.FailedCount,
		PagesCompleted:  d.PagesCompleted,
		ErrorKind:       d.ErrorKind,
		ErrorSummary:    d.ErrorSummary,
		StrategyVersion: d.StrategyVersion,
	}
	if d.FinishedAt != "" {
		finished := domain.MustParseTimestamp(d.FinishedAt)
		run.FinishedAt = &finished
	}
	// A run whose status this build does not know is recorded as an error: it
	// certainly did not succeed, and calling it "idle" would be a lie.
	if !domain.ValidSyncStatus(run.Status) {
		run.Status = domain.SyncError
	}
	// The error-kind vocabulary has two generations; an unknown value is kept
	// rather than defaulted, because the column deliberately accepts both.
	if run.StrategyVersion == 0 {
		run.StrategyVersion = domain.SyncStrategyVersion
	}
	return run
}

// AnalysisDTO is one verdict from the auto-tagger's ledger.
type AnalysisDTO struct {
	BookmarkID    string `json:"bookmarkId"`
	AnalyzedAt    string `json:"analyzedAt"`
	PromptVersion int    `json:"promptVersion"`
	Model         string `json:"model"`
	ContentHash   string `json:"contentHash"`
	Outcome       string `json:"outcome"`
	TagCount      int    `json:"tagCount"`
}

// NewAnalysisDTO renders an analysis record.
func NewAnalysisDTO(record domain.LlmAnalysis) AnalysisDTO {
	return AnalysisDTO{
		BookmarkID: record.BookmarkID, AnalyzedAt: record.AnalyzedAt,
		PromptVersion: record.PromptVersion, Model: record.Model,
		ContentHash: record.ContentHash, Outcome: record.Outcome, TagCount: record.TagCount,
	}
}

// ToDomain turns a wire analysis record back into a domain one.
func (d AnalysisDTO) ToDomain() domain.LlmAnalysis {
	return domain.LlmAnalysis{
		BookmarkID: d.BookmarkID, AnalyzedAt: d.AnalyzedAt,
		PromptVersion: d.PromptVersion, Model: d.Model,
		ContentHash: d.ContentHash, Outcome: d.Outcome, TagCount: d.TagCount,
	}
}

// RawPayloadDTO is one untouched response body.
//
// It is only written when the user explicitly asks for it: the parsed fields
// already carry everything the app reads, so this is a diagnostic record.
type RawPayloadDTO struct {
	TweetID    string          `json:"tweetId"`
	Payload    json.RawMessage `json:"payload"`
	CapturedAt string          `json:"capturedAt"`
}

// NewRawPayloadDTO renders a raw payload.
func NewRawPayloadDTO(payload ports.RawPayload) RawPayloadDTO {
	return RawPayloadDTO{
		TweetID:    payload.TweetID,
		Payload:    json.RawMessage(payload.PayloadJSON),
		CapturedAt: payload.CapturedAt,
	}
}
