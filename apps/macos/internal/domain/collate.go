package domain

import (
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// CompareNaturalNames orders display names the way the extension does.
//
// The extension sorts through `Intl.Collator(undefined, {numeric: true,
// sensitivity: "base"})`, which is three behaviours at once and each one matters:
//
//   - numeric: "Tag 2" sorts before "Tag 10", not after it, which is the difference
//     between a tag list a person can scan and one that looks shuffled.
//   - sensitivity "base": case and accents do not distinguish, so "Go" and "go" are
//     equal rather than adjacent.
//   - It is a collation, not a byte compare, so "apple" precedes "Banana".
//
// A plain string comparison gets all three wrong. This reproduces them; the fixture
// test beside it checks the result against the real collator rather than against my
// reading of it.
func CompareNaturalNames(left, right string) int {
	leftParts := splitNatural(foldForCollation(left))
	rightParts := splitNatural(foldForCollation(right))

	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		l, r := leftParts[index], rightParts[index]

		// A different class decides the order outright: the collator ranks the classes
		// before it ever looks at the characters.
		if l.class != r.class {
			if l.class < r.class {
				return -1
			}
			return 1
		}

		switch l.class {
		case classDigit:
			if comparison := compareNumeric(l.text, r.text); comparison != 0 {
				return comparison
			}
		case classPunctuation:
			if comparison := comparePunctuation(l.text, r.text); comparison != 0 {
				return comparison
			}
		default:
			if l.text != r.text {
				if l.text < r.text {
					return -1
				}
				return 1
			}
		}
	}

	// One is a prefix of the other, so the shorter sorts first.
	switch {
	case len(leftParts) < len(rightParts):
		return -1
	case len(leftParts) > len(rightParts):
		return 1
	default:
		return 0
	}
}

// Collation classes, in the order the collator ranks them.
//
// ICU's root collation puts the ignorable characters first, then digits, then
// letters. A comparison that only knows "digit or not" gets "01" against ".NET"
// backwards, which is why the class is explicit rather than inferred.
const (
	classSpace = iota
	classPunctuation
	classDigit
	classLetter
	classOther
)

// charClass ranks a rune for collation.
func charClass(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return classSpace
	case unicode.IsDigit(r):
		return classDigit
	case unicode.IsLetter(r):
		return classLetter
	case unicode.IsPunct(r), unicode.IsSymbol(r):
		return classPunctuation
	default:
		return classOther
	}
}

// naturalPart is one run of the input: characters of a single class.
type naturalPart struct {
	text  string
	class int
}

// splitNatural separates the input into runs of one class each.
func splitNatural(value string) []naturalPart {
	parts := make([]naturalPart, 0, 8)
	var current strings.Builder
	currentClass := 0
	started := false

	for _, r := range value {
		class := charClass(r)
		switch {
		case !started:
			currentClass = class
			started = true
		case class != currentClass:
			// The run changed class, so the previous one ends and this one begins.
			// Updating currentClass here is what lets the next characters join the new
			// run instead of ending it again on every one.
			parts = append(parts, naturalPart{text: current.String(), class: currentClass})
			current.Reset()
			currentClass = class
		}
		current.WriteRune(r)
	}
	if started {
		parts = append(parts, naturalPart{text: current.String(), class: currentClass})
	}

	return parts
}

// punctuationOrder is the collator's ranking of the punctuation that tag names
// actually contain, taken from the collator itself rather than from code points.
//
// Code-point order is wrong here: it puts "-" (0x2D) before "_" (0x5F), while the
// collator puts "_" first. That single difference is what the fixture caught.
var punctuationOrder = map[rune]int{
	'_': 0, '-': 1, ',': 2, ';': 3, ':': 4, '!': 5, '?': 6, '.': 7,
	'\'': 8, '"': 9, '(': 10, ')': 11, '[': 12, ']': 13, '{': 14, '}': 15,
	'@': 16, '*': 17, '/': 18, '\\': 19, '&': 20, '#': 21, '%': 22, '`': 23,
	'^': 24, '+': 25, '<': 26, '=': 27, '>': 28, '|': 29, '~': 30, '$': 31,
}

// UnknownPunctuation is where punctuation not in the table ranks: after everything
// the collator was measured on.
const unknownPunctuation = 1000

// comparePunctuation compares two punctuation runs.
func comparePunctuation(left, right string) int {
	leftRunes, rightRunes := []rune(left), []rune(right)

	for index := 0; index < len(leftRunes) && index < len(rightRunes); index++ {
		l, r := rankPunctuation(leftRunes[index]), rankPunctuation(rightRunes[index])
		if l != r {
			if l < r {
				return -1
			}
			return 1
		}
	}

	switch {
	case len(leftRunes) < len(rightRunes):
		return -1
	case len(leftRunes) > len(rightRunes):
		return 1
	default:
		return 0
	}
}

// rankPunctuation gives a rune its collation rank.
func rankPunctuation(r rune) int {
	if rank, known := punctuationOrder[r]; known {
		return rank
	}
	// Anything unmeasured falls after the measured set, ordered by code point, so
	// the comparison stays total.
	return unknownPunctuation + int(r)
}

// compareNumeric compares two digit runs by value.
//
// Leading zeros are ignored, and a difference only in them is not a difference at
// all: the collator reports "01" and "001" as equal, so treating them as ordered
// would show the user two distinct tags the sorter calls the same.
func compareNumeric(left, right string) int {
	trimmedLeft := strings.TrimLeft(left, "0")
	trimmedRight := strings.TrimLeft(right, "0")

	switch {
	case len(trimmedLeft) != len(trimmedRight):
		if len(trimmedLeft) < len(trimmedRight) {
			return -1
		}
		return 1
	case trimmedLeft != trimmedRight:
		if trimmedLeft < trimmedRight {
			return -1
		}
		return 1
	default:
		return 0
	}
}

// foldForCollation lowercases and strips accents, which is what sensitivity "base"
// compares on.
func foldForCollation(value string) string {
	// Decompose, drop the combining marks, then lowercase. That is the order that
	// makes "Ä" and "a" compare equal.
	stripped, _, err := transform.String(
		transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC),
		value,
	)
	if err != nil {
		stripped = value
	}

	// The same case folding the tag key uses, so the two agree on what "the same
	// name" means -- including the Final_Sigma and dotted-I cases a plain ToLower
	// gets wrong.
	return cases.Lower(language.Und).String(stripped)
}
