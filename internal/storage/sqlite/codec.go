package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// Several fields are stored as JSON strings in the schema. Keeping them as JSON
// columns preserves the exact shape a restored backup carries, so nothing has to
// be guessed about how a nested field was written.

// encodeStrings renders a string slice as a JSON array.
//
// A nil slice becomes "[]" rather than "null": the columns are NOT NULL, and a
// null would make every reader handle two empty cases.
func encodeStrings(values []string) (string, error) {
	if len(values) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("sqlite: encode string array: %w", err)
	}
	return string(encoded), nil
}

// decodeStrings parses a JSON array, tolerating null and the empty string.
func decodeStrings(raw sql.NullString) ([]string, error) {
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw.String), &values); err != nil {
		// A malformed array must not make the whole row unreadable: the rest of
		// the bookmark is still useful, and refusing to show it is worse than
		// showing it untagged.
		return nil, nil
	}
	return values, nil
}

// encodeMedia renders attachments as a JSON array.
func encodeMedia(media []domain.Media) (string, error) {
	if len(media) == 0 {
		return "[]", nil
	}
	encoded, err := json.Marshal(media)
	if err != nil {
		return "", fmt.Errorf("sqlite: encode media: %w", err)
	}
	return string(encoded), nil
}

func decodeMedia(raw sql.NullString) ([]domain.Media, error) {
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	var media []domain.Media
	if err := json.Unmarshal([]byte(raw.String), &media); err != nil {
		return nil, nil
	}
	return media, nil
}

// encodeMetrics renders engagement counts, keeping "absent" distinct from zero.
//
// The distinction matters: reporting a missing field as 0 would make an
// unmeasured post look like a post with no engagement.
func encodeMetrics(metrics *domain.Metrics) (sql.NullString, error) {
	if metrics == nil {
		return sql.NullString{}, nil
	}
	encoded, err := json.Marshal(metrics)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("sqlite: encode metrics: %w", err)
	}
	return sql.NullString{String: string(encoded), Valid: true}, nil
}

func decodeMetrics(raw sql.NullString) (*domain.Metrics, error) {
	if !raw.Valid || raw.String == "" {
		return nil, nil
	}
	var metrics domain.Metrics
	if err := json.Unmarshal([]byte(raw.String), &metrics); err != nil {
		return nil, nil
	}
	return &metrics, nil
}

// nullString renders an empty string as SQL NULL, for the columns that are
// nullable in the schema (author_avatar_url, last_seen_at).
func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

// nullTimestamp renders the zero time as SQL NULL, for the columns whose absence is a
// state rather than missing data: read_at unset is "unread", archived_at unset is "in
// play". Storing them as ” would parse back as a zero time anyway, but NULL is what the
// partial index and the IS NULL conditions are written against.
func nullTimestamp(value time.Time) sql.NullString {
	if value.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: domain.FormatTimestamp(value), Valid: true}
}

// bookmarkColumns is the column order every bookmark query selects.
//
// It is a constant because scanning by hand-written index is how a column gets
// silently swapped for another of the same type.
const bookmarkColumns = `tweet_id, tweet_url, author_name, author_handle, author_avatar_url,
	text, created_at_on_x, saved_at, last_seen_at, timeline_rank, read_at, archived_at,
	hashtags_json, source_kind, media_json, metrics_json, updated_at, is_longform`

// scanBookmark reads one bookmark row.
func scanBookmark(scanner interface{ Scan(...any) error }) (domain.Bookmark, error) {
	var (
		bookmark     domain.Bookmark
		avatarURL    sql.NullString
		savedAt      string
		lastSeenAt   sql.NullString
		timelineRank sql.NullInt64
		readAt       sql.NullString
		archivedAt   sql.NullString
		hashtagsJSON sql.NullString
		mediaJSON    sql.NullString
		metricsJSON  sql.NullString
		updatedAt    string
		isLongform   int
	)

	err := scanner.Scan(
		&bookmark.TweetID, &bookmark.TweetURL, &bookmark.AuthorName, &bookmark.AuthorHandle,
		&avatarURL, &bookmark.Text, &bookmark.CreatedAtOnX, &savedAt, &lastSeenAt,
		&timelineRank, &readAt, &archivedAt, &hashtagsJSON, &bookmark.SourceKind,
		&mediaJSON, &metricsJSON, &updatedAt, &isLongform,
	)
	if err != nil {
		return domain.Bookmark{}, err
	}

	bookmark.AuthorAvatarURL = avatarURL.String

	bookmark.SavedAt = domain.MustParseTimestamp(savedAt)
	if lastSeenAt.Valid && lastSeenAt.String != "" {
		bookmark.LastSeenAt = domain.MustParseTimestamp(lastSeenAt.String)
	}
	// Both are NULL until something local sets them, and NULL is the zero time: unread
	// and not archived.
	if readAt.Valid && readAt.String != "" {
		bookmark.ReadAt = domain.MustParseTimestamp(readAt.String)
	}
	if archivedAt.Valid && archivedAt.String != "" {
		bookmark.ArchivedAt = domain.MustParseTimestamp(archivedAt.String)
	}
	bookmark.UpdatedAt = domain.MustParseTimestamp(updatedAt)

	if timelineRank.Valid {
		rank := int(timelineRank.Int64)
		bookmark.TimelineRank = &rank
	}

	if bookmark.Hashtags, err = decodeStrings(hashtagsJSON); err != nil {
		return domain.Bookmark{}, err
	}
	if bookmark.Media, err = decodeMedia(mediaJSON); err != nil {
		return domain.Bookmark{}, err
	}
	if bookmark.Metrics, err = decodeMetrics(metricsJSON); err != nil {
		return domain.Bookmark{}, err
	}

	if bookmark.SourceKind == "" {
		bookmark.SourceKind = domain.SourceBookmark
	}

	// is_longform is authoritative: it was computed with the UTF-16 rule, which
	// SQLite cannot reproduce, so the stored flag is trusted over a re-derivation.
	_ = isLongform

	return bookmark, nil
}

// scanTimestamp reads a timestamp column.
func scanTimestamp(raw sql.NullString) time.Time {
	if !raw.Valid || raw.String == "" {
		return time.Time{}
	}
	return domain.MustParseTimestamp(raw.String)
}

// chunkStrings splits a slice into groups no larger than SQLite's comfortable
// bound-parameter count.
//
// A 50k-bookmark sync in one statement would exceed the parameter limit, and the
// failure appears only at scale.
func chunkStrings(values []string, size int) [][]string {
	if size <= 0 {
		size = 500
	}
	var out [][]string
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		out = append(out, values[start:end])
	}
	return out
}

// placeholders renders "?, ?, ?" for n values.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	buffer := make([]byte, 0, n*3)
	for i := 0; i < n; i++ {
		if i > 0 {
			buffer = append(buffer, ',', ' ')
		}
		buffer = append(buffer, '?')
	}
	return string(buffer)
}

// argsFor builds an []any from a []string.
func argsFor(values []string) []any {
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = value
	}
	return args
}

// escapeLike escapes the wildcards in a user's search text.
//
// Without it a search for "100%" matches everything, and a search for "_" matches
// any single character.
func escapeLike(value string) string {
	var out []byte
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '%', '_', '\\':
			out = append(out, '\\')
		}
		out = append(out, value[i])
	}
	return string(out)
}
