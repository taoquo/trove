package xsource

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/taoquo/trove/internal/domain"
)

// ParseOptions configures parsing.
type ParseOptions struct {
	// SeenAt stamps savedAt/lastSeenAt; it is the moment the sync observed the
	// post, not anything X reported.
	SeenAt time.Time
	// CaptureRawPayload also returns each tweet's raw JSON for the diagnostics
	// table. It costs a re-marshal per bookmark, so it is opt-in and driven by a
	// setting: the main table must never depend on raw JSON to render.
	CaptureRawPayload bool
}

// ParseResult is what one timeline response yielded.
type ParseResult struct {
	Bookmarks []domain.Bookmark
	// NextCursor is empty when the response carried no bottom cursor.
	NextCursor string
	// UnrecognizedEntries counts entries that looked like tweets but whose shape
	// we did not understand. Counting them separately distinguishes a silent X change
	// from an empty page.
	UnrecognizedEntries int
	// SkippedEntries counts entries that were understood but had nothing to
	// store, such as an unavailable post. These are normal.
	SkippedEntries int
	// RawPayloads maps tweet id to the raw tweet result, populated only when
	// ParseOptions.CaptureRawPayload is set.
	RawPayloads map[string]json.RawMessage
}

// entryPrefix and cursorPrefix identify timeline entries.
const (
	entryTweetPrefix  = "tweet-"
	entryCursorPrefix = "cursor-bottom-"
)

// ParseBookmarkEntries turns a decoded timeline response into bookmarks.
//
// A missing TimelineAddEntries instruction is a schema change: X returning
// "nothing here" does so as an *empty* entry list, so an absent instruction means
// the shape moved rather than the list being empty.
func ParseBookmarkEntries(payload map[string]any, opts ParseOptions) (ParseResult, error) {
	seenAt := opts.SeenAt
	instructions := timelineInstructions(payload)

	var addEntries map[string]any
	for _, instruction := range instructions {
		if str(instruction["type"]) == "TimelineAddEntries" {
			addEntries = instruction
			break
		}
	}
	if addEntries == nil {
		return ParseResult{}, newError(domain.ErrResponseSchemaChanged, "parse", 0,
			fmt.Errorf("no TimelineAddEntries instruction in the bookmark timeline"))
	}

	entries := slice(addEntries["entries"])
	seenAtText := domain.FormatTimestamp(seenAt)

	result := ParseResult{Bookmarks: make([]domain.Bookmark, 0, len(entries))}
	tweetEntries := 0

	for _, rawEntry := range entries {
		entry := obj(rawEntry)
		entryID := str(entry["entryId"])

		if strings.HasPrefix(entryID, entryCursorPrefix) {
			if content := obj(entry["content"]); content != nil {
				result.NextCursor = str(content["value"])
			}
			continue
		}

		if !strings.HasPrefix(entryID, entryTweetPrefix) {
			// Promoted posts, module headers and anything else that is not a
			// saved post.
			continue
		}
		tweetEntries++

		bookmark, rawResult, ok := parseTweetEntry(entry, seenAtText)
		if !ok {
			result.UnrecognizedEntries++
			continue
		}
		if bookmark.TweetID == "" {
			// Understood, but there is nothing to store: an unavailable or
			// deleted post still occupies a slot in the timeline.
			result.SkippedEntries++
			continue
		}
		if opts.CaptureRawPayload && rawResult != nil && bookmark.TweetID != "" {
			if encoded, err := json.Marshal(rawResult); err == nil {
				if result.RawPayloads == nil {
					result.RawPayloads = make(map[string]json.RawMessage, len(entries))
				}
				result.RawPayloads[bookmark.TweetID] = encoded
			}
		}
		result.Bookmarks = append(result.Bookmarks, bookmark)
	}

	// Every tweet-shaped entry was unfamiliar, and nothing came out. That is a
	// changed response shape, not an empty page.
	if tweetEntries > 0 && len(result.Bookmarks) == 0 && result.UnrecognizedEntries == tweetEntries {
		return ParseResult{}, newError(domain.ErrResponseSchemaChanged, "parse", 0,
			fmt.Errorf("none of the %d tweet entries matched the expected shape", tweetEntries))
	}

	// A cursor-less first page of a non-empty timeline is also suspicious.
	if result.NextCursor == "" && len(result.Bookmarks) == 0 && result.UnrecognizedEntries > 0 {
		return ParseResult{}, newError(domain.ErrResponseSchemaChanged, "parse", 0,
			fmt.Errorf("timeline yielded no bookmarks and no cursor"))
	}

	return result, nil
}

