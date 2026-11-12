package xsource

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/taoquo/x-master/apps/macos/internal/domain"
)

// decode parses a fixture the way the client does, with json.Number, so ids are
// never routed through a float64.
func decode(t *testing.T, raw string) map[string]any {
	t.Helper()
	payload, err := decodePayload([]byte(raw))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return payload
}

func mustParse(t *testing.T, raw string) ParseResult {
	t.Helper()
	result, err := ParseBookmarkEntries(decode(t, raw), ParseOptions{SeenAt: testTime()})
	if err != nil {
		t.Fatalf("ParseBookmarkEntries: %v", err)
	}
	return result
}

func testTime() time.Time {
	return time.Date(2026, 10, 12, 14, 22, 31, 0, time.UTC)
}

// pageWithResult wraps one tweet result object in the timeline envelope.
//
// Written as a helper because hand-counting the envelope's closing braces in
// every fixture is noisy and easy to get wrong.
func pageWithResult(resultJSON string) string {
	return `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[` +
		`{"type":"TimelineAddEntries","entries":[` +
		`{"entryId":"tweet-x","content":{"itemContent":{"tweet_results":{"result":` + resultJSON + `}}}}` +
		`]}]}}}}`
}

// pageWithLegacy wraps a legacy object, for tests that vary only those fields.
func pageWithLegacy(legacyJSON string) string {
	return pageWithResult(`{"rest_id":"m","legacy":` + legacyJSON + `}`)
}

// The fixtures below are the ones from the extension's own parser tests
// (tests/x/parseBookmarks.test.ts), so the port is checked against the behaviour
// that was already pinned rather than against a fresh reading.
const twoBookmarkPage = `{
  "data": {
    "bookmark_timeline_v2": {
      "timeline": {
        "instructions": [
          {
            "type": "TimelineAddEntries",
            "entries": [
              {
                "entryId": "tweet-123",
                "content": {
                  "itemContent": {
                    "tweet_results": {
                      "result": {
                        "rest_id": "123",
                        "legacy": {
                          "full_text": "hello world",
                          "created_at": "Mon Mar 15 00:00:00 +0000 2026",
                          "favorite_count": 10,
                          "retweet_count": 2,
                          "reply_count": 1
                        },
                        "core": {
                          "user_results": {
                            "result": {
                              "legacy": {
                                "name": "Alice",
                                "screen_name": "alice",
                                "profile_image_url_https": "https://pbs.twimg.com/profile_images/alice_normal.jpg"
                              }
                            }
                          }
                        }
                      }
                    }
                  }
                }
              },
              {
                "entryId": "tweet-456",
                "content": {
                  "itemContent": {
                    "tweet_results": {
                      "result": {
                        "rest_id": "456",
                        "legacy": {
                          "full_text": "legacy teaser",
                          "created_at": "Tue Mar 16 00:00:00 +0000 2026",
                          "favorite_count": 5,
                          "retweet_count": 1,
                          "reply_count": 0
                        },
                        "note_tweet": {
                          "note_tweet_results": { "result": { "text": "full note tweet body" } }
                        },
                        "core": {
                          "user_results": {
                            "result": { "legacy": { "name": "Bob", "screen_name": "bob" } }
                          }
                        }
                      }
                    }
                  }
                }
              },
              { "entryId": "cursor-bottom-1", "content": { "value": "next-cursor" } }
            ]
          }
        ]
      }
    }
  }
}`

