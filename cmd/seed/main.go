// Command seed fills a fresh database from a fixture JSON.
//
// It is a development tool, not part of the app: it writes only to the directory it is given,
// and refuses to touch a database that already has bookmarks. The site's screenshots are taken
// from a seeded directory (testdata/site-fixture.json) so no real account or post appears in
// them.
//
//	go run ./cmd/seed -fixture testdata/site-fixture.json -data-dir /tmp/trove-site
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/taoquo/trove/internal/domain"
	"github.com/taoquo/trove/internal/media"
	"github.com/taoquo/trove/internal/storage/sqlite"
)

const databaseFile = "trove.db"

type mediaFixture struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	// PosterURL is a video's still frame. Without it a seeded video has no picture at all:
	// MediaThumbnailURL stands the poster in for a video, so a video with no poster draws no
	// card media and no play mark, which is not what the app does with real data.
	PosterURL string `json:"posterUrl"`
	AltText   string `json:"altText"`
}

// metricsFixture is the fixture's engagement block. The pointers keep the fixture's own
// distinction between "absent" and "zero", which the card draws differently: a nil count draws
// no glyph at all, a zero draws the glyph with a 0 beside it.
type metricsFixture struct {
	Likes    *int `json:"likes"`
	Retweets *int `json:"retweets"`
	Replies  *int `json:"replies"`
}

type bookmarkFixture struct {
	TweetID      string          `json:"tweetId"`
	TweetURL     string          `json:"tweetUrl"`
	AuthorName   string          `json:"authorName"`
	AuthorHandle string          `json:"authorHandle"`
	Text         string          `json:"text"`
	CreatedAtOnX string          `json:"createdAtOnX"`
	SavedAt      string          `json:"savedAt"`
	Rank         int             `json:"bookmarkTimelineRank"`
	Media        []mediaFixture  `json:"media"`
	Metrics      *metricsFixture `json:"metrics"`
}

type tagFixture struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type relationFixture struct {
	BookmarkID string `json:"bookmarkId"`
	TagID      string `json:"tagId"`
}

type suggestionFixture struct {
	ID         string  `json:"id"`
	BookmarkID string  `json:"bookmarkId"`
	TagName    string  `json:"tagName"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
	Source     string  `json:"source"`
	CreatedAt  string  `json:"createdAt"`
}

type syncRunFixture struct {
	ID           string `json:"id"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	Status       string `json:"status"`
	FetchedCount int    `json:"fetchedCount"`
}

type settingsFixture struct {
	Locale            string  `json:"locale"`
	LeftSidebarWidth  float32 `json:"leftSidebarWidth"`
	RightSidebarWidth float32 `json:"rightSidebarWidth"`
}

type fixture struct {
	Settings       settingsFixture     `json:"settings"`
	Bookmarks      []bookmarkFixture   `json:"bookmarks"`
	Tags           []tagFixture        `json:"tags"`
	BookmarkTags   []relationFixture   `json:"bookmarkTags"`
	SyncRuns       []syncRunFixture    `json:"syncRuns"`
	TagSuggestions []suggestionFixture `json:"tagSuggestions"`
}

