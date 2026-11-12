// Generates the vocabulary-review fixture from the real TypeScript
// (src/lib/autotag/vocabularyReview.ts), so the Go port is checked against the source
// rather than a re-reading of it.
//
// The parser is tolerant by design, which is exactly why it needs a fixture: every
// "drop this proposal" rule is a judgement call, and a re-implementation would make
// different ones.
//
//   node apps/macos/testdata/vocabreview/genvocabreview.mjs
import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../../..");

const { buildVocabularyReviewPrompt, buildVocabularyEntries, parseVocabularyReview, mergePlanKey, renamePlanKey } =
  await import("file://" + path.join(repoRoot, "src/lib/autotag/vocabularyReview.ts"));

// Tags whose canonical keys collide are the interesting case: "AI Agents" and
// "ai agent" are the same tag as far as canonicalTagKey is concerned.
const TAGS = [
  { id: "t1", name: "ai agent" },
  { id: "t2", name: "agentic ai" },
  { id: "t3", name: "llm agents" },
  { id: "t4", name: "llm" },
  { id: "t5", name: "LLMs" },
  { id: "t6", name: "databases" },
  { id: "t7", name: "postgres" },
  { id: "t8", name: "postgresql" },
  { id: "t9", name: "Café" },
  { id: "t10", name: "kubernetes" },
  { id: "t11", name: "k8s" },
  { id: "t12", name: "#rust" },
  { id: "t13", name: "Rust" },
  { id: "t14", name: "a very long tag name that exceeds the forty character limit for sure" },
];

const RELATIONS = [
  { bookmarkId: "b1", tagId: "t1" }, { bookmarkId: "b2", tagId: "t1" },
  { bookmarkId: "b3", tagId: "t1" }, { bookmarkId: "b4", tagId: "t2" },
  { bookmarkId: "b5", tagId: "t3" }, { bookmarkId: "b6", tagId: "t4" },
  { bookmarkId: "b7", tagId: "t4" }, { bookmarkId: "b8", tagId: "t4" },
  { bookmarkId: "b9", tagId: "t4" }, { bookmarkId: "b10", tagId: "t5" },
  { bookmarkId: "b11", tagId: "t6" }, { bookmarkId: "b12", tagId: "t7" },
  { bookmarkId: "b13", tagId: "t8" }, { bookmarkId: "b14", tagId: "t9" },
  { bookmarkId: "b15", tagId: "t10" }, { bookmarkId: "b16", tagId: "t11" },
  { bookmarkId: "b17", tagId: "t12" }, { bookmarkId: "b18", tagId: "t13" },
];

const VOCABULARY = buildVocabularyEntries(TAGS, RELATIONS);

const KNOWN = TAGS.map((tag) => tag.name);

