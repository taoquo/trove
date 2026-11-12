// Generates the tag-key equivalence fixture from the *actual* TypeScript
// implementation in the old extension (src/lib/tags/canonical.ts), so the Go
// port is checked against the real code rather than a re-reading of it.
//
// Run from the repo root with Node 22.6+ (native TypeScript stripping):
//
//   node apps/macos/testdata/tagkey/gentagkeys.mjs
//
// Writes tagkeys.json next to this file. Regenerate whenever the TypeScript
// source changes; the Go test then fails if the two implementations diverge.
//
// See docs/domain-migration-map.md §4: canonicalTagKey drives tag de-duplication,
// blockedTagKeys matching, backup restore cleanup and suggestion ids, so any
// drift silently changes which tags exist.

import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { execSync } from "node:child_process";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../../..");
const canonicalPath = path.join(repoRoot, "src/lib/tags/canonical.ts");

const { canonicalTagKey, normalizeTagDisplayName, isSameTagName } = await import(
  canonicalPath
);

// ---------------------------------------------------------------------------
// Corpus
// ---------------------------------------------------------------------------

// Every word in the TypeScript NEVER_DEPLURALIZE set: the naive plural stripper
// would turn "devops" into "devop", so each must survive unchanged.
const WHITELIST = [
  "news", "devops", "ops", "ios", "macos", "aws", "gcs", "kubernetes",
  "series", "redis", "nextjs", "nodejs", "nestjs", "threejs", "js", "ts",
  "os", "saas", "paas", "iaas", "cms", "dns", "gis", "aigc",
];

const curated = [
  // --- headline collapse cases from the source comment ---
  "AI Agent", "ai-agents", "AI_Agent", "ai.agent", "ai/agent", "A I  Agent",
  // --- separators ---
  "machine-learning", "machine_learning", "machine.learning", "machine/learning",
  "machine learning", "  machine   learning  ", "-lead-", "_trail_", "a-b_c.d/e f",
  // --- leading hash marks ---
  "#AI", "##AI", "### AI", "#  AI  ", "#ai", "#", "##", "# #", "#-",
  // --- empty / whitespace ---
  "", " ", "   ", "\t", "\n", " \t\n ",
  // --- depluralize: -ies -> -y ---
  "technologies", "companies", "policies", "batteries", "stories", "ies", "ties",
  // --- depluralize: (s|x|z|ch|sh)es -> drop "es" ---
  "boxes", "churches", "dishes", "quizzes", "sizes", "taxes", "cases", "es",
  // --- depluralize: -s -> drop "s" ---
  "agents", "tags", "apps", "notebooks", "links", "tools", "models", "s",
  // --- -s but (ss|us|is|os) endings: must NOT lose the s ---
  "class", "status", "analysis", "boss", "virus", "basis", "focus", "bus",
  "campus", "crisis", "chaos", "pathos",
  // --- length <= 3 boundary: never pluralised ---
  "s", "js", "ts", "os", "css", "a", "ab", "abc", "abcd", "abcde",
  // --- not pure [a-z]: never pluralised ---
  "c++", "c#", "3d", "4k", "ai2", "v2", "gpt-4", "node.js", "vue.js", "x86-64",
  "AI2", "123", "a1b2",
  // --- Unicode: NFKC and case ---
  "ＡＩ", "＃ＡＩ", "ＡＩ Ａｇｅｎｔ", "café", "naïve", "Über", "STRASSE", "Straße",
  "e\u0301", "ﬁle", "㈱", "①②",
  // --- CJK / emoji / symbols (not pure [a-z], must pass through) ---
  "中文", "中文分词", "标签", "#中文", "机器学习", "日本語", "한국어",
  "🚀 rocket", "rocket 🚀", "☕", "a☕b", "—dash", "(paren)", "[bracket]",
  // --- mixed case + separators ---
  "GraphQL", "graphql", "Graph_QL", "Graph-QL", "REST API", "rest-api",
  "TypeScript", "typescript", "OpenAI", "open-ai", "open_ai",
  // --- long / pathological ---
  "a".repeat(300), ("tag ".repeat(50)).trim(), "x".repeat(4), "sses", "ieses",
  // --- Unicode whitespace: JS \s and trim() differ from Go's \s / TrimSpace ---
  "\u00a0AI\u00a0",        // NBSP: in JS \s, in Go regexp \s? (no)
  "\u3000AI\u3000",        // ideographic space
  "\u0085AI",              // NEL: Go IsSpace yes, JS trim() no
  "\ufeffAI",              // ZWNBSP/BOM: JS \s yes
  "A\u00a0B", "A\u3000B", "A\u2028B", "A\u2029B",
  "\u2028AI", "\u2000AI", "\u205fAI",
  // --- case mapping divergences between JS toLowerCase() and Go ToLower ---
  "\u0130",                // LATIN CAPITAL I WITH DOT ABOVE: JS -> "i\u0307" (2 runes)
  "\u0131",                // dotless i
  "İstanbul", "I\u0307",
  "\u03a3", "\u03c2", "\u03c3", "\u03a3\u03a3", // Greek sigma / final sigma
  "\u1e9e",                // capital sharp s
  // --- NFKC expansions ---
  "\ufb00le",              // ff ligature -> "ffle"
  "\u2460", "\u3231",      // circled 1 -> "1"; ㈱ -> "(株)"
  "e\u0301", "\u00e9",     // é whether composed or decomposed
];

