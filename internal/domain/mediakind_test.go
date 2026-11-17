package domain

import "testing"

// TestSummarizeMediaKindPicksTheMostDemandingAttachment covers the rule the video
// filter depends on: a post is described by the attachment that asks the most of
// the reader, so a video is never hidden behind the photo attached beside it.
func TestSummarizeMediaKindPicksTheMostDemandingAttachment(t *testing.T) {
	photo := Media{Type: MediaPhoto, URL: "photo"}
	video := Media{Type: MediaVideo, URL: "video"}
	gif := Media{Type: MediaAnimatedGif, URL: "gif"}
	unknown := Media{Type: "spaces", URL: "space"}

	tests := []struct {
		name  string
		media []Media
		want  MediaType
	}{
		{"no attachments", nil, ""},
		{"one picture", []Media{photo}, MediaPhoto},
		{"a video alone", []Media{video}, MediaVideo},
		{"an animation", []Media{gif}, MediaAnimatedGif},
		// The order in the payload must not decide: a video carries a poster image,
		// so the photo routinely comes first.
		{"a photo before a video", []Media{photo, video}, MediaVideo},
		{"a video before a photo", []Media{video, photo}, MediaVideo},
		{"a photo before an animation", []Media{photo, gif}, MediaAnimatedGif},
		// An attachment this build does not know is still media, and a known kind
		// beside it still says more.
		{"an unknown attachment", []Media{unknown}, MediaKindOther},
		{"an unknown beside a video", []Media{unknown, video}, MediaVideo},
		{"a picture beside an unknown", []Media{photo, unknown}, MediaPhoto},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SummarizeMediaKind(tc.media); got != tc.want {
				t.Errorf("SummarizeMediaKind = %q, want %q", got, tc.want)
			}
		})
	}
}