// Responses covering each branch of the parser.
const RESPONSES = [
  {
    name: "not json",
    content: "I cannot help with that.",
  },
  {
    name: "empty object",
    content: "{}",
  },
  {
    name: "empty arrays",
    content: '{"merges":[],"renames":[]}',
  },
  {
    name: "a merge onto an existing tag",
    content: '{"merges":[{"from":["ai agent","agentic ai","llm agents"],"into":"ai agent","reason":"same concept"}]}',
  },
  {
    name: "a merge whose target does not exist yet",
    content: '{"merges":[{"from":["postgres","postgresql"],"into":"postgres database","reason":"same concept"}]}',
  },
  {
    name: "a merge whose target does not exist and only one source",
    content: '{"merges":[{"from":["k8s"],"into":"kubernetes cluster","reason":"naming"}]}',
  },
  {
    name: "the target is inside from",
    content: '{"merges":[{"from":["ai agent","agentic ai"],"into":"ai agent"}]}',
  },
  {
    name: "an unknown source name",
    content: '{"merges":[{"from":["nonexistent tag","agentic ai"],"into":"ai agent"}]}',
  },
  {
    name: "a merge with no resolvable source",
    content: '{"merges":[{"from":["nothing here"],"into":"ai agent"}]}',
  },
  {
    name: "a merge with an empty into",
    content: '{"merges":[{"from":["agentic ai"],"into":""}]}',
  },
  {
    name: "a merge with a blank into after trimming",
    content: '{"merges":[{"from":["agentic ai"],"into":"   "}]}',
  },
  {
    name: "a merge with an over-long into",
    content: `{"merges":[{"from":["agentic ai"],"into":"${"x".repeat(80)}"}]}`,
  },
  {
    name: "a renames entry",
    content: '{"renames":[{"from":"LLMs","to":"llm","reason":"naming"}]}',
  },
  {
    name: "a rename onto itself through case",
    content: '{"renames":[{"from":"llm","to":"LLM"}]}',
  },
  {
    name: "a rename onto an existing tag becomes a merge",
    content: '{"renames":[{"from":"agentic ai","to":"ai agent","reason":"same concept"}]}',
  },
  {
    name: "a rename of an unknown tag",
    content: '{"renames":[{"from":"not a tag","to":"something"}]}',
  },
  {
    name: "the same tag claimed twice",
    content: '{"merges":[{"from":["agentic ai"],"into":"ai agent"},{"from":["agentic ai"],"into":"llm"}]}',
  },
  {
    name: "two merges fighting over one target",
    content: '{"merges":[{"from":["agentic ai"],"into":"llm"},{"from":["llm agents"],"into":"llm"}]}',
  },
  {
    name: "a name that only differs by punctuation",
    content: '{"merges":[{"from":["#rust"],"into":"Rust"}]}',
  },
  {
    name: "an accented name",
    content: '{"renames":[{"from":"Café","to":"cafe"}]}',
  },
  {
    name: "reasons are trimmed and clipped",
    content: `{"merges":[{"from":["agentic ai"],"into":"ai agent","reason":"${"word ".repeat(40)}"}]}`,
  },
  {
    name: "wrongly typed entries are ignored",
    content: '{"merges":"not an array","renames":[null,42,{"from":42,"to":43}]}',
  },
  {
    name: "more proposals than the cap",
    content: JSON.stringify({
      renames: Array.from({ length: 80 }, (_, i) => ({ from: "llm", to: `llm-${i}` })),
    }),
  },
];

const cases = RESPONSES.map((entry) => {
  const review = parseVocabularyReview(entry.content, KNOWN);
  return {
    name: entry.name,
    content: entry.content,
    expected: {
      review,
      mergeKeys: [],
      renameKeys: [],
    },
  };
});

// The prompt is text, so it is compared whole.
const promptCases = [
  { name: "the real vocabulary", vocabulary: VOCABULARY },
  { name: "one entry", vocabulary: [{ name: "go", count: 3 }] },
  { name: "empty vocabulary", vocabulary: [] },
  {
    name: "more entries than the cap",
    vocabulary: Array.from({ length: 400 }, (_, i) => ({ name: `tag ${i}`, count: 400 - i })),
  },
  {
    name: "ties are broken by name",
    vocabulary: [
      { name: "zeta", count: 5 },
      { name: "alpha", count: 5 },
      { name: "mu", count: 5 },
    ],
  },
];

writeFileSync(
  path.join(here, "vocabreview.json"),
  JSON.stringify(
    {
      generatedFrom: "src/lib/autotag/vocabularyReview.ts",
      tags: TAGS,
      bookmarkTags: RELATIONS,
      vocabulary: VOCABULARY,
      cases,
      prompts: promptCases.map((entry) => ({
        name: entry.name,
        vocabulary: entry.vocabulary,
        prompt: buildVocabularyReviewPrompt({ vocabulary: entry.vocabulary }),
      })),
    },
    null,
    2
  ) + "\n"
);
console.log(`wrote vocabreview.json: ${cases.length} parse cases, ${promptCases.length} prompts`);
