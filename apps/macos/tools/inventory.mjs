#!/usr/bin/env node
/**
 * Extracts the replication inventory from the extension source.
 *
 * The point of generating this rather than writing it by hand is that a hand-written
 * list goes stale the moment the stylesheet changes, and a stale list is worse than
 * no list: it reports completeness that no longer holds.
 *
 * Run: node apps/macos/tools/inventory.mjs
 */
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const ROOT = new URL("../../..", import.meta.url).pathname.replace(/\/$/, "");
const CSS = join(ROOT, "src/styles/extension.css");

/** Every class selector in the stylesheet, with the line it first appears on. */
function classes() {
  const source = readFileSync(CSS, "utf8").split("\n");
  const found = new Map();
  source.forEach((line, index) => {
    // A selector line, not a declaration: skip lines inside a rule body.
    const trimmed = line.trim();
    if (trimmed.startsWith("--") || /^[a-z-]+:/.test(trimmed)) return;
    for (const match of line.matchAll(/\.([a-z][a-z0-9-]*)/g)) {
      const name = match[1];
      if (!found.has(name)) found.set(name, index + 1);
    }
  });
  return found;
}

/** The token blocks, so the inventory can list them too. */
function tokens() {
  const source = readFileSync(CSS, "utf8");
  const blocks = {};
  // Light is :root { ... } and dark is :root[data-theme="dark"] { ... }.
  const re = /:root(\[data-theme="dark"\])?\s*\{([\s\S]*?)\n  \}/g;
  let match;
  while ((match = re.exec(source))) {
    const model = match[1] ? "dark" : "light";
    const body = match[2];
    blocks[model] = blocks[model] || {};
    for (const decl of body.matchAll(/--([a-z0-9-]+):\s*([^;]+);/g)) {
      blocks[model][decl[1]] = decl[2].trim();
    }
  }
  return blocks;
}

/** Source modules, grouped by directory, so functional surfaces are visible. */
function modules() {
  const out = [];
  const walk = (dir) => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry);
      if (statSync(full).isDirectory()) walk(full);
      else if (/\.(ts|tsx|css)$/.test(entry)) out.push(relative(ROOT, full));
    }
  };
  walk(join(ROOT, "src"));
  return out.sort();
}

const cls = classes();
const tk = tokens();
const mods = modules();

const families = {};
for (const name of cls.keys()) {
  const prefix = name.includes("-") ? name.split("-")[0] : "(bare)";
  families[prefix] = (families[prefix] || 0) + 1;
}

const light = Object.keys(tk.light || {}).length;
const dark = Object.keys(tk.dark || {}).length;

console.log(JSON.stringify({
  counts: {
    classes: cls.size,
    tokensLight: light,
    tokensDark: dark,
    modules: mods.length,
  },
  families,
  classes: Object.fromEntries([...cls.entries()].sort()),
  tokens: { light: tk.light || {}, dark: tk.dark || {} },
  modules: mods,
}, null, 2));
