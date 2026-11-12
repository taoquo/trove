// Generates the tag-relation-graph equivalence fixture from the *actual*
// TypeScript implementation in the extension (src/lib/graph/tagRelations.ts), so the
// Go port is checked against the real code rather than a re-reading of it.
//
// Run from the repo root with Node 22.6+ (native TypeScript stripping):
//
//   node apps/macos/testdata/taggraph/gentaggraph.mjs
//
// Writes taggraph.json next to this file. Regenerate whenever the TypeScript source
// changes; the Go test then fails if the two implementations diverge.
//
// The algorithm decides which edges a user sees, so drift would silently change the
// shape of the graph -- a wrong picture looks just as convincing as a right one.

import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../../..");
const graphPath = path.join(repoRoot, "src/lib/graph/tagRelations.ts");

const { buildTagRelationGraph, DEFAULT_TAG_RELATION_OPTIONS } = await import(
  "file://" + graphPath
);

const tag = (id, name, extra = {}) => ({ id, name, ...extra });
const relation = (bookmarkId, tagId) => ({ bookmarkId, tagId });
const bookmark = (id, authorHandle) => ({ id, tweetId: id, authorHandle });

/** Name, input, and any options, covering the branches the algorithm has. */
const CASES = [
  {
    name: "empty",
    input: { tags: [], bookmarkTags: [], bookmarks: [] },
  },
  {
    name: "single tag no relations",
    input: {
      tags: [tag("t1", "Go")],
      bookmarkTags: [],
      bookmarks: [],
    },
  },
  {
    name: "two tags co-occurring on one bookmark",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust")],
      bookmarkTags: [relation("b1", "t1"), relation("b1", "t2")],
      bookmarks: [bookmark("b1", "ada")],
    },
  },
  {
    name: "co-occurrence count three",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust")],
      bookmarkTags: [
        relation("b1", "t1"), relation("b1", "t2"),
        relation("b2", "t1"), relation("b2", "t2"),
        relation("b3", "t1"), relation("b3", "t2"),
        // A bookmark with only one of them, so the Jaccard union is 4.
        relation("b4", "t1"),
      ],
      bookmarks: [
        bookmark("b1", "ada"), bookmark("b2", "ada"),
        bookmark("b3", "grace"), bookmark("b4", "grace"),
      ],
    },
  },
  {
    name: "duplicate relation rows count once per bookmark",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust")],
      bookmarkTags: [
        relation("b1", "t1"), relation("b1", "t1"), relation("b1", "t2"),
        relation("b1", "t2"),
      ],
      bookmarks: [bookmark("b1", "ada")],
    },
  },
  {
    name: "relation to an unknown tag is ignored",
    input: {
      tags: [tag("t1", "Go")],
      bookmarkTags: [relation("b1", "t1"), relation("b1", "missing")],
      bookmarks: [bookmark("b1", "ada")],
    },
  },
  {
    name: "hierarchy produces an edge and clusters the child",
    input: {
      tags: [
        tag("root", "Languages"),
        tag("child", "Go", { parentId: "root" }),
        tag("orphan", "Rust", { parentId: "removed" }),
      ],
      bookmarkTags: [],
      bookmarks: [],
    },
  },
  {
    name: "author bridge without shared bookmarks",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust")],
      bookmarkTags: [
        relation("b1", "t1"), relation("b2", "t2"), relation("b3", "t1"),
      ],
      bookmarks: [
        bookmark("b1", "ada"), bookmark("b2", "ada"), bookmark("b3", "grace"),
      ],
    },
  },
  {
    // Two authors each write about both topics, but never both in one bookmark, so
    // the only signal is the author bridge. With the default minimum of two authors
    // this is exactly on the threshold: it must survive.
    name: "author bridge at the threshold survives",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust")],
      bookmarkTags: [
        relation("b1", "t1"), relation("b2", "t2"),
        relation("b3", "t1"), relation("b4", "t2"),
      ],
      bookmarks: [
        bookmark("b1", "ada"), bookmark("b2", "ada"),
        bookmark("b3", "grace"), bookmark("b4", "grace"),
      ],
    },
  },
  {
    name: "author bridge below the threshold is dropped",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust")],
      bookmarkTags: [relation("b1", "t1"), relation("b2", "t2")],
      // One author only, and the default minimum is two.
      bookmarks: [bookmark("b1", "ada"), bookmark("b2", "ada")],
    },
  },
  {
    name: "author layer included",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust"), tag("t3", "SQL")],
      bookmarkTags: [
        relation("b1", "t1"), relation("b2", "t2"), relation("b3", "t3"),
        relation("b4", "t1"), relation("b5", "t2"),
      ],
      bookmarks: [
        bookmark("b1", "ada"), bookmark("b2", "ada"), bookmark("b3", "ada"),
        bookmark("b4", "grace"), bookmark("b5", "grace"),
      ],
    },
    options: { includeAuthors: true },
  },
  {
    name: "author layer excluded by default",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust"), tag("t3", "SQL")],
      bookmarkTags: [
        relation("b1", "t1"), relation("b2", "t2"), relation("b3", "t3"),
      ],
      bookmarks: [
        bookmark("b1", "ada"), bookmark("b2", "ada"), bookmark("b3", "ada"),
      ],
    },
  },
  {
    name: "bookmark without an author handle is skipped",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust")],
      bookmarkTags: [relation("b1", "t1"), relation("b2", "t2")],
      bookmarks: [bookmark("b1", "   "), bookmark("b2", "")],
    },
  },
  {
    name: "per-node edge budget trims a dense graph",
    input: {
      tags: [tag("c", "Centre"), tag("a", "A"), tag("b", "B"), tag("d", "D")],
      bookmarkTags: [
        // Centre co-occurs with three tags, so with a budget of two the weakest
        // edge survives only if the other endpoint ranks it.
        relation("x1", "c"), relation("x1", "a"),
        relation("x2", "c"), relation("x2", "b"),
        relation("x3", "c"), relation("x3", "d"),
        relation("x4", "a"), relation("x4", "b"),
      ],
      bookmarks: [
        bookmark("x1", "ada"), bookmark("x2", "ada"),
        bookmark("x3", "ada"), bookmark("x4", "grace"),
      ],
    },
    options: { maxEdgesPerNode: 2, includeAuthors: false },
  },
  {
    name: "co-occurrence weight below the floor is dropped",
    input: {
      tags: [tag("big", "Big"), tag("small", "Small")],
      // One shared bookmark out of many, so the Jaccard weight is tiny.
      bookmarkTags: [
        relation("s1", "big"), relation("s1", "small"),
        ...Array.from({ length: 40 }, (_, i) => relation(`big${i}`, "big")),
      ],
      bookmarks: [],
    },
    options: { minCooccurrenceWeight: 0.5 },
  },
  {
    name: "aliases and top authors round-trip on the node",
    input: {
      tags: [tag("t1", "Go", { aliases: ["golang", "go-lang"] })],
      bookmarkTags: [
        relation("b1", "t1"), relation("b2", "t1"),
        relation("b3", "t1"), relation("b4", "t1"),
      ],
      bookmarks: [
        bookmark("b1", "ada"), bookmark("b2", "ada"),
        bookmark("b3", "grace"), bookmark("b4", "alan"),
      ],
    },
    options: { topAuthorsPerTag: 2 },
  },
  {
    name: "three tags on one bookmark make three edges",
    input: {
      tags: [tag("t1", "Go"), tag("t2", "Rust"), tag("t3", "SQL")],
      bookmarkTags: [relation("b1", "t1"), relation("b1", "t2"), relation("b1", "t3")],
      bookmarks: [bookmark("b1", "ada")],
    },
  },
];

const cases = CASES.map((entry) => ({
  name: entry.name,
  input: entry.input,
  options: entry.options ?? {},
  expected: buildTagRelationGraph({
    ...entry.input,
    options: entry.options ?? {},
  }),
}));

const output = {
  generatedFrom: "src/lib/graph/tagRelations.ts",
  defaults: DEFAULT_TAG_RELATION_OPTIONS,
  cases,
};

const target = path.join(here, "taggraph.json");
writeFileSync(target, JSON.stringify(output, null, 2) + "\n");

const edges = cases.reduce((sum, entry) => sum + entry.expected.edges.length, 0);
const nodes = cases.reduce((sum, entry) => sum + entry.expected.nodes.length, 0);
console.log(
  `wrote ${path.relative(repoRoot, target)}: ${cases.length} cases, ${nodes} nodes, ${edges} edges`
);
