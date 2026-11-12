package autotag

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/taoquo/trove/apps/macos/internal/domain"
)

// Vocabulary review, ported from src/lib/autotag/vocabularyReview.ts.
//
// A tag library grows one accepted suggestion at a time and drifts into
// near-synonyms: "ai agent", "agentic workflow", "llm agents". This asks the model to
// look at the whole vocabulary once and propose merges and renames, which the user
// then applies selectively.
//
// Nothing here mutates storage. It produces plans; applying them is the caller's job,
// and the user's decision.

// Review limits, from the source.
const (
	// MaxReviewVocabularyEntries is how many tags the prompt carries. A vocabulary
	// past this is truncated to the most used, so the prompt stays a bounded size.
	MaxReviewVocabularyEntries = 300
	// MaxProposals caps what one review may return, so a runaway model cannot
	// produce a list nobody can read.
	MaxProposals = 60
	// MaxTagNameLength matches the tag name limit the rest of the app enforces.
	MaxTagNameLength = 40
)

// VocabularyEntry is one tag in the prompt, with how often it is used.
type VocabularyEntry struct {
	Name  string
	Count int
}

// MergeProposal asks to fold several tags into one.
type MergeProposal struct {
	From   []string `json:"from"`
	Into   string   `json:"into"`
	Reason string   `json:"reason,omitempty"`
}

// RenameProposal asks to rename one tag.
type RenameProposal struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
}

// VocabularyReview is what the model returned, resolved against local names.
type VocabularyReview struct {
	Merges  []MergeProposal  `json:"merges"`
	Renames []RenameProposal `json:"renames"`
}

// TagMergePlan is a merge resolved to real tag ids.
type TagMergePlan struct {
	TargetTagID  string   `json:"targetTagId"`
	TargetName   string   `json:"targetName"`
	SourceTagIDs []string `json:"sourceTagIds"`
	SourceNames  []string `json:"sourceNames"`
	Reason       string   `json:"reason,omitempty"`
}

