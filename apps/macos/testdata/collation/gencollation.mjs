// Generates the name-collation fixture from the extension's own collator
// (src/options/collation.ts), so the Go port is checked against the real
// Intl.Collator rather than against a reading of its documentation.
//
//   node apps/macos/testdata/collation/gencollation.mjs
import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));

// The same options the extension constructs.
const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

// Names a tag library actually holds: numbered series, mixed case, punctuation,
// accents, emoji, and the empty string.
const NAMES = [
  "", "a", "A", "b", "B", "z",
  "Tag 1", "Tag 2", "Tag 10", "Tag 20", "Tag 100", "tag 3", "TAG 4",
  "1", "2", "10", "01", "001", "007",
  "x1", "x2", "x10", "x 1", "x-2", "x_10",
  "apple", "Banana", "cherry", "Apple", "APPLE",
  "Go", "go", "Golang", "Rust", "rust-lang",
  "C++", "C#", "C", ".NET", "node.js",
  "café", "cafe", "CAFÉ", "naïve", "naive",
  "Ä", "ä", "a", "z", "Ö", "o",
  "日本語", "中文", "English",
  "Tag", "Tags", "Tag 0", "Tag 00",
  "  spaced  ", "spaced",
  "Ünïcödé", "unicode",
];

const pairs = [];
for (const left of NAMES) {
  for (const right of NAMES) {
    pairs.push({ left, right, sign: Math.sign(collator.compare(left, right)) });
  }
}

// A few explicitly sorted lists, so the ordering as a whole is pinned too.
const LISTS = [
  ["Tag 1", "Tag 2", "Tag 10", "Tag 20", "Tag 100"],
  ["a", "B", "c", "D"],
  ["apple", "Banana", "cherry"],
];

const sortedLists = LISTS.map((list) => ({
  input: list,
  sorted: [...list].sort((l, r) => collator.compare(l, r)),
}));

writeFileSync(
  path.join(here, "collation.json"),
  JSON.stringify({ generatedFrom: "src/options/collation.ts", pairs, sortedLists }, null, 2) + "\n"
);
console.log(`wrote collation.json: ${pairs.length} pairs, ${sortedLists.length} lists`);
