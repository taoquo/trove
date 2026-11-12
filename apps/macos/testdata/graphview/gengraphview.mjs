// Generates the graph-view-model fixture from the real TypeScript
// (src/options/graph/graphModel.ts), so the Go port is checked against the source
// rather than a re-reading of it -- including the layout arithmetic, where a rounding
// difference would move every node.
//
//   node apps/macos/testdata/graphview/gengraphview.mjs
import { writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../../..");

const { buildGraphViewModel, getGraphNeighbourTagIds, EMPTY_GRAPH_SCOPE } = await import(
  "file://" + path.join(repoRoot, "src/options/graph/graphModel.ts")
);

const tag = (id, name, extra = {}) => ({ id, name, ...extra });
const rel = (bookmarkId, tagId) => ({ bookmarkId, tagId });
const bm = (id, authorHandle) => ({ id, tweetId: id, authorHandle });

const TAGS = [
  tag("lang", "Languages"),
  tag("go", "Go", { parentId: "lang", aliases: ["golang"] }),
  tag("rust", "Rust", { parentId: "lang" }),
  tag("sql", "SQL"),
  tag("db", "Databases"),
  tag("pg", "PostgreSQL", { parentId: "db" }),
  tag("sqlite", "SQLite", { parentId: "db" }),
  tag("k8s", "Kubernetes"),
  tag("docker", "Docker"),
  tag("web", "Web"),
  tag("css", "CSS", { parentId: "web" }),
  tag("html", "HTML", { parentId: "web" }),
  tag("js", "JavaScript", { parentId: "web" }),
  tag("ts", "TypeScript", { parentId: "web" }),
  tag("cafe", "Café"),
  tag("Tag 1", "Tag 1"),
  tag("Tag 2", "Tag 2"),
  tag("Tag 10", "Tag 10"),
];

const BOOKMARKS = [
  bm("b1", "ada"), bm("b2", "ada"), bm("b3", "ada"),
  bm("b4", "grace"), bm("b5", "grace"),
  bm("b6", "alan"), bm("b7", "@ALAN"),
  bm("b8", "  linus  "), bm("b9", "linus"),
];

const RELATIONS = [
  rel("b1", "go"), rel("b1", "k8s"), rel("b1", "docker"),
  rel("b2", "go"), rel("b2", "sql"),
  rel("b3", "k8s"), rel("b3", "sql"),
  rel("b4", "rust"), rel("b4", "k8s"),
  rel("b5", "rust"), rel("b5", "sqlite"),
  rel("b6", "css"), rel("b6", "html"),
  rel("b7", "js"), rel("b7", "ts"),
  rel("b8", "pg"), rel("b8", "sql"),
  rel("b9", "Tag 1"), rel("b9", "Tag 2"), rel("b9", "Tag 10"),
  rel("b1", "missing"),
];

for (let i = 0; i < 70; i += 1) {
  const id = `filler-${i}`;
  TAGS.push(tag(id, `Filler ${i}`));
  RELATIONS.push(rel(`filler-bm-${i}`, id));
  BOOKMARKS.push(bm(`filler-bm-${i}`, i % 2 ? "ada" : "grace"));
}

const CASES = [
  { name: "empty scope shows everything under the limit" },
  { name: "focus on one tag keeps its neighbours", scope: { activeTagIds: ["go"], activeAuthorHandles: [], searchQuery: "" } },
  { name: "focus on all is no focus", scope: { activeTagIds: ["all"], activeAuthorHandles: [], searchQuery: "" } },
  { name: "author scope filters the data before the graph", scope: { activeTagIds: ["all"], activeAuthorHandles: ["ada"], searchQuery: "" } },
  { name: "author scope normalises the handle", scope: { activeTagIds: ["all"], activeAuthorHandles: ["@ALAN", " Linus "], searchQuery: "" } },
  { name: "search matches a name", scope: { activeTagIds: ["all"], activeAuthorHandles: [], searchQuery: "sql" } },
  { name: "search matches an alias", scope: { activeTagIds: ["all"], activeAuthorHandles: [], searchQuery: "golang" } },
  { name: "search is case-insensitive", scope: { activeTagIds: ["all"], activeAuthorHandles: [], searchQuery: "KUBERNETES" } },
  { name: "search that matches nothing", scope: { activeTagIds: ["all"], activeAuthorHandles: [], searchQuery: "zzzz" } },
  { name: "a small node limit hides the rest", nodeLimit: 5 },
  { name: "a node limit of one", nodeLimit: 1 },
  { name: "a large node limit keeps everything", nodeLimit: 500 },
  { name: "authors included", includeAuthors: true, authorLimit: 3 },
  { name: "search with a focused tag", scope: { activeTagIds: ["lang"], activeAuthorHandles: [], searchQuery: "go" } },
];

const cases = CASES.map((entry) => {
  const scope = entry.scope ?? EMPTY_GRAPH_SCOPE;
  const options = {
    scope,
    ...(entry.nodeLimit !== undefined ? { nodeLimit: entry.nodeLimit } : {}),
    ...(entry.authorLimit !== undefined ? { authorLimit: entry.authorLimit } : {}),
    includeAuthors: entry.includeAuthors ?? false,
  };

  const model = buildGraphViewModel({
    tags: TAGS,
    bookmarkTags: RELATIONS,
    bookmarks: BOOKMARKS,
    ...options,
  });

  const neighbours = {};
  for (const tagId of ["go", "k8s", "css", "nothing-here"]) {
    neighbours[tagId] = [...getGraphNeighbourTagIds(model.edges, tagId)].sort();
  }

  return {
    name: entry.name,
    scope,
    nodeLimit: options.nodeLimit ?? null,
    authorLimit: options.authorLimit ?? null,
    includeAuthors: options.includeAuthors,
    expected: model,
    neighbours,
  };
});

writeFileSync(
  path.join(here, "graphview.json"),
  JSON.stringify(
    { generatedFrom: "src/options/graph/graphModel.ts", tags: TAGS, bookmarks: BOOKMARKS, bookmarkTags: RELATIONS, cases },
    null,
    2
  ) + "\n"
);

const nodes = cases.reduce((sum, c) => sum + c.expected.nodes.length, 0);
console.log(`wrote graphview.json: ${cases.length} cases, ${nodes} nodes`);
