package domain

import "testing"

// Expectations below were produced by running the TypeScript predicate
// (t.trim().length > 280) on the same inputs, so they pin the port rather than a
// guess at what JavaScript does.
func TestIsLongformMatchesTypeScript(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"exactly 280", repeat("a", 280), false},
		{"281", repeat("a", 281), true},
		{"280 with surrounding spaces", repeat(" ", 50) + repeat("a", 280) + repeat(" ", 50), false},
		{"140 emoji = 280 UTF-16 units", repeat("\U0001F600", 140), false},
		{"141 emoji = 282 UTF-16 units", repeat("\U0001F600", 141), true},
		{"280 CJK", repeat("中", 280), false},
		{"281 CJK", repeat("中", 281), true},
		{"280 padded with NBSP", repeat("\u00a0", 10) + repeat("a", 280) + repeat("\u00a0", 10), false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Bookmark{Text: tc.text}.IsLongform()
			if got != tc.want {
				t.Errorf("IsLongform() = %v, want %v (text len %d)", got, tc.want, len(tc.text))
			}
		})
	}
}

func TestUTF16Length(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"中", 1},
		{"\U0001F600", 2},   // astral: two UTF-16 units
		{"a\U0001F600b", 4}, // 1 + 2 + 1
		{"\u00e9", 1},       // BMP
	}
	for _, tc := range tests {
		if got := utf16Length(tc.in); got != tc.want {
			t.Errorf("utf16Length(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestBookmarkHasMedia(t *testing.T) {
	if (Bookmark{}).HasMedia() {
		t.Error("a bookmark with no attachments must not report media")
	}
	if !(Bookmark{Media: []Media{{Type: MediaPhoto, URL: "u"}}}).HasMedia() {
		t.Error("a bookmark with one attachment must report media")
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
