package importexport

// The large-library check.
//
// Every other import test uses one or two rows, because those tests are about the mapping
// between the two formats. The acceptance list asks a different question -- "重复同步与重复导入
// 不产生重复 tweet / tag relation" at the size a real library reaches -- and the answer depends
// on the merge path doing its existence checks in bulk rather than one query per row. A
// 9,300-row backup is the size the project's own benchmarks use for "a real library".

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/taoquo/trove/apps/macos/internal/domain"
)

func TestALargeFlatBackupImportsAndMergesIdempotently(t *testing.T) {
	service, _, repos := newService(t)
	ctx := context.Background()

	const (
		bookmarkCount = 9300
		tagCount      = 40
	)
	tags := make([]map[string]any, 0, tagCount)
	for i := 0; i < tagCount; i++ {
		name := fmt.Sprintf("tag-%02d", i)
		tags = append(tags, map[string]any{
			"id": fmt.Sprintf("t%d", i), "name": name, "canonicalKey": name,
			"createdAt": "2026-10-01T00:00:00.000Z", "updatedAt": "2026-10-01T00:00:00.000Z",
		})
	}
	bookmarks := make([]map[string]any, 0, bookmarkCount)
	relations := make([]map[string]any, 0, bookmarkCount/2)
	for i := 0; i < bookmarkCount; i++ {
		id := fmt.Sprintf("b%05d", i)
		bookmarks = append(bookmarks, map[string]any{
			"tweetId": id, "tweetUrl": "https://x.com/author" + id + "/status/" + id,
			"authorName": "Author " + id, "authorHandle": "author" + id,
			"text":         "bookmark number " + id + " about compilers and software",
			"savedAt":      "2026-10-12T12:00:00.000Z",
			"createdAtOnX": "Wed Oct 10 20:19:24 +0000 2018",
		})
		if i%2 == 0 {
			relations = append(relations, map[string]any{
				"bookmarkId": id, "tagId": fmt.Sprintf("t%d", i%tagCount),
				"createdAt": "2026-10-12T12:00:00.000Z",
			})
		}
	}
	payload := map[string]any{
		"exportVersion": 2,
		"schemaVersion": 9,
		"exportedAt":    "2026-10-12T14:22:31.000Z",
		"bookmarks":     bookmarks,
		"tags":          tags,
		"bookmarkTags":  relations,
		"settings":      map[string]any{"savedViews": []any{}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	preview, err := service.Preview(ctx, raw)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !preview.Legacy {
		t.Error("a version 2 flat backup must take the legacy path")
	}

	started := time.Now()
	report, err := service.Import(ctx, raw)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if report.BookmarksInserted != bookmarkCount {
		t.Errorf("inserted %d bookmarks, want %d", report.BookmarksInserted, bookmarkCount)
	}
	if report.TagsInserted != tagCount {
		t.Errorf("inserted %d tags, want %d", report.TagsInserted, tagCount)
	}
	// The relation count is the one that catches a per-row existence query gone wrong: half the
	// bookmarks carry one, and a merge that inserted a row per relation twice would report the
	// same number twice while the table doubled.
	if report.RelationsAdded != len(relations) {
		t.Errorf("added %d relations, want %d", report.RelationsAdded, len(relations))
	}
	stored, err := repos.Bookmarks.Count(ctx, domain.LibraryFilter{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if stored != bookmarkCount {
		t.Errorf("the table holds %d bookmarks, want %d", stored, bookmarkCount)
	}
	relationsStored, err := repos.Tags.AllRelations(ctx)
	if err != nil {
		t.Fatalf("AllRelations: %v", err)
	}
	if len(relationsStored) != len(relations) {
		t.Errorf("the table holds %d relations, want %d", len(relationsStored), len(relations))
	}
	t.Logf("imported %d bookmarks, %d tags and %d relations in %s",
		report.BookmarksInserted, report.TagsInserted, len(relations), elapsed)

	// The second import is the acceptance item: nothing new is inserted, and the counts stand.
	second, err := service.Import(ctx, raw)
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if second.BookmarksInserted != 0 || second.TagsInserted != 0 || second.RelationsAdded != 0 {
		t.Errorf("the second import inserted %d bookmarks, %d tags and %d relations, want none",
			second.BookmarksInserted, second.TagsInserted, second.RelationsAdded)
	}
	again, err := repos.Bookmarks.Count(ctx, domain.LibraryFilter{})
	if err != nil {
		t.Fatalf("Count after the second import: %v", err)
	}
	if again != bookmarkCount {
		t.Errorf("the table holds %d bookmarks after the second import, want %d",
			again, bookmarkCount)
	}
}
