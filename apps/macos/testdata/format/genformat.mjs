// Generates the display-formatting fixture from the real TypeScript
// (src/options/utils.ts), so the Go port is checked against the real formatter rather
// than a reading of it.
//
//   node apps/macos/testdata/format/genformat.mjs
//
// Compact notation is the reason this exists: it is not a k-suffix rule. Chinese
// abbreviates at 万 and has no thousands step, and no amount of reading the call site
// says so -- running the formatter does.
import { writeFileSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../../../..");

const {
  formatCompactCount, truncateText, getVisibleCardTagNames, isVideoMediaType, formatTimestamp,
  getListCopyDensityClass, getListCardTemplateClass,
} = await import("file://" + path.join(repoRoot, "src/options/utils.ts"));

const VALUES = [0, 1, 2, 9, 10, 99, 100, 999, 1000, 1001, 1234, 9999, 10000, 12345, 99999,
  100000, 123456, 999999, 1000000, 1234567, 9999999, 12345678, 999999999];

const compact = [];
for (const locale of ["en", "zh-CN"]) {
  for (const value of VALUES) {
    compact.push({ locale, value, expected: formatCompactCount(value, locale) });
  }
}

const TEXTS = ["", "a", "x".repeat(118), "x".repeat(119), "x".repeat(120), "x".repeat(121), "x".repeat(200)];
const truncated = TEXTS.map((text) => ({
  length: text.length,
  // The body is not written to the fixture: 200 characters per case is noise, and the
  // length plus the output length is what the rule is about.
  expectedLength: truncateText(text).length,
  expectedTail: truncateText(text).slice(-4),
}));

const tagCases = [];
for (const viewMode of ["grid", "list"]) {
  for (const count of [0, 1, 2, 3, 5, 12]) {
    const names = Array.from({ length: count }, (_, i) => `t${i}`);
    tagCases.push({ viewMode, count, expected: getVisibleCardTagNames(names, viewMode) });
  }
}

const videoCases = ["video", "animated_gif", "photo", "image", "", undefined].map((type) => ({
  type: type ?? null,
  expected: isVideoMediaType(type),
}));

const timestampCases = [
  { value: null, locale: "en", expected: formatTimestamp(undefined, "en") },
  { value: null, locale: "zh-CN", expected: formatTimestamp(undefined, "zh-CN") },
];

// The density rule reads the text length, so the cases walk both thresholds and both
// sides of the media and tags flags.
const densityCases = [];
for (const hasMedia of [false, true]) {
  for (const hasTags of [false, true]) {
    for (const length of [0, 1, 47, 48, 49, 100, 239, 240, 241, 1000]) {
      const text = "x".repeat(length);
      const copyDensityClass = getListCopyDensityClass(text, hasMedia, hasTags);
      densityCases.push({
        length, hasMedia, hasTags, copyDensityClass,
        templateClass: getListCardTemplateClass(hasMedia, hasTags, copyDensityClass),
      });
    }
  }
}

// Whitespace is trimmed before the length is taken, which is easy to miss.
const trimCases = [
  { text: "   ", hasMedia: false, hasTags: false, expected: getListCopyDensityClass("   ", false, false) },
  { text: " ".repeat(60) + "short", hasMedia: false, hasTags: false, expected: getListCopyDensityClass(" ".repeat(60) + "short", false, false) },
];

// Every keyframe animation the stylesheet applies, read from the stylesheet rather than
// transcribed: the durations and curves differ per selector and two of them disagree
// with the motion tokens.
const cssPath = path.join(repoRoot, "src/styles/extension.css");
const css = readFileSync(cssPath, "utf8");
const animationUses = [];
{
  const lines = css.split("\n");
  let selector = "";
  for (const line of lines) {
    const sel = line.match(/^\s{2}(\.[a-z0-9-]+)\s*\{/);
    if (sel) selector = sel[1];
    const anim = line.match(/^\s*animation:\s*([a-z-]+)\s+([0-9]+)ms\s+([^;]+);/);
    if (anim && anim[1] !== "none") {
      animationUses.push({ selector, keyframes: anim[1], durationMs: Number(anim[2]), curve: anim[3].trim() });
    }
  }
}

// The keyframes' own transforms, parsed from the stylesheet: an entrance is a fade
// plus a scale and/or a lift, and the numbers differ per overlay.
const keyframeBodies = {};
{
  const re = /@keyframes\s+([a-z-]+)\s*\{([\s\S]*?)\n  \}/g;
  let m;
  while ((m = re.exec(css)) !== null) {
    const body = m[2];
    const fromScale = body.match(/from\s*\{[^}]*scale\(([0-9.]+)\)/);
    const fromY = body.match(/from\s*\{[^}]*translateY\((-?[0-9.]+)px\)/);
    const centered = /from\s*\{[^}]*translateX\(-50%\)/.test(body);
    const fade = /from\s*\{[^}]*opacity:\s*0/.test(body);
    keyframeBodies[m[1]] = {
      scaleFrom: fromScale ? Number(fromScale[1]) : 1,
      offsetYFrom: fromY ? Number(fromY[1]) : 0,
      centeredX: centered,
      fades: fade
    };
  }
}

writeFileSync(
  path.join(here, "format.json"),
  JSON.stringify({
    generatedFrom: "src/options/utils.ts",
    compact,
    truncated,
    tagCases,
    videoCases,
    timestampCases,
    densityCases,
    trimCases,
    animationUses,
    keyframeBodies,
    defaultTruncateLength: 120,
  }, null, 2) + "\n"
);
console.log(`wrote format.json: ${Object.keys(keyframeBodies).length} keyframes, ${animationUses.length} animations, ${compact.length} counts, ${truncated.length} texts, ${tagCases.length} tag cases, ${densityCases.length} density cases`);
