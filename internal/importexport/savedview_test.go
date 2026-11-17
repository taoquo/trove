package importexport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/taoquo/trove/internal/domain"
)

// TestASavedViewFromAnOlderBundleOpensTheLibrary covers the wire side of the archive
// flag.
//
// A bundle written before the archive existed has no onlyArchived field. It must decode
// to the library view those files were saved from, not be refused for a missing field,
// and a view that does live in the archive must carry the flag both ways.
func TestASavedViewFromAnOlderBundleOpensTheLibrary(t *testing.T) {
	const legacy = `{"id":"v","name":"V","query":"rust","sortOrder":"timeline",` +
		`"viewMode":"grid","createdAt":"2026-10-12T12:00:00Z","updatedAt":"2026-10-12T12:00:00Z"}`

	var dto SavedViewDTO
	if err := json.Unmarshal([]byte(legacy), &dto); err != nil {
		t.Fatalf("decode a legacy saved view: %v", err)
	}
	if dto.ToDomain().OnlyArchived {
		t.Error("a saved view from before the archive opened in the archive")
	}

	view := domain.SavedView{
		ID: "a", Name: "Archive", OnlyArchived: true,
		SortOrder: domain.SortTimeline, ViewMode: domain.ViewGrid,
		CreatedAt: dto.ToDomain().CreatedAt, UpdatedAt: dto.ToDomain().UpdatedAt,
	}
	encoded, err := json.Marshal(NewSavedViewDTO(view))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(string(encoded), `"onlyArchived":true`) {
		t.Errorf("the encoded view does not carry the archive: %s", encoded)
	}

	view.OnlyArchived = false
	encoded, err = json.Marshal(NewSavedViewDTO(view))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if strings.Contains(string(encoded), "onlyArchived") {
		t.Errorf("a library view encodes an archive flag: %s", encoded)
	}
}

// TestASavedViewKeepsTheVideoFilter covers the wire side of the video condition: it
// survives a bundle both ways, and a bundle written before it existed decodes to the
// media view those files held rather than to an unknown state.
func TestASavedViewKeepsTheVideoFilter(t *testing.T) {
	const older = `{"id":"v","name":"V","onlyWithMedia":true,"sortOrder":"timeline",` +
		`"viewMode":"grid","createdAt":"2026-10-12T12:00:00Z","updatedAt":"2026-10-12T12:00:00Z"}`
	var dto SavedViewDTO
	if err := json.Unmarshal([]byte(older), &dto); err != nil {
		t.Fatalf("decode an older saved view: %v", err)
	}
	if view := dto.ToDomain(); view.OnlyWithVideo || !view.OnlyWithMedia {
		t.Errorf("an older view decoded as video=%v media=%v, want false/true", view.OnlyWithVideo, view.OnlyWithMedia)
	}

	view := domain.SavedView{
		ID: "v", Name: "Clips", OnlyWithMedia: true, OnlyWithVideo: true,
		SortOrder: domain.SortTimeline, ViewMode: domain.ViewGrid,
		CreatedAt: dto.ToDomain().CreatedAt, UpdatedAt: dto.ToDomain().UpdatedAt,
	}
	encoded, err := json.Marshal(NewSavedViewDTO(view))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	var decoded SavedViewDTO
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if round := decoded.ToDomain(); !round.OnlyWithVideo || !round.OnlyWithMedia {
		t.Errorf("the video filter did not round-trip: video=%v media=%v", round.OnlyWithVideo, round.OnlyWithMedia)
	}

	view.OnlyWithVideo = false
	encoded, err = json.Marshal(NewSavedViewDTO(view))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if strings.Contains(string(encoded), "onlyWithVideo") {
		t.Errorf("a view without the video filter encodes one: %s", encoded)
	}
}
