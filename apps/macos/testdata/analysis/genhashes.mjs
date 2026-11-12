// Generates the content-hash equivalence fixture from the *actual* TypeScript
// implementation (src/lib/storage/llmAnalysisStore.ts).
//
//   node apps/macos/testdata/analysis/genhashes.mjs
//
// Why this matters: hashAnalyzedContent decides whether a stored LLM verdict is
// still valid. JavaScript hashes UTF-16 code units; the natural Go port hashes
// UTF-8 bytes. Those agree for ASCII and disagree for everything else, so a
// wrong port re-sends every CJK bookmark to the model on first run after
// migration. See docs/ai-autotagging.md §4.4 and decision D9.

import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { execSync } from "node:child_process";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../../..");
const storePath = path.join(repoRoot, "src/lib/storage/llmAnalysisStore.ts");

const { hashAnalyzedContent, LLM_PROMPT_VERSION } = await import(storePath);

const ascii = "The quick brown fox jumps over the lazy dog";
const cjk = "机器学习与人工智能代理系统设计";
const emoji = "Rocket 🚀 launch ☕ coffee 🎉";
const mixed = "AI 代理 (agents) — naïve café Ünïcode 🚀";

const inputs = [
  "",
  " ",
  "\n",
  "a",
  "ab",
  ascii,
  ascii.toUpperCase(),
  `${ascii}.`,
  cjk,
  "中文",
  "标签",
  "机器学习",
  emoji,
  mixed,
  "café",
  "e\u0301",
  "\u00e9",
  "\u0130stanbul",
  "\u03a3\u03a3",
  "ＡＩ",
  "ﬁle",
  "\u2460",
  "\t\r\n",
  "a".repeat(1000),
  "中".repeat(500),
  "\u{1F600}".repeat(100),
  // Lengths around the 32-bit wrap so the modulo arithmetic is exercised.
  "x".repeat(31), "x".repeat(32), "x".repeat(33),
  "x".repeat(255), "x".repeat(256), "x".repeat(257),
  // Byte/unit boundary cases: these differ between UTF-8 and UTF-16 hashing.
  "é", "éé", "ééé", "🚀", "🚀🚀",
];

const cases = [...new Set(inputs)].map((input) => ({
  input,
  // Code-unit length is recorded so a failure shows whether the two sides even
  // agreed on how long the input is.
  utf16Length: input.length,
  hash: hashAnalyzedContent(input),
}));

let commit = "unknown";
try {
  commit = execSync("git rev-parse HEAD", { cwd: repoRoot }).toString().trim();
} catch {
  // Not a git checkout.
}

const fixture = {
  note: "Generated from src/lib/storage/llmAnalysisStore.ts. Do not edit by hand.",
  generator: "apps/macos/testdata/analysis/genhashes.mjs",
  sourceFile: "src/lib/storage/llmAnalysisStore.ts",
  sourceCommit: commit,
  promptVersion: LLM_PROMPT_VERSION,
  caseCount: cases.length,
  cases,
};

const out = path.join(here, "hashes.json");
writeFileSync(out, `${JSON.stringify(fixture, null, 2)}\n`, "utf8");
console.log(`wrote ${out}\n  cases: ${cases.length}\n  promptVersion: ${LLM_PROMPT_VERSION}\n  source commit: ${commit}`);