// timelineInstructions finds the instructions array under either known path.
//
// Two paths are supported because the same parser is reused for the search
// timeline.
func timelineInstructions(payload map[string]any) []map[string]any {
	data := obj(payload["data"])
	if data == nil {
		return nil
	}

	paths := [][]string{
		{"bookmark_timeline_v2", "timeline", "instructions"},
		{"search_by_raw_query", "bookmarks_search_timeline", "timeline", "instructions"},
	}

	for _, path := range paths {
		if found := dig(data, path...); found != nil {
			if list := slice(found); list != nil {
				out := make([]map[string]any, 0, len(list))
				for _, item := range list {
					if m := obj(item); m != nil {
						out = append(out, m)
					}
				}
				return out
			}
		}
	}
	return nil
}

// parseTweetEntry reads one tweet entry.
//
// It returns ok=false when the entry does not have the shape we expect, which is
// what distinguishes "X changed the response" from "this post is unavailable".
func parseTweetEntry(entry map[string]any, seenAtText string) (domain.Bookmark, map[string]any, bool) {
	content := obj(entry["content"])
	if content == nil {
		return domain.Bookmark{}, nil, false
	}
	itemContent := obj(content["itemContent"])
	if itemContent == nil {
		return domain.Bookmark{}, nil, false
	}
	tweetResults := obj(itemContent["tweet_results"])
	if tweetResults == nil {
		return domain.Bookmark{}, nil, false
	}

	result := unwrapTweetResult(obj(tweetResults["result"]))
	if result == nil {
		return domain.Bookmark{}, nil, false
	}

	legacy := obj(result["legacy"])
	userLegacy := obj(dig(result, "core", "user_results", "result", "legacy"))

	// A visible post wrapped in a tweet object nests its own legacy elsewhere.
	if legacy == nil {
		if inner := unwrapTweetResult(obj(result["tweet"])); inner != nil {
			result = inner
			legacy = obj(result["legacy"])
			if userLegacy == nil {
				userLegacy = obj(dig(result, "core", "user_results", "result", "legacy"))
			}
		}
	}

	restID := str(result["rest_id"])
	screenName := str(userLegacy["screen_name"])

	// Text: a long-form Note Tweet carries the real body; legacy.full_text holds
	// only a truncated teaser for those.
	text := str(dig(result, "note_tweet", "note_tweet_results", "result", "text"))
	sourceKind := domain.SourceBookmark
	if text != "" {
		sourceKind = domain.SourceNoteTweet
	} else if legacy != nil {
		text = str(legacy["full_text"])
	}

	bookmark := domain.Bookmark{
		TweetID:         restID,
		TweetURL:        tweetURL(screenName, restID),
		AuthorName:      str(userLegacy["name"]),
		AuthorHandle:    screenName,
		AuthorAvatarURL: firstNonEmpty(str(userLegacy["profile_image_url_https"]), str(userLegacy["profile_image_url"])),
		Text:            text,
		CreatedAtOnX:    str(legacy["created_at"]),
		SavedAt:         domain.MustParseTimestamp(seenAtText),
		LastSeenAt:      domain.MustParseTimestamp(seenAtText),
		SourceKind:      sourceKind,
		UpdatedAt:       domain.MustParseTimestamp(seenAtText),
		Media:           parseMedia(legacy),
		Metrics:         parseMetrics(legacy),
		Hashtags:        ParseHashtags(legacy),
	}

	return bookmark, result, true
}

// unwrapTweetResult follows the visible-tweet wrapper the timeline sometimes adds.
func unwrapTweetResult(result map[string]any) map[string]any {
	if result == nil {
		return nil
	}
	if inner := obj(result["tweet"]); inner != nil {
		return inner
	}
	return result
}

func tweetURL(screenName, restID string) string {
	if restID == "" {
		return ""
	}
	if screenName != "" {
		return "https://x.com/" + screenName + "/status/" + restID
	}
	return "https://x.com/i/status/" + restID
}