func TestParseExtractsNormalizedBookmarks(t *testing.T) {
	result := mustParse(t, twoBookmarkPage)

	if len(result.Bookmarks) != 2 {
		t.Fatalf("got %d bookmarks, want 2", len(result.Bookmarks))
	}

	first := result.Bookmarks[0]
	if first.TweetID != "123" {
		t.Errorf("TweetID = %q, want 123", first.TweetID)
	}
	if first.AuthorHandle != "alice" {
		t.Errorf("AuthorHandle = %q, want alice", first.AuthorHandle)
	}
	if first.AuthorAvatarURL != "https://pbs.twimg.com/profile_images/alice_normal.jpg" {
		t.Errorf("AuthorAvatarURL = %q", first.AuthorAvatarURL)
	}
	if first.Text != "hello world" {
		t.Errorf("Text = %q", first.Text)
	}
	if first.SourceKind != domain.SourceBookmark {
		t.Errorf("SourceKind = %q, want x-bookmark", first.SourceKind)
	}
	if first.TweetURL != "https://x.com/alice/status/123" {
		t.Errorf("TweetURL = %q", first.TweetURL)
	}

	// The Note Tweet body wins over the truncated legacy teaser.
	second := result.Bookmarks[1]
	if second.Text != "full note tweet body" {
		t.Errorf("Text = %q, want the note tweet body", second.Text)
	}
	if second.SourceKind != domain.SourceNoteTweet {
		t.Errorf("SourceKind = %q, want x-note-tweet", second.SourceKind)
	}

	if result.NextCursor != "next-cursor" {
		t.Errorf("NextCursor = %q, want next-cursor", result.NextCursor)
	}
}

func TestParseMissingInstructionIsASchemaChange(t *testing.T) {
	_, err := ParseBookmarkEntries(
		decode(t, `{"data":{"bookmark_timeline_v2":{"timeline":{}}}}`), ParseOptions{SeenAt: testTime()})
	if err == nil {
		t.Fatal("a missing TimelineAddEntries must be reported, not treated as an empty page")
	}
	if kind := KindOf(err); kind != domain.ErrResponseSchemaChanged {
		t.Errorf("kind = %q, want %q", kind, domain.ErrResponseSchemaChanged)
	}
}

// TestParseAcceptsRecognizableEmptyTimeline is the counterweight to the test
// above: X saying "nothing here" is an empty entry list, and that must not be
// mistaken for a schema change.
func TestParseAcceptsRecognizableEmptyTimeline(t *testing.T) {
	result := mustParse(t, `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[
		{"type":"TimelineAddEntries","entries":[]}]}}}}`)

	if len(result.Bookmarks) != 0 {
		t.Errorf("got %d bookmarks, want none", len(result.Bookmarks))
	}
	if result.NextCursor != "" {
		t.Errorf("NextCursor = %q, want empty", result.NextCursor)
	}
}

// TestParseDistinguishesUnrecognizedFromUnavailable is the fix for the failure
// mode where a silent X change looked like "no new bookmarks".
func TestParseDistinguishesUnrecognizedFromUnavailable(t *testing.T) {
	t.Run("an unavailable post is skipped, not a schema change", func(t *testing.T) {
		result := mustParse(t, `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[
			{"type":"TimelineAddEntries","entries":[
				{"entryId":"tweet-gone","content":{"itemContent":{"tweet_results":{"result":{
					"legacy":{"full_text":"deleted"}}}}}},
				{"entryId":"tweet-789","content":{"itemContent":{"tweet_results":{"result":{
					"rest_id":"789","legacy":{"full_text":"valid"}}}}}}
			]}]}}}}`)

		if len(result.Bookmarks) != 1 || result.Bookmarks[0].TweetID != "789" {
			t.Fatalf("got %d bookmarks (%v), want just 789", len(result.Bookmarks), result.Bookmarks)
		}
		if result.SkippedEntries != 1 {
			t.Errorf("SkippedEntries = %d, want 1", result.SkippedEntries)
		}
	})

	t.Run("entries whose shape is unfamiliar are a schema change", func(t *testing.T) {
		_, err := ParseBookmarkEntries(decode(t, `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[
			{"type":"TimelineAddEntries","entries":[
				{"entryId":"tweet-a","content":{"itemContent":{"something_else":{"x":1}}}},
				{"entryId":"tweet-b","content":{"itemContent":{"something_else":{"x":2}}}}
			]}]}}}}`), ParseOptions{SeenAt: testTime()})
		if err == nil {
			t.Fatal("when no tweet entry is understood, that is a schema change")
		}
		if kind := KindOf(err); kind != domain.ErrResponseSchemaChanged {
			t.Errorf("kind = %q, want %q", kind, domain.ErrResponseSchemaChanged)
		}
	})

	t.Run("a readable page with some odd entries still succeeds", func(t *testing.T) {
		result := mustParse(t, `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[
			{"type":"TimelineAddEntries","entries":[
				{"entryId":"tweet-ok","content":{"itemContent":{"tweet_results":{"result":{
					"rest_id":"ok","legacy":{"full_text":"fine"}}}}}},
				{"entryId":"tweet-odd","content":{"itemContent":{"something_else":{}}}}
			]}]}}}}`)

		if len(result.Bookmarks) != 1 {
			t.Fatalf("got %d bookmarks, want 1", len(result.Bookmarks))
		}
		if result.UnrecognizedEntries != 1 {
			t.Errorf("UnrecognizedEntries = %d, want 1 recorded for diagnostics", result.UnrecognizedEntries)
		}
	})
}

