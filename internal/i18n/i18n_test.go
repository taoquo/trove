package i18n

import (
	"regexp"
	"strings"
	"testing"
)

// TestEveryLocaleTranslatesEveryKey is the point of the catalog: a missing string
// must be a test failure rather than something a user finds by seeing the wrong
// language.
func TestEveryLocaleTranslatesEveryKey(t *testing.T) {
	for _, locale := range AllLocales {
		if missing := Missing(locale); len(missing) > 0 {
			t.Errorf("locale %q is missing %d key(s): %v", locale, len(missing), missing)
		}
	}
}

// TestNoTranslationIsEmpty catches the other half: a key that exists but was left
// blank renders as nothing, which looks like a layout bug.
func TestNoTranslationIsEmpty(t *testing.T) {
	for _, locale := range AllLocales {
		translator := New(locale)
		for _, key := range Keys() {
			if text := translator.T(key); text == "" {
				t.Errorf("locale %q renders %q as empty", locale, key)
			}
		}
	}
}

// TestNoTranslationIsStillAKey catches a key that was never added to English and
// so would be shown raw.
func TestNoTranslationIsStillAKey(t *testing.T) {
	translator := New(EN)

	for _, key := range Keys() {
		if text := translator.T(key); text == key {
			t.Errorf("key %q is not translated in the fallback catalog", key)
		}
	}
}

func TestChineseAndEnglishDiffer(t *testing.T) {
	zh := New(ZHCN)
	en := New(EN)

	// If these matched, one catalog would be a copy of the other. AppName is not
	// sampled: "trove" is the product's name in its own lower case, and names do not
	// translate.
	for _, key := range []Key{ActionCancel, SidebarAllBookmarks, EmptyLibrary, ActionDelete} {
		if zh.T(key) == en.T(key) {
			t.Errorf("key %q is identical in both locales (%q)", key, zh.T(key))
		}
	}
}

// TestFallbackIsEnglish covers a half-translated locale: it must degrade to
// readable text, never to a raw key.
func TestFallbackIsEnglish(t *testing.T) {
	translator := &Translator{locale: "de"}

	if got := translator.T(AppName); got != catalogs[EN][AppName] {
		t.Errorf("an unknown locale rendered %q, want the English fallback", got)
	}
}

func TestTfInterpolates(t *testing.T) {
	translator := New(EN)

	// The count is a bare number, because the number and the word sit on separate lines: the
	// visual reference reads "9300" over "结果", not "9300 结果" on one line.
	if got := translator.Tf(ToolbarCount, 42); got != "42" {
		t.Errorf("Tf = %q", got)
	}
	if got := New(ZHCN).Tf(ToolbarShowingOf, 100, 4321); got != "100 / 4321" {
		t.Errorf("Tf = %q", got)
	}

	// A string that carries a word and a verb still interpolates into the word.
	if got := translator.Tf(AutoTagAt, "Oct 12, 8:00 PM"); got != "Recorded Oct 12, 8:00 PM" {
		t.Errorf("Tf = %q", got)
	}
}

func TestResolveLocale(t *testing.T) {
	tests := []struct {
		name      string
		preferred string
		want      Locale
	}{
		{"a stored Chinese preference wins", ZHCN, ZHCN},
		{"a stored English preference wins", EN, EN},
		// Nothing stored is the fresh install, and it is English whatever the machine is set
		// to: the language is chosen inside the app, not by the OS.
		{"nothing stored is English", "", EN},
		{"an unsupported preference falls back to English", "fr", EN},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveLocale(tc.preferred); got != tc.want {
				t.Errorf("ResolveLocale(%q) = %q, want %q", tc.preferred, got, tc.want)
			}
		})
	}
}

func TestValidLocale(t *testing.T) {
	for _, locale := range []string{"zh-CN", "en"} {
		if !ValidLocale(locale) {
			t.Errorf("%q must be valid", locale)
		}
	}
	for _, locale := range []string{"", "zh", "en-US", "ZH-CN"} {
		if ValidLocale(locale) {
			t.Errorf("%q must not be valid", locale)
		}
	}
}