// ParseHashtags reads legacy.entities.hashtags, de-duplicating case-insensitively
// while keeping the author's original casing.
func ParseHashtags(legacy map[string]any) []string {
	if legacy == nil {
		return nil
	}
	raw := slice(dig(legacy, "entities", "hashtags"))
	if raw == nil {
		return nil
	}

	seen := make(map[string]bool, len(raw))
	names := make([]string, 0, len(raw))
	for _, item := range raw {
		name := strings.TrimSpace(str(obj(item)["text"]))
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, name)
	}
	return names
}

// parseMedia converts the attachments, choosing the highest-bitrate mp4 for
// video so the poster is playable rather than a stream manifest.
//
// A photo has no poster at all, which is represented as a nil pointer: collapsing
// it to "" makes the UI treat an image as a video with a missing poster.
func parseMedia(legacy map[string]any) []domain.Media {
	if legacy == nil {
		return nil
	}
	raw := slice(dig(legacy, "extended_entities", "media"))
	if raw == nil {
		return nil
	}

	media := make([]domain.Media, 0, len(raw))
	for _, item := range raw {
		entry := obj(item)
		if entry == nil {
			continue
		}

		mediaType := str(entry["type"])
		poster := firstNonEmpty(str(entry["media_url_https"]), str(entry["media_url"]))

		url := poster
		var posterPtr *string
		if mediaType == domain.MediaVideo || mediaType == domain.MediaAnimatedGif {
			// Keep the poster even when empty, so a video with no poster stays
			// distinguishable from an image.
			posterCopy := poster
			posterPtr = &posterCopy
			if best := highestBitrateMP4(entry); best != "" {
				url = best
			}
		}

		media = append(media, domain.Media{
			Type:      mediaType,
			URL:       url,
			PosterURL: posterPtr,
			AltText:   str(entry["ext_alt_text"]),
		})
	}
	return media
}

// highestBitrateMP4 picks the video/mp4 variant with the largest bitrate.
func highestBitrateMP4(entry map[string]any) string {
	variants := slice(dig(entry, "video_info", "variants"))
	best := ""
	bestBitrate := -1

	for _, raw := range variants {
		variant := obj(raw)
		if variant == nil || str(variant["content_type"]) != "video/mp4" {
			continue
		}
		url := str(variant["url"])
		if url == "" {
			continue
		}
		bitrate := intValue(variant["bitrate"])
		if bitrate > bestBitrate {
			bestBitrate = bitrate
			best = url
		}
	}
	return best
}

// parseMetrics reads the engagement counts, keeping "absent" distinct from zero.
//
// Coercing a missing count to zero would record it as a real zero — a post with
// unknown likes would look like a post with no likes, and "most liked" would sort it
// as such.
func parseMetrics(legacy map[string]any) *domain.Metrics {
	if legacy == nil {
		return nil
	}

	likes, hasLikes := optionalInt(legacy["favorite_count"])
	retweets, hasRetweets := optionalInt(legacy["retweet_count"])
	replies, hasReplies := optionalInt(legacy["reply_count"])

	if !hasLikes && !hasRetweets && !hasReplies {
		// No metric was returned at all: nil says so.
		return nil
	}

	metrics := &domain.Metrics{}
	if hasLikes {
		metrics.Likes = &likes
	}
	if hasRetweets {
		metrics.Retweets = &retweets
	}
	if hasReplies {
		metrics.Replies = &replies
	}
	return metrics
}

// --- small decoding helpers ---

func obj(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func slice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

// dig walks nested maps, returning nil as soon as the path leaves the object.
func dig(v any, keys ...string) any {
	current := v
	for _, key := range keys {
		m := obj(current)
		if m == nil {
			return nil
		}
		current = m[key]
	}
	return current
}

// str renders a JSON scalar as a string.
//
// json.Number matters here: tweet ids exceed the range a float64 represents
// exactly, so the decoder is configured with UseNumber and ids are never parsed
// as floats.
func str(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case json.Number:
		return value.String()
	case bool:
		return strconv.FormatBool(value)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return ""
	}
}

func intValue(v any) int {
	switch value := v.(type) {
	case json.Number:
		if n, err := value.Int64(); err == nil {
			return int(n)
		}
		if f, err := value.Float64(); err == nil {
			return int(f)
		}
	case float64:
		return int(value)
	case int:
		return value
	}
	return 0
}

// optionalInt reports a count and whether it was actually present.
func optionalInt(v any) (int, bool) {
	switch v.(type) {
	case nil:
		return 0, false
	case json.Number, float64, int:
		return intValue(v), true
	default:
		return 0, false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