func TestParsePreservesTextWhitespace(t *testing.T) {
	result := mustParse(t, pageWithResult(`{
		"rest_id":"ws",
		"legacy":{"full_text":"line one\n\n  line two with   spaces","created_at":"Wed Mar 17 00:00:00 +0000 2026"},
		"core":{"user_results":{"result":{"legacy":{"name":"W","screen_name":"w","profile_image_url":"https://pbs.twimg.com/profile_images/w.jpg"}}}}
	}`))

	if got := result.Bookmarks[0].Text; got != "line one\n\n  line two with   spaces" {
		t.Errorf("Text = %q, whitespace must be preserved verbatim", got)
	}
	if got := result.Bookmarks[0].AuthorAvatarURL; got != "https://pbs.twimg.com/profile_images/w.jpg" {
		t.Errorf("AuthorAvatarURL = %q, want the non-https fallback", got)
	}
}

func TestParseVideoPicksHighestBitrateMP4(t *testing.T) {
	result := mustParse(t, pageWithResult(`{
		"rest_id":"v",
		"legacy":{"full_text":"video","extended_entities":{"media":[{
			"type":"video",
			"media_url_https":"https://pbs.twimg.com/ext_tw_video_thumb/poster.jpg",
			"video_info":{"variants":[
				{"content_type":"application/x-mpegURL","url":"https://video.twimg.com/master.m3u8"},
				{"bitrate":832000,"content_type":"video/mp4","url":"https://video.twimg.com/video-360.mp4"},
				{"bitrate":2176000,"content_type":"video/mp4","url":"https://video.twimg.com/video-720.mp4"}
			]}}]}}
	}`))

	if len(result.Bookmarks) != 1 {
		t.Fatalf("got %d bookmarks, want 1", len(result.Bookmarks))
	}
	media := result.Bookmarks[0].Media
	if len(media) != 1 {
		t.Fatalf("got %d media, want 1", len(media))
	}
	if media[0].Type != domain.MediaVideo {
		t.Errorf("Type = %q, want video", media[0].Type)
	}
	if media[0].URL != "https://video.twimg.com/video-720.mp4" {
		t.Errorf("URL = %q, want the highest-bitrate mp4", media[0].URL)
	}
	if media[0].PosterURL == nil || *media[0].PosterURL != "https://pbs.twimg.com/ext_tw_video_thumb/poster.jpg" {
		t.Errorf("PosterURL = %v, want the thumbnail", media[0].PosterURL)
	}
}

func TestParsePhotoHasNoPoster(t *testing.T) {
	result := mustParse(t, pageWithResult(`{
		"rest_id":"p",
		"legacy":{"full_text":"photo","extended_entities":{"media":[{
			"type":"photo","media_url_https":"https://pbs.twimg.com/media/photo.jpg",
			"ext_alt_text":"a description"}]}}
	}`))

	media := result.Bookmarks[0].Media
	if len(media) != 1 {
		t.Fatalf("got %d media, want 1", len(media))
	}
	if media[0].PosterURL != nil {
		t.Errorf("PosterURL = %q, want nil for a photo", *media[0].PosterURL)
	}
	if media[0].URL != "https://pbs.twimg.com/media/photo.jpg" {
		t.Errorf("URL = %q", media[0].URL)
	}
	if media[0].AltText != "a description" {
		t.Errorf("AltText = %q", media[0].AltText)
	}
}