// Systematic sweep: every whitelist word in several shapes, so a wrong entry in
// the Go stop-list is caught by more than its bare form.
const whitelistShapes = WHITELIST.flatMap((w) => [
  w,
  w.toUpperCase(),
  `${w}s`,
  `#${w}`,
  `${w}-tag`,
  `pre-${w}`,
]);

// Systematic sweep of the depluralize length boundary around 4.
const lengthBoundary = [];
for (let n = 1; n <= 6; n++) {
  lengthBoundary.push("a".repeat(n), `${"a".repeat(n)}s`, `${"a".repeat(n)}es`);
  lengthBoundary.push(`${"a".repeat(Math.max(0, n - 3))}ies`);
  lengthBoundary.push(`${"a".repeat(Math.max(0, n - 2))}es`);
  lengthBoundary.push(`${"a".repeat(Math.max(0, n - 2))}ss`);
}

const inputs = [...new Set([...curated, ...whitelistShapes, ...lengthBoundary])];

const cases = inputs.map((input) => ({
  input,
  display: normalizeTagDisplayName(input),
  key: canonicalTagKey(input),
}));

// Cross-checks for isSameTagName, including the asymmetric empty case.
const samePairs = [
  ["AI Agent", "ai-agents"],
  ["AI Agent", "AI_Agent"],
  ["Agent", "Agents"],
  ["Tag", "tags"],
  ["news", "news"],
  ["news", "new"],
  ["devops", "devop"],
  ["", ""],
  ["", "x"],
  ["x", ""],
  ["中文", "中文"],
  ["中文", "中"],
  ["#AI", "AI"],
  ["class", "classes"],
  ["box", "boxes"],
  ["technology", "technologies"],
];

const sameTagName = samePairs.map(([left, right]) => ({
  left,
  right,
  same: isSameTagName(left, right),
}));

// ---------------------------------------------------------------------------

let commit = "unknown";
try {
  commit = execSync("git rev-parse HEAD", { cwd: repoRoot }).toString().trim();
} catch {
  // Not a git checkout; the fixture is still usable.
}

const fixture = {
  note: "Generated from src/lib/tags/canonical.ts. Do not edit by hand.",
  generator: "apps/macos/testdata/tagkey/gentagkeys.mjs",
  sourceFile: "src/lib/tags/canonical.ts",
  sourceCommit: commit,
  caseCount: cases.length,
  cases,
  sameTagName,
};

const out = path.join(here, "tagkeys.json");
writeFileSync(out, `${JSON.stringify(fixture, null, 2)}\n`, "utf8");
console.log(
  `wrote ${out}\n  cases: ${cases.length}\n  sameTagName pairs: ${sameTagName.length}\n  source commit: ${commit}`
);