func main() {
	fixturePath := flag.String("fixture", "testdata/site-fixture.json", "fixture JSON")
	dataDir := flag.String("data-dir", "", "directory that will hold trove.db")
	mediaBase := flag.String("media-base", "http://127.0.0.1:8799", "prefix for fixture media paths")
	warmMedia := flag.Bool("warm-media", false, "fetch the fixture's pictures into the cache the app reads")
	locale := flag.String("locale", "", "override the fixture's locale (en, zh-CN)")
	flag.Parse()

	if *dataDir == "" {
		fmt.Fprintln(os.Stderr, "seed: -data-dir is required")
		os.Exit(2)
	}

	raw, err := os.ReadFile(*fixturePath)
	if err != nil {
		fail("read fixture: %v", err)
	}
	var data fixture
	if err := json.Unmarshal(raw, &data); err != nil {
		fail("parse fixture: %v", err)
	}
	if *locale != "" {
		data.Settings.Locale = *locale
	}

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		fail("create data dir: %v", err)
	}
	db, err := sqlite.Open(filepath.Join(*dataDir, databaseFile))
	if err != nil {
		fail("open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	bookmarks := sqlite.NewBookmarkRepository(db)
	existing, err := bookmarks.Count(ctx, domain.LibraryFilter{})
	if err != nil {
		fail("count bookmarks: %v", err)
	}
	if existing > 0 {
		fail("database already holds %d bookmarks; seed a fresh directory", existing)
	}

	rows := make([]domain.Bookmark, 0, len(data.Bookmarks))
	for _, item := range data.Bookmarks {
		savedAt, err := time.Parse(time.RFC3339, item.SavedAt)
		if err != nil {
			fail("bookmark %s savedAt: %v", item.TweetID, err)
		}
		rank := item.Rank
		bookmark := domain.Bookmark{
			TweetID:      item.TweetID,
			TweetURL:     item.TweetURL,
			AuthorName:   item.AuthorName,
			AuthorHandle: item.AuthorHandle,
			Text:         item.Text,
			CreatedAtOnX: item.CreatedAtOnX,
			SavedAt:      savedAt,
			TimelineRank: &rank,
			SourceKind:   domain.SourceBookmark,
			Hashtags:     []string{},
		}
		// The fixture carries the same 9/24/128 the source render shows, and the card's stat row
		// is drawn from these counts. Dropping them here made the native cards 20 points shorter
		// than the reference's with an empty footer -- a gap in the *seeder*, not in the card.
		if item.Metrics != nil {
			bookmark.Metrics = &domain.Metrics{
				Likes:    item.Metrics.Likes,
				Retweets: item.Metrics.Retweets,
				Replies:  item.Metrics.Replies,
			}
		}
		for _, media := range item.Media {
			entry := domain.Media{
				Type:    domain.MediaType(media.Type),
				URL:     *mediaBase + media.URL,
				AltText: media.AltText,
			}
			// A pointer, not a string: the domain tells "a photo, so no poster" (nil) from "a
			// video whose poster URL is empty" (set and empty), and the seeder keeps that
			// distinction rather than flattening both to "".
			if media.PosterURL != "" {
				poster := *mediaBase + media.PosterURL
				entry.PosterURL = &poster
			}
			bookmark.Media = append(bookmark.Media, entry)
		}
		rows = append(rows, bookmark)
	}
	if _, err := bookmarks.UpsertMany(ctx, rows); err != nil {
		fail("insert bookmarks: %v", err)
	}

	tags := sqlite.NewTagRepository(db)
	for _, item := range data.Tags {
		if err := tags.Create(ctx, domain.Tag{
			ID:           item.ID,
			Name:         item.Name,
			CanonicalKey: domain.CanonicalTagKey(item.Name),
			CreatedAt:    time.Now().UTC().Format(time.RFC3339),
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			fail("create tag %s: %v", item.ID, err)
		}
	}

	relations := make([]domain.BookmarkTag, 0, len(data.BookmarkTags))
	for _, item := range data.BookmarkTags {
		relations = append(relations, domain.BookmarkTag{
			BookmarkID: item.BookmarkID,
			TagID:      item.TagID,
			CreatedAt:  time.Now().UTC(),
			Source:     domain.TagSourceManual,
		})
	}
	if len(relations) > 0 {
		if err := tags.AttachMany(ctx, relations); err != nil {
			fail("attach tags: %v", err)
		}
	}

	suggestions := make([]domain.TagSuggestion, 0, len(data.TagSuggestions))
	for _, item := range data.TagSuggestions {
		createdAt, _ := time.Parse(time.RFC3339, item.CreatedAt)
		suggestions = append(suggestions, domain.TagSuggestion{
			ID:         domain.SuggestionID(item.BookmarkID, item.TagName),
			BookmarkID: item.BookmarkID,
			TagName:    item.TagName,
			Source:     item.Source,
			Confidence: item.Confidence,
			Status:     item.Status,
			CreatedAt:  createdAt,
		})
	}
	if len(suggestions) > 0 {
		if _, err := sqlite.NewSuggestionRepository(db).InsertMany(ctx, suggestions); err != nil {
			fail("insert suggestions: %v", err)
		}
	}

	runs := sqlite.NewSyncRunRepository(db)
	for _, item := range data.SyncRuns {
		startedAt, _ := time.Parse(time.RFC3339, item.StartedAt)
		finishedAt, _ := time.Parse(time.RFC3339, item.FinishedAt)
		run := domain.SyncRun{
			ID:           item.ID,
			Status:       item.Status,
			StartedAt:    startedAt,
			FinishedAt:   &finishedAt,
			FetchedCount: item.FetchedCount,
		}
		if err := runs.Start(ctx, run); err != nil {
			fail("start sync run: %v", err)
		}
		if err := runs.Finish(ctx, run); err != nil {
			fail("finish sync run: %v", err)
		}
	}

	// The headline result lives in its own settings key, not in the run table: the workspace
	// reads it for "last sync" and "not synced yet", so a seeded run without it looks like a
	// library that arrived from nowhere.
	if len(data.SyncRuns) > 0 {
		last := data.SyncRuns[len(data.SyncRuns)-1]
		finishedAt, err := time.Parse(time.RFC3339, last.FinishedAt)
		if err != nil {
			fail("run %s finishedAt: %v", last.ID, err)
		}
		summary := domain.SyncSummary{
			Status:       last.Status,
			FetchedCount: last.FetchedCount,
			LastSyncedAt: &finishedAt,
		}
		encoded, err := json.Marshal(summary)
		if err != nil {
			fail("encode summary: %v", err)
		}
		settings := sqlite.NewSettingsRepository(db)
		if err := settings.Set(ctx, "sync.summary", string(encoded), time.Now().UTC()); err != nil {
			fail("write sync summary: %v", err)
		}
		if err := settings.Set(ctx, "sync.hasCompletedInitialFullSync", "true", time.Now().UTC()); err != nil {
			fail("write full-sync flag: %v", err)
		}
	}

	// The preferences blob is what makes the native capture use the same language,
	// theme and sidebar width as the source render. Without it the two pictures are
	// a Chinese page beside an English one and every Δ becomes "they differ".
	preferences := map[string]any{
		"appearance": "light",
		"locale":     data.Settings.Locale,
		"layout": map[string]any{
			"leftSidebarWidth":  data.Settings.LeftSidebarWidth,
			"rightSidebarWidth": data.Settings.RightSidebarWidth,
		},
		"viewMode": "grid",
	}
	blob, err := json.Marshal(preferences)
	if err != nil {
		fail("encode preferences: %v", err)
	}
	if err := sqlite.NewSettingsRepository(db).Set(ctx, "app.preferences", string(blob), time.Now().UTC()); err != nil {
		fail("write preferences: %v", err)
	}

	// The headless capture is a single ui.Render: a picture whose load starts while it draws
	// has no second frame to appear on. So the fixture's pictures are fetched here, through
	// the same fetcher and disk cache the app uses, before the picture is taken. Without this
	// every media card was photographed as its placeholder with a select toggle floating in it.
	if *warmMedia {
		fetcher := media.NewFetcher()
		fetcher.UseDiskCache(media.NewDiskCache(filepath.Join(*dataDir, mediaCacheDir), media.DefaultDiskBudget))
		warmed := 0
		for _, row := range rows {
			if url := domain.MediaThumbnailURL(row.Media); url != "" {
				loadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				_, err := fetcher.Load(loadCtx, url)
				cancel()
				if err != nil {
					fail("warm media %s: %v", url, err)
				}
				warmed++
			}
		}
		fmt.Printf("warmed %d pictures into %s\n", warmed, filepath.Join(*dataDir, mediaCacheDir))
	}

	fmt.Printf("seeded %d bookmarks, %d tags, %d relations, %d suggestions into %s\n",
		len(rows), len(data.Tags), len(relations), len(suggestions), *dataDir)
}

// mediaCacheDir is where the app keeps fetched pictures, relative to the data directory.
const mediaCacheDir = "media-cache"

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "seed: "+format+"\n", args...)
	os.Exit(1)
}