func TestParseMetricsKeepsZeroDistinctFromMissing(t *testing.T) {
	tests := []struct {
		name        string
		legacy      string
		wantNil     bool
		wantLikes   *int
		wantRetweet *int
	}{
		{name: "all present", legacy: `"favorite_count":10,"retweet_count":2,"reply_count":1`,
			wantLikes: intPtr(10), wantRetweet: intPtr(2)},
		{name: "explicit zero is a real zero", legacy: `"favorite_count":0`,
			wantLikes: intPtr(0)},
		{name: "no metrics at all is nil", legacy: `"full_text":"x"`, wantNil: true},
		{name: "one metric present", legacy: `"retweet_count":7`,
			wantRetweet: intPtr(7)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := mustParse(t, `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[
				{"type":"TimelineAddEntries","entries":[
					{"entryId":"tweet-m","content":{"itemContent":{"tweet_results":{"result":{
						"rest_id":"m","legacy":{`+tc.legacy+`}}}}}}
				]}]}}}}`)

			metrics := result.Bookmarks[0].Metrics
			if tc.wantNil {
				if metrics != nil {
					t.Fatalf("Metrics = %+v, want nil when no count was returned", metrics)
				}
				return
			}
			if metrics == nil {
				t.Fatal("Metrics = nil, want a value")
			}
			if !equalIntPtr(metrics.Likes, tc.wantLikes) {
				t.Errorf("Likes = %s, want %s", formatIntPtr(metrics.Likes), formatIntPtr(tc.wantLikes))
			}
			if !equalIntPtr(metrics.Retweets, tc.wantRetweet) {
				t.Errorf("Retweets = %s, want %s", formatIntPtr(metrics.Retweets), formatIntPtr(tc.wantRetweet))
			}
		})
	}
}

