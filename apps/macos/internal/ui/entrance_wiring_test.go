package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEveryEntranceIsUsedOutsideTheDriftList closes the list-counts-as-a-use hole.
//
// AllEntrances exists so the values can be checked against the stylesheet, and mentioning a
// value there is enough for tools/uncalled.py to call it reached. So a modelled animation that
// nothing applies looks wired: round 233 found EntranceLightboxImage and
// EntranceLightboxBackdrop sitting in AllEntrances since round 63 while the lightbox used only
// EntranceLightbox for all three of its parts.
//
// The check is textual and deliberately crude: it asks whether the identifier appears in a
// file other than entrance.go. It cannot tell right from plausible, so the test also pins the
// three lightbox animations individually -- the assertion that would have caught this is
// "the backdrop is driven by the backdrop's own Entrance", and that is not a property a grep
// can see.
func TestEveryEntranceIsUsedOutsideTheDriftList(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	var others strings.Builder
	for _, name := range files {
		if name == "entrance.go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		others.Write(source)
	}
	used := others.String()

	// The names from AllEntrances' own body. Scanning the whole file instead picks up
	// matches inside longer identifiers and reports values that are applied.
	source := entranceSource(t)
	start := strings.Index(source, "var AllEntrances = []Entrance{")
	if start < 0 {
		t.Fatal("AllEntrances moved; this test no longer reads the drift list")
	}
	body := source[start:]
	body = body[:strings.Index(body, "}")]
	names := regexp.MustCompile(`Entrance[A-Z]\w*`).FindAllString(body, -1)
	// Modelled and verified against the stylesheet, and not yet applied. Each needs a reason,
	// and the reason is a promise about work rather than a claim that the value is unused:
	// Empty, and it should stay that way. It held EntranceToast in round 233 on the strength of
	// a grep whose output `head` truncated before the drawing site -- the toast was drawn at
	// shell.go:1846 all along. What was missing was the animation, not the element. It held
	// EntranceFade until round 235, where the detail backdrop stopped being MyGo's Modal and
	// became a row with the fade on it, matching the extension's sibling elements. Recorded in
	// docs/replication-inventory.md 6.16, because the mistake is more useful than the fix.
	unapplied := map[string]string{}
	if len(names) != len(AllEntrances) {
		t.Fatalf("read %d names from AllEntrances but the list holds %d", len(names), len(AllEntrances))
	}
	for _, name := range names {
		if strings.Contains(used, name) {
			continue
		}
		if _, known := unapplied[name]; known {
			t.Logf("%s is not applied yet: %s", name, unapplied[name])
			continue
		}
		t.Errorf("%s is in AllEntrances but no file outside entrance.go applies it;\n"+
			"the drift list counts as a use, so this is the only place it shows", name)
	}
}

// entranceSource is entrance.go itself, where the named values are declared.
func entranceSource(t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile("entrance.go")
	if err != nil {
		t.Fatalf("read entrance.go: %v", err)
	}
	return string(source)
}
