// Package i18n holds the interface's text.
//
// Every user-visible string goes through here rather than being written inline, so
// a missing translation is a compile-time key and a test failure rather than a
// stray English word in a Chinese interface.
package i18n

import (
	"fmt"
	"sort"
	"strings"
)

// Locale is a language tag.
type Locale = string

const (
	// ZHCN is Simplified Chinese.
	ZHCN Locale = "zh-CN"
	// EN is English, and the fallback for anything untranslated.
	EN Locale = "en"
)

// AllLocales is the supported set, in display order.
var AllLocales = []Locale{ZHCN, EN}

// DefaultLocale is what a fresh install uses, and it is English.
//
// It used to follow the system locale when that was supported, which made the app's first
// screen depend on whose Mac installed it. This is an English-first app shipped to both
// audiences: the language is a setting inside it, and it starts in English until someone says
// otherwise. The strings are the same either way; only the guess about the reader's language
// has gone.
const DefaultLocale = EN

// ValidLocale reports whether a stored preference is supported.
func ValidLocale(locale string) bool {
	for _, candidate := range AllLocales {
		if locale == candidate {
			return true
		}
	}
	return false
}

// ResolveLocale picks the locale to use from the stored preference.
//
// An empty or unsupported preference is English. Nothing consults the OS: a stored "zh-CN" is
// the only thing that makes the interface Chinese.
func ResolveLocale(preferred string) Locale {
	if ValidLocale(preferred) {
		return preferred
	}
	return DefaultLocale
}

// Key identifies a message.
type Key = string

// messages is one domain's strings, per locale.
type messages map[Locale]map[Key]string

// catalogs holds every translation, merged at init from the per-domain files.
//
// Each domain file keeps a key beside its strings: a constant and its sentence used to live
// hundreds of lines apart in one table, and the two drifted. English is complete by
// construction: a key missing from another catalog falls back to it, so a half-translated
// locale degrades to English rather than to a raw key.
var catalogs = mergeCatalogs(shellMessages, libraryMessages, settingsMessages, syncMessages)

// mergeCatalogs joins the domain tables into the one map the Translator reads.
//
// The type system cannot see a key that two files both translate, and the later one would
// silently win. The panic happens during package initialisation, before a window is drawn, so
// a duplicate is a build failure in every test and every launch rather than a sentence that
// changes depending on which file the linker listed first.
func mergeCatalogs(groups ...messages) map[Locale]map[Key]string {
	merged := map[Locale]map[Key]string{}
	for _, group := range groups {
		for locale, entries := range group {
			if merged[locale] == nil {
				merged[locale] = map[Key]string{}
			}
			for key, text := range entries {
				if previous, duplicate := merged[locale][key]; duplicate {
					panic("i18n: " + key + " is translated twice for " + locale +
						": " + previous + " / " + text)
				}
				merged[locale][key] = text
			}
		}
	}
	return merged
}

// Translator renders messages for one locale.
type Translator struct {
	locale Locale
}

// New builds a translator for a stored preference.
func New(preferred string) *Translator {
	return &Translator{locale: ResolveLocale(preferred)}
}

// Locale returns the resolved locale.
func (t *Translator) Locale() Locale { return t.locale }

// T renders a message.
//
// A key with no translation falls back to English, and a key in neither returns
// the key itself: showing the key makes a missing string obvious in a screenshot,
// where an empty label would look like a layout bug.
func (t *Translator) T(key Key) string {
	if catalog, ok := catalogs[t.locale]; ok {
		if text, ok := catalog[key]; ok {
			return text
		}
	}
	if text, ok := catalogs[EN][key]; ok {
		return text
	}
	return key
}

// Tf renders a message with arguments.
func (t *Translator) Tf(key Key, args ...any) string {
	return fmt.Sprintf(t.T(key), args...)
}

// Keys is every key any catalogue holds, sorted.
//
// It used to read the English table alone, which made a key that existed only in Chinese
// invisible to Missing: SettingsRuleArrow was missing from English for the whole life of the
// build, and the first English screenshot of the rule editor printed the key's own name. A
// union is the only version that can notice a hole in the table it is reading from.
func Keys() []Key {
	seen := make(map[Key]bool, len(catalogs[EN]))
	for _, catalog := range catalogs {
		for key := range catalog {
			seen[key] = true
		}
	}
	keys := make([]Key, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// Missing returns the keys a locale does not translate.
//
// It exists so a half-finished translation is a test failure rather than something
// a user discovers by seeing English in a Chinese interface.
func Missing(locale Locale) []Key {
	catalog, ok := catalogs[locale]
	if !ok {
		return Keys()
	}

	var missing []Key
	for _, key := range Keys() {
		if _, present := catalog[key]; !present {
			missing = append(missing, key)
		}
	}
	return missing
}

// AssertComplete reports a locale that does not translate every key.
//
// The composition root calls it at startup. A missing string is a build defect
// rather than a runtime condition, and the interface is rendered from these
// catalogs: a hole shows a raw key to the user, which is worse than refusing to
// start with a message naming the key.
func AssertComplete() error {
	for _, locale := range AllLocales {
		if missing := Missing(locale); len(missing) > 0 {
			sort.Strings(missing)
			return fmt.Errorf("i18n: locale %q is missing %d key(s): %s",
				locale, len(missing), strings.Join(missing, ", "))
		}
	}
	return nil
}
