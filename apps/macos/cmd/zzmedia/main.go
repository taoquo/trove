// zzmedia fetches the media URLs stored in the real database and reports which fail.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"github.com/taoquo/trove/apps/macos/internal/media"
)

func main() {
	dbPath := os.Args[1]
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	rows, err := db.Query(`select tweet_id, media_json, author_avatar_url from bookmarks`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	fetcher := media.NewFetcher()
	ctx := context.Background()

	urls := map[string]string{} // url -> tweet
	for rows.Next() {
		var id string
		var mediaJSON, avatar sql.NullString
		if err := rows.Scan(&id, &mediaJSON, &avatar); err != nil {
			panic(err)
		}
		_ = mediaJSON
		if avatar.Valid && avatar.String != "" {
			urls[avatar.String] = id + " avatar"
		}
	}

	fmt.Printf("trying %d distinct avatar urls\n", len(urls))
	ok, failed := 0, 0
	for url, who := range urls {
		started := time.Now()
		_, err := fetcher.Load(ctx, url)
		if err != nil {
			failed++
			fmt.Printf("  FAIL %-40s %v\n", who, err)
			continue
		}
		ok++
		if ok <= 3 {
			fmt.Printf("  ok   %-40s %s\n", who, time.Since(started).Round(time.Millisecond))
		}
	}
	fmt.Printf("avatars: %d ok, %d failed\n", ok, failed)
}
