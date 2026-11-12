// Generates the tag-library fixture from the real TypeScript
// (src/options/tagLibrary.ts), so the Go port is checked against the source.
//
//   node apps/macos/testdata/taglibrary/gentaglibrary.mjs
import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../../..");

const {
  buildTagLibraryItems, countOrphanTags, countLongTailTags, getRemovableOrphanTagIds,
  countUnparentedTags, getRootTagCandidates, getParentCandidates, hasParentCandidates,
  getVisibleTagLibraryItems, filterSidebarTags, getVisibleSidebarTags, LONG_TAIL_MAX_BOOKMARKS,
} = await import("file://" + path.join(repoRoot, "src/options/tagLibrary.ts"));

const TAGS = [
  { id: "t1", name: "languages", createdAt: "2026-01-01T00:00:00.000Z", updatedAt: "2026-05-01T00:00:00.000Z" },
  { id: "t2", name: "Go", parentId: "t1", aliases: ["golang"], createdAt: "2026-02-01T00:00:00.000Z", updatedAt: "2026-06-01T00:00:00.000Z" },
  { id: "t3", name: "Rust", parentId: "t1", createdAt: "2026-02-02T00:00:00.000Z", updatedAt: "2026-04-01T00:00:00.000Z" },
  { id: "t4", name: "AI Agents", createdAt: "2026-03-01T00:00:00.000Z", updatedAt: "2026-07-01T00:00:00.000Z" },
  { id: "t5", name: "agentic ai", createdAt: "2026-03-02T00:00:00.000Z", updatedAt: "2026-03-02T00:00:00.000Z" },
  { id: "t6", name: "orphan tag", createdAt: "2026-03-03T00:00:00.000Z", updatedAt: "2026-03-03T00:00:00.000Z" },
  { id: "t7", name: "under review", createdAt: "2026-03-04T00:00:00.000Z", updatedAt: "2026-03-04T00:00:00.000Z" },
  { id: "t8", name: "dangling parent", parentId: "deleted-tag", createdAt: "2026-03-05T00:00:00.000Z", updatedAt: "2026-03-05T00:00:00.000Z" },
  { id: "t9", name: "Tag 2", createdAt: "2026-03-06T00:00:00.000Z", updatedAt: "2026-03-06T00:00:00.000Z" },
  { id: "t10", name: "Tag 10", createdAt: "2026-03-07T00:00:00.000Z", updatedAt: "2026-03-07T00:00:00.000Z" },
  { id: "t11", name: "one bookmark theme", createdAt: "2026-03-08T00:00:00.000Z", updatedAt: "2026-03-08T00:00:00.000Z" },
  { id: "t12", name: "childless parent candidate", createdAt: "2026-03-09T00:00:00.000Z", updatedAt: "2026-03-09T00:00:00.000Z" },
];

// t11 has one bookmark AND a child, so it is a theme rather than a long tail.
TAGS.push({ id: "t13", name: "under t11", parentId: "t11", createdAt: "2026-03-10T00:00:00.000Z", updatedAt: "2026-03-10T00:00:00.000Z" });

const RELATIONS = [
  ...Array.from({ length: 5 }, (_, i) => ({ bookmarkId: `g${i}`, tagId: "t2" })),
  ...Array.from({ length: 3 }, (_, i) => ({ bookmarkId: `r${i}`, tagId: "t3" })),
  ...Array.from({ length: 4 }, (_, i) => ({ bookmarkId: `a${i}`, tagId: "t4" })),
  { bookmarkId: "x1", tagId: "t5" },
  { bookmarkId: "x2", tagId: "t5" },
  { bookmarkId: "y1", tagId: "t9" },
  { bookmarkId: "y2", tagId: "t10" },
  { bookmarkId: "z1", tagId: "t11" },
  { bookmarkId: "z2", tagId: "t8" },
  // t1 has no direct bookmarks: an orphan that is also a parent.
];

const SUGGESTIONS = [
  { id: "s1", tagName: "under review", status: "pending" },
  { id: "s2", tagName: "under review", status: "pending" },
  { id: "s3", tagName: "orphan tag", status: "accepted" },
  { id: "s4", tagName: "AI Agents", status: "pending" },
  { id: "s5", tagName: "", status: "pending" },
];

const items = buildTagLibraryItems({ tags: TAGS, bookmarkTags: RELATIONS, tagSuggestions: SUGGESTIONS });
const rootCandidates = getRootTagCandidates(items);

const FILTERS = ["all", "orphan", "long-tail", "pending", "unparented"];
const SORTS = ["count-desc", "count-asc", "name-asc", "recent"];
const QUERIES = ["", "go", "golang", "AI Agents", "ai agent", "languages", "zzzz", "Tag 1"];

const views = [];
for (const filter of FILTERS) {
  for (const sortOrder of SORTS) {
    for (const searchQuery of QUERIES) {
      views.push({
        filter, sortOrder, searchQuery,
        expected: getVisibleTagLibraryItems({ items, searchQuery, filter, sortOrder }).map((i) => i.tagId),
      });
    }
  }
}

const sidebar = [];
for (const expanded of [false, true]) {
  for (const searchQuery of ["", "go", "zzzz"]) {
    const result = getVisibleSidebarTags({ tags: TAGS, searchQuery, expanded });
    sidebar.push({
      searchQuery, expanded,
      expected: { items: result.items.map((t) => t.id), shouldShowToggle: result.shouldShowToggle },
    });
  }
}

writeFileSync(
  path.join(here, "taglibrary.json"),
  JSON.stringify({
    generatedFrom: "src/options/tagLibrary.ts",
    longTailMaxBookmarks: LONG_TAIL_MAX_BOOKMARKS,
    tags: TAGS.map((t) => ({ id: t.id, name: t.name, aliases: t.aliases ?? [], parentId: t.parentId ?? "", createdAt: t.createdAt, updatedAt: t.updatedAt })),
    bookmarkTags: RELATIONS,
    suggestions: SUGGESTIONS.map((s) => ({ tagName: s.tagName, status: s.status })),
    expected: {
      items,
      orphanCount: countOrphanTags(items),
      longTailCount: countLongTailTags(items),
      removableOrphanIds: getRemovableOrphanTagIds(items),
      unparentedCount: countUnparentedTags(items),
      rootCandidateIds: rootCandidates.map((i) => i.tagId),
      parentCandidates: Object.fromEntries(TAGS.map((t) => [t.id, getParentCandidates(items, t.id).map((i) => i.tagId)])),
      hasParentCandidates: Object.fromEntries(TAGS.map((t) => [t.id, hasParentCandidates(rootCandidates, items.find((i) => i.tagId === t.id))])),
    },
    views,
    sidebar,
  }, null, 2) + "\n"
);
console.log(`wrote taglibrary.json: ${items.length} items, ${views.length} views, ${sidebar.length} sidebar cases`);