// TagRenamePlan is a rename resolved to a real tag id.
type TagRenamePlan struct {
	TagID  string `json:"tagId"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
}

// TagVocabularyReviewResult is what the caller gets.
type TagVocabularyReviewResult struct {
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Merges  []TagMergePlan  `json:"merges"`
	Renames []TagRenamePlan `json:"renames"`
}

// MergePlanKey is a stable identity for a proposal, used as the selection key in the
// review panel so a checked box survives a re-render.
func MergePlanKey(plan TagMergePlan) string {
	return "merge:" + plan.TargetTagID + "<" + strings.Join(plan.SourceTagIDs, ",")
}

// RenamePlanKey is the identity for a rename proposal.
func RenamePlanKey(plan TagRenamePlan) string {
	return "rename:" + plan.TagID + ">" + plan.To
}

// BuildVocabularyEntries pairs each tag with its usage count.
func BuildVocabularyEntries(tags []domain.Tag, relations []domain.BookmarkTag) []VocabularyEntry {
	countByTagID := make(map[string]int, len(tags))
	for _, relation := range relations {
		countByTagID[relation.TagID]++
	}

	entries := make([]VocabularyEntry, 0, len(tags))
	for _, tag := range tags {
		entries = append(entries, VocabularyEntry{Name: tag.Name, Count: countByTagID[tag.ID]})
	}
	return entries
}

// BuildVocabularyReviewPrompt is the instruction the model receives.
//
// It is reproduced verbatim. The wording is what keeps the model from merging across
// unrelated domains to hit a number, and from renaming into a name that already
// exists; paraphrasing it would change the proposals.
func BuildVocabularyReviewPrompt(vocabulary []VocabularyEntry) string {
	ordered := append([]VocabularyEntry{}, vocabulary...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Count != ordered[j].Count {
			return ordered[i].Count > ordered[j].Count
		}
		// The source breaks a tie with localeCompare, which is case- and
		// accent-insensitive: byte order would put "LLMs" before "agentic ai".
		// CompareNaturalNames reproduces that. One honest difference: it is also
		// numeric-aware, which localeCompare without options is not, so two names
		// differing only in a number would order differently. No name in the review
		// prompt is expected to contain one.
		return domain.CompareNaturalNames(ordered[i].Name, ordered[j].Name) < 0
	})
	if len(ordered) > MaxReviewVocabularyEntries {
		ordered = ordered[:MaxReviewVocabularyEntries]
	}

	formatted := make([]string, 0, len(ordered))
	for _, entry := range ordered {
		formatted = append(formatted, fmt.Sprintf("%s (%d)", entry.Name, entry.Count))
	}
	joined := strings.Join(formatted, ", ")
	if joined == "" {
		joined = "(empty)"
	}

	return strings.Join([]string{
		"You are a librarian consolidating the tag taxonomy of a bookmark library. The library has drifted: many tags are near-synonyms or one-off labels.",
		strings.Join([]string{
			"Goal:",
			fmt.Sprintf("- The healthy taxonomy size is %d-%d durable subject tags. Propose the merges needed to move towards that size.",
				HealthyTagCountMin, HealthyTagCountMax),
			"- Merge only tags that describe the same durable subject area. Never merge across different domains just to reduce the count.",
			"- Prefer the broader, most established name (highest usage count) as the surviving tag.",
			"- Rename a tag when its name breaks the naming rules but no other tag covers the same concept.",
			"- Leave a tag untouched when you are unsure. Silence is better than a wrong merge.",
		}, "\n"),
		strings.Join([]string{
			"Naming rules for the surviving tag:",
			"- English only, lowercase, singular.",
			"- One or two words, words separated by a single space.",
			"- No leading '#', no emoji, no punctuation, no author handles.",
		}, "\n"),
		strings.Join([]string{
			"Output rules:",
			"- Every name in \"from\" must be copied exactly from the vocabulary below.",
			"- \"into\" is the surviving tag name. \"reason\" is one short English clause.",
			"- Do not repeat a tag in more than one merge group.",
		}, "\n"),
		fmt.Sprintf("Current vocabulary, most used first (name and usage count): %s", joined),
		"Respond with JSON only: {\"merges\":[{\"from\":[\"ai agents\",\"agentic ai\"],\"into\":\"ai agent\",\"reason\":\"same concept\"}],\"renames\":[{\"from\":\"LLMs\",\"to\":\"llm\",\"reason\":\"naming\"}]}",
	}, "\n\n")
}

// sanitizeTargetName normalizes a model-supplied name, or rejects it.
func sanitizeTargetName(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	name := domain.NormalizeTagDisplayName(text)
	if name == "" || len([]rune(name)) > MaxTagNameLength {
		return ""
	}
	return name
}

// buildKnownNameIndex maps a canonical key back to the exact local display name.
//
// It is what makes "AI Agents" resolve to the existing "ai agent" tag instead of
// creating a second one.
func buildKnownNameIndex(knownTagNames []string) map[string]string {
	index := make(map[string]string, len(knownTagNames))
	for _, name := range knownTagNames {
		key := domain.CanonicalTagKey(name)
		if key == "" {
			continue
		}
		if _, present := index[key]; !present {
			index[key] = name
		}
	}
	return index
}

// sanitizeReason trims a reason to one short clause.
func sanitizeReason(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	fields := strings.Fields(text)
	joined := strings.Join(fields, " ")
	runes := []rune(joined)
	if len(runes) > 120 {
		joined = string(runes[:120])
	}
	return joined
}

// ParseVocabularyReview reads the model's JSON, discarding anything it cannot trust.
//
// It is deliberately forgiving of shape and strict about meaning: a proposal naming a
// tag that does not exist locally is dropped, a tag may appear in only one proposal,
// and a merge whose surviving name does not exist yet is turned into a merge onto the
// best-established source plus a rename. Refusing the whole response over one bad
// entry would throw away the good proposals with it.
func ParseVocabularyReview(content string, knownTagNames []string) VocabularyReview {
	review := VocabularyReview{Merges: []MergeProposal{}, Renames: []RenameProposal{}}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return review
	}

	known := buildKnownNameIndex(knownTagNames)
	// A tag may only appear in one proposal; later duplicates are dropped.
	claimedKeys := make(map[string]bool)

	rawMerges, _ := parsed["merges"].([]any)
	for _, entry := range rawMerges {
		if len(review.Merges) >= MaxProposals {
			break
		}

		object, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		rawFrom, ok := object["from"].([]any)
		if !ok {
			continue
		}
		intoName := sanitizeTargetName(object["into"])
		if intoName == "" {
			continue
		}

		intoKey := domain.CanonicalTagKey(intoName)
		sources := make([]string, 0, len(rawFrom))
		seenSourceKeys := make(map[string]bool)

		for _, candidate := range rawFrom {
			text, _ := candidate.(string)
			key := domain.CanonicalTagKey(text)
			if key == "" {
				continue
			}
			localName, exists := known[key]
			if !exists || key == intoKey || seenSourceKeys[key] || claimedKeys[key] {
				continue
			}

			seenSourceKeys[key] = true
			sources = append(sources, localName)
		}

		if len(sources) == 0 {
			continue
		}

		target, targetExists := known[intoKey]
		reason := sanitizeReason(object["reason"])

		if targetExists {
			if claimedKeys[intoKey] {
				continue
			}
			claimedKeys[intoKey] = true
			for key := range seenSourceKeys {
				claimedKeys[key] = true
			}
			review.Merges = append(review.Merges, MergeProposal{From: sources, Into: target, Reason: reason})
			continue
		}

		// The surviving name does not exist yet, so the most established source
		// becomes the target and is renamed afterwards.
		survivor := sources[0]
		for key := range seenSourceKeys {
			claimedKeys[key] = true
		}
		if len(sources) > 1 {
			review.Merges = append(review.Merges, MergeProposal{From: sources[1:], Into: survivor, Reason: reason})
		}
		review.Renames = append(review.Renames, RenameProposal{From: survivor, To: intoName, Reason: reason})
	}

	rawRenames, _ := parsed["renames"].([]any)
	for _, entry := range rawRenames {
		if len(review.Renames) >= MaxProposals {
			break
		}

		object, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		fromText, _ := object["from"].(string)
		fromKey := domain.CanonicalTagKey(fromText)
		localName, exists := known[fromKey]
		if fromKey == "" || !exists || claimedKeys[fromKey] {
			continue
		}

		toName := sanitizeTargetName(object["to"])
		if toName == "" {
			continue
		}
		toKey := domain.CanonicalTagKey(toName)
		if toKey == fromKey {
			continue
		}

		reason := sanitizeReason(object["reason"])

		// Renaming onto an existing tag is a merge in disguise.
		if existingTarget, present := known[toKey]; present {
			if claimedKeys[toKey] {
				continue
			}
			claimedKeys[fromKey] = true
			claimedKeys[toKey] = true
			review.Merges = append(review.Merges,
				MergeProposal{From: []string{localName}, Into: existingTarget, Reason: reason})
			continue
		}

		claimedKeys[fromKey] = true
		review.Renames = append(review.Renames, RenameProposal{From: localName, To: toName, Reason: reason})
	}

	return review
}

// ResolveReview turns proposals into plans against real tag ids.
//
// A proposal naming a tag that is not in the library is dropped here rather than
// earlier: the proposals are what the model said, and the plans are what can actually
// be applied.
func ResolveReview(review VocabularyReview, tags []domain.Tag) ([]TagMergePlan, []TagRenamePlan) {
	tagByKey := make(map[string]domain.Tag, len(tags))
	for _, tag := range tags {
		key := domain.CanonicalTagKey(tag.Name)
		if key == "" {
			continue
		}
		if _, present := tagByKey[key]; !present {
			tagByKey[key] = tag
		}
	}

	merges := make([]TagMergePlan, 0, len(review.Merges))
	for _, proposal := range review.Merges {
		target, present := tagByKey[domain.CanonicalTagKey(proposal.Into)]
		if !present {
			continue
		}

		sources := make([]domain.Tag, 0, len(proposal.From))
		for _, name := range proposal.From {
			source, found := tagByKey[domain.CanonicalTagKey(name)]
			if found && source.ID != target.ID {
				sources = append(sources, source)
			}
		}
		if len(sources) == 0 {
			continue
		}

		sourceIDs := make([]string, 0, len(sources))
		sourceNames := make([]string, 0, len(sources))
		for _, source := range sources {
			sourceIDs = append(sourceIDs, source.ID)
			sourceNames = append(sourceNames, source.Name)
		}

		merges = append(merges, TagMergePlan{
			TargetTagID: target.ID, TargetName: target.Name,
			SourceTagIDs: sourceIDs, SourceNames: sourceNames, Reason: proposal.Reason,
		})
	}

	renames := make([]TagRenamePlan, 0, len(review.Renames))
	for _, proposal := range review.Renames {
		tag, present := tagByKey[domain.CanonicalTagKey(proposal.From)]
		if !present {
			continue
		}
		renames = append(renames, TagRenamePlan{
			TagID: tag.ID, From: tag.Name, To: proposal.To, Reason: proposal.Reason,
		})
	}

	return merges, renames
}

// ReviewChatFunc performs one chat request and returns its content.
type ReviewChatFunc func(prompt string) (string, error)

// ReviewTagVocabulary builds the prompt, asks the model, and resolves the answer.
func ReviewTagVocabulary(
	tags []domain.Tag,
	relations []domain.BookmarkTag,
	chat ReviewChatFunc,
) (TagVocabularyReviewResult, error) {
	// One tag cannot be consolidated with anything, and asking would spend a request
	// to be told so.
	if len(tags) < 2 {
		return TagVocabularyReviewResult{OK: true, Merges: []TagMergePlan{}, Renames: []TagRenamePlan{}}, nil
	}
	if chat == nil {
		return TagVocabularyReviewResult{}, fmt.Errorf("autotag: no chat function configured for the review")
	}

	prompt := BuildVocabularyReviewPrompt(BuildVocabularyEntries(tags, relations))

	content, err := chat(prompt)
	if err != nil {
		return TagVocabularyReviewResult{OK: false, Error: err.Error(), Merges: []TagMergePlan{}, Renames: []TagRenamePlan{}}, nil
	}

	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}

	review := ParseVocabularyReview(content, names)
	merges, renames := ResolveReview(review, tags)

	return TagVocabularyReviewResult{OK: true, Merges: merges, Renames: renames}, nil
}