// TestAssertCompletePasses pins the startup guard: it must not refuse to launch a
// correctly built app.
func TestAssertCompletePasses(t *testing.T) {
	if err := AssertComplete(); err != nil {
		t.Errorf("AssertComplete must pass for a complete catalog: %v", err)
	}
}

// TestAssertCompleteNamesTheMissingKey covers the failure it exists for: a hole in
// a catalog must be reported with the key, not as a bare failure.
func TestAssertCompleteNamesTheMissingKey(t *testing.T) {
	// A locale with nothing translated stands in for a catalog with a hole.
	saved, present := catalogs["xx-TEST"]
	catalogs["xx-TEST"] = map[Key]string{}
	AllLocales = append(AllLocales, "xx-TEST")

	t.Cleanup(func() {
		if present {
			catalogs["xx-TEST"] = saved
		} else {
			delete(catalogs, "xx-TEST")
		}
		AllLocales = AllLocales[:len(AllLocales)-1]
	})

	err := AssertComplete()
	if err == nil {
		t.Fatal("an incomplete catalog must be reported")
	}
	if !strings.Contains(err.Error(), "xx-TEST") {
		t.Errorf("the error must name the locale: %v", err)
	}
	// It must name an actual key, so the hole can be found.
	if !strings.Contains(err.Error(), string(AppName)) {
		t.Errorf("the error must name a missing key: %v", err)
	}
}

// TestNoEnglishIsLeftInTheChineseCatalogue is the machine check for the class this was
// found by.
//
// The Chinese sort label read "Recently saved" while its neighbours read 最近保存: the
// English value had been copied into the zh table for one key. A screenshot caught it and
// nothing else would have -- both catalogues were complete, so every tool that counts
// translations was satisfied.
//
// A value that is legitimately the same in both languages is a proper noun or a symbol,
// and those are listed here rather than inferred.
func TestNoEnglishIsLeftInTheChineseCatalogue(t *testing.T) {
	// Every value that is legitimately the same in both tables, with its reason: a brand
	// name, a proper noun, a symbol, and one string that is typed rather than read.
	allowed := map[Key]bool{
		AppName:         true, // "trove" -- the product's name, lowercase in every language
		SettingsAPIKey:  true, // "API key" -- kept in English in the Chinese table too
		CreateTagSymbol: true, // "+" is a symbol, not a word
		// The paste example is literally the two cookie names and the separators
		// between them. There is nothing in it to translate: a reader has to type
		// exactly this, and the field takes the same text in every language.
		SettingsXCredentialPastePlaceholder: true,
	}
	for key, english := range catalogs[EN] {
		chinese, ok := catalogs[ZHCN][key]
		if !ok {
			t.Errorf("%s has no Chinese translation", key)
			continue
		}
		if english == chinese && !allowed[key] {
			// A value with no letters at all cannot be English: it is placeholders and
			// punctuation, and both tables have to spell it the same way. The result count is
			// one of these -- "%d" -- and an earlier version of this test called it a
			// copy-paste. The exemption is inferred rather than listed, because a new
			// format-only string would
			// otherwise need an allowlist entry every time, and an allowlist that grows to fit
			// is how a check stops checking.
			// Strip the format verbs first: "%d" contains a d, so a bare letter scan says it
			// is English. What matters is whether anything is left that could be a word.
			if !hasLetters(formatVerb.ReplaceAllString(english, "")) {
				continue
			}
			t.Errorf("%s reads %q in both languages: the English value was copied into the "+
				"Chinese table", key, english)
		}
	}
}

// formatVerb matches one printf verb, so a placeholder can be removed before asking whether a
// string still holds any words.
var formatVerb = regexp.MustCompile(`%[-+ #0-9.]*[a-zA-Z]`)

func hasLetters(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	})
}