func TestParseHashtags(t *testing.T) {
	tests := []struct {
		name   string
		legacy string
		want   []string
	}{
		{"none", `{}`, nil},
		{"empty entities", `{"entities":{}}`, nil},
		{"dedupes case-insensitively keeping first casing", `{"entities":{"hashtags":[
			{"text":"AI"},{"text":"ai"},{"text":" Rust "},{"text":""}]}}`, []string{"AI", "Rust"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var legacy map[string]any
			if err := json.Unmarshal([]byte(tc.legacy), &legacy); err != nil {
				t.Fatalf("bad fixture: %v", err)
			}
			got := ParseHashtags(legacy)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("hashtag %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestParseSupportsSearchTimelinePath covers the second instruction path, which
// the old parser also accepted.
func TestParseSupportsSearchTimelinePath(t *testing.T) {
	result := mustParse(t, `{"data":{"search_by_raw_query":{"bookmarks_search_timeline":{"timeline":{"instructions":[`+
		`{"type":"TimelineAddEntries","entries":[`+
		`{"entryId":"tweet-s","content":{"itemContent":{"tweet_results":{"result":`+
		`{"rest_id":"s","legacy":{"full_text":"via search path"}}}}}}`+
		`]}]}}}}}`)

	if len(result.Bookmarks) != 1 || result.Bookmarks[0].TweetID != "s" {
		t.Fatalf("got %+v, want one bookmark from the search timeline path", result.Bookmarks)
	}
}

func TestParseKeepsLargeTweetIDsExact(t *testing.T) {
	const bigID = "18446744073709551615" // max uint64
	result := mustParse(t, `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[
		{"type":"TimelineAddEntries","entries":[
			{"entryId":"tweet-big","content":{"itemContent":{"tweet_results":{"result":{
				"rest_id":"`+bigID+`","legacy":{"full_text":"big"}}}}}}
		]}]}}}}`)

	if got := result.Bookmarks[0].TweetID; got != bigID {
		t.Errorf("TweetID = %q, want %q exactly", got, bigID)
	}
	if strings.Contains(result.Bookmarks[0].TweetURL, "1.8446744") {
		t.Errorf("TweetURL = %q, the id was routed through a float", result.Bookmarks[0].TweetURL)
	}
}

// TestParseHandlesWrappedVisibleTweet covers the extra `tweet` wrapper the
// timeline adds around some results.
func TestParseHandlesWrappedVisibleTweet(t *testing.T) {
	result := mustParse(t, `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[
		{"type":"TimelineAddEntries","entries":[
			{"entryId":"tweet-wrapped","content":{"itemContent":{"tweet_results":{"result":{
				"tweet":{"rest_id":"wrapped","legacy":{"full_text":"inside the wrapper"},
					"core":{"user_results":{"result":{"legacy":{"name":"W","screen_name":"w"}}}}}}}}}}
		]}]}}}}`)

	if len(result.Bookmarks) != 1 {
		t.Fatalf("got %d bookmarks, want 1", len(result.Bookmarks))
	}
	if result.Bookmarks[0].TweetID != "wrapped" || result.Bookmarks[0].Text != "inside the wrapper" {
		t.Errorf("got %+v, want the wrapped tweet unwrapped", result.Bookmarks[0])
	}
}

func TestParseSetsTimestampsFromSeenAt(t *testing.T) {
	result := mustParse(t, `{"data":{"bookmark_timeline_v2":{"timeline":{"instructions":[
		{"type":"TimelineAddEntries","entries":[
			{"entryId":"tweet-t","content":{"itemContent":{"tweet_results":{"result":{
				"rest_id":"t","legacy":{"full_text":"t"}}}}}}
		]}]}}}}`)

	bookmark := result.Bookmarks[0]
	want := domain.FormatTimestamp(testTime())
	if domain.FormatTimestamp(bookmark.SavedAt) != want {
		t.Errorf("SavedAt = %q, want %q", domain.FormatTimestamp(bookmark.SavedAt), want)
	}
	if domain.FormatTimestamp(bookmark.LastSeenAt) != want {
		t.Errorf("LastSeenAt = %q, want %q", domain.FormatTimestamp(bookmark.LastSeenAt), want)
	}
}

// TestParseRawPayloadCaptureIsOptIn pins the setting's contract: the list must
// render without any raw JSON, so capture is off unless asked for.
func TestParseRawPayloadCaptureIsOptIn(t *testing.T) {
	payload := decode(t, twoBookmarkPage)

	off, err := ParseBookmarkEntries(payload, ParseOptions{SeenAt: testTime()})
	if err != nil {
		t.Fatalf("parse without capture: %v", err)
	}
	if len(off.RawPayloads) != 0 {
		t.Errorf("RawPayloads = %d entries, want none when capture is off", len(off.RawPayloads))
	}

	on, err := ParseBookmarkEntries(payload, ParseOptions{SeenAt: testTime(), CaptureRawPayload: true})
	if err != nil {
		t.Fatalf("parse with capture: %v", err)
	}
	if len(on.RawPayloads) != 2 {
		t.Fatalf("RawPayloads = %d entries, want 2", len(on.RawPayloads))
	}
	raw, ok := on.RawPayloads["123"]
	if !ok {
		t.Fatal("expected a raw payload keyed by tweet id")
	}
	// The captured JSON must round-trip to the same tweet.
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("captured payload is not valid JSON: %v", err)
	}
	if got := decoded["rest_id"]; got != "123" {
		t.Errorf("captured rest_id = %v, want 123", got)
	}
	// Capture must not change what lands in the main record.
	if len(on.Bookmarks) != len(off.Bookmarks) {
		t.Errorf("capture changed the bookmark count: %d vs %d", len(on.Bookmarks), len(off.Bookmarks))
	}
}

// --- helpers ---

func intPtr(v int) *int { return &v }

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func formatIntPtr(v *int) string {
	if v == nil {
		return "nil"
	}
	return strconv.Itoa(*v)
}
