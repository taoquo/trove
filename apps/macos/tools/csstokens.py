#!/usr/bin/env python3
"""Extract the source's design tokens so the native build can quote them instead of guessing.

    python3 tools/csstokens.py            # print the table
    python3 tools/csstokens.py --write    # regenerate docs/css-tokens.md

Why a tool and not a table in a document
----------------------------------------
Round 449 compared two pictures -- the target's and this build's -- and the gap was large. The
target is the source product rendered (all nine strings in it are in src/options/i18n/copy.ts), so
the honest way to close that gap is to quote the source's own CSS, not to reverse-engineer sizes
from a compressed screenshot. A hand-copied table would be correct the day it was written; this is
the same argument tools/baseline.sh makes about the numbers it prints.

What it reads
-------------
src/styles/extension.css, for two kinds of thing:

  * every `--*` custom property, grouped by the block that declares it, because the dark block
    redefines the same names and the two must not be conflated;
  * for a fixed list of classes, the layout and typography properties that decide how they look.

It deliberately does not try to be a CSS parser. It matches `selector { ... }` blocks, which is all
extension.css contains, and reports how many blocks it found and how many of the wanted classes it
matched -- so a silent miss shows up as a count, not as a plausible-looking gap.
"""

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent.parent  # repo root
CSS = ROOT / "src/styles/extension.css"
OUT = ROOT / "docs/css-tokens.md"

# The properties that decide how something looks. Colour is excluded on purpose: round 449 was
# told to leave the theme alone, and mixing roles and values here would make the table look like
# a palette change.
PROPS = (
    "font-size", "font-weight", "font-family", "line-height", "letter-spacing",
    "width", "height", "min-height", "max-height", "min-width", "max-width",
    "padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
    "margin", "margin-top", "margin-bottom",
    "gap", "row-gap", "column-gap",
    "border-radius", "aspect-ratio", "grid-template-columns",
    "flex-direction", "align-items", "justify-content", "display",
    "position", "top", "right", "bottom", "left", "opacity", "overflow",
    "text-transform", "white-space", "object-fit",
)

# The classes this build has to reproduce. Grouped the way the screens are built, so a missing
# group is a missing screen rather than one missing selector.
CLASSES = {
    # Everything here is a name that exists in extension.css or in the TSX that emits it. The first
    # draft of this list guessed -- `options-autotag-panel`, `options-card-checkbox`, `options-results-title`
    # and nine others -- and the tool reported 40 of 67 matched, which was the guess being wrong
    # rather than the source being incomplete. The real names are what the tool now asks for.
    "shell": [".options-sidebar-shell", ".options-main-header", ".options-main-header-row",
              ".options-main-header-top", ".options-main-header-copy"],
    # Round 464 answered the last of section 2.3's "not yet extracted" list. Every name here was
    # checked against the stylesheet before being written down, because this list's own history is
    # a warning: a guessed draft matched 40 of 67 and the tool was blamed for the guess.
    "sidebar": [".options-sidebar-hero", ".options-sidebar-stats", ".options-sidebar-config",
                ".options-sidebar-autotag", ".options-sidebar-search-input", ".options-nav-count",
                ".options-sidebar-section", ".options-sidebar-footer",
                ".options-nav-row", ".options-nav-row-main", ".options-nav-row-icon",
                ".options-sidebar-section-head", ".options-sidebar-section-toggle"],
    "filters": [".options-active-filters", ".options-active-filters-label", ".options-chip"],
    "type": [".options-display-title", ".options-overline", ".options-results-value",
             ".options-main-header-summary-value"],
    "autotag": [".options-autotag-progress", ".options-autotag-progress-head",
                ".options-autotag-progress-phase", ".options-autotag-progress-count",
                ".options-autotag-progress-track", ".options-autotag-progress-fill",
                ".options-autotag-progress-foot", ".options-autotag-progress-stalled"],
    "search": [".options-toolbar-search", ".options-toolbar-shell", ".options-toolbar-controls",
               ".options-toolbar-field", ".options-toolbar-action", ".options-toolbar-grid",
               ".options-toolbar-primary", ".options-toolbar-results-meta",
               ".options-toolbar-context-row", ".options-toolbar-summary-row"],
    "banner": [".options-bulk-actions-bar", ".options-bulk-actions-meta",
               ".options-bulk-actions-controls", ".options-secondary-button"],
    "review": [".options-tag-review-panel", ".options-tag-review-head", ".options-tag-review-row",
               ".options-tag-review-check", ".options-tag-review-kind", ".options-tag-review-actions"],
    "grid": [".options-results-grid", ".options-results-masonry", ".options-results-stack-grid"],
    "card": [".options-result-card", ".options-card-body", ".options-card-layout", ".options-card-column",
             ".options-card-main", ".options-card-avatar", ".options-card-content", ".options-card-header",
             ".options-card-author", ".options-card-author-name", ".options-card-author-handle",
             ".options-card-timestamp", ".options-card-copy-wrap", ".options-card-copy",
             ".options-card-tags", ".options-card-tag", ".options-card-actions", ".options-card-action",
             ".options-card-media", ".options-bookmark-select-check", ".options-bookmark-select-toggle",
             ".options-result-card-selected"],
    "inspector": [".options-inspector-shell", ".options-inspector-header", ".options-inspector-section",
                  ".options-detail-hero", ".options-detail-summary-card", ".options-detail-layout",
                  ".options-detail-rail", ".options-inspector-divider",
                  ".options-detail-section-overline"],
}

BLOCK = re.compile(r"(?m)^[ \t]*(?P<sel>[^{}\n]+?)\s*\{(?P<body>[^}]*)\}")
VAR = re.compile(r"(?m)^[ \t]*(--[a-z0-9-]+)\s*:\s*([^;]+);")


def blocks():
    text = CSS.read_text(encoding="utf-8")
    for m in BLOCK.finditer(text):
        yield m.group("sel").strip(), m.group("body")


def line_of(text, needle, start=0):
    return text.count("\n", 0, text.find(needle, start)) + 1


def wanted_decls(body):
    out = []
    for decl in body.split(";"):
        decl = decl.strip()
        if not decl or ":" not in decl:
            continue
        prop = decl.split(":", 1)[0].strip()
        if prop in PROPS:
            out.append(" ".join(decl.split()))
    return out


def main(argv):
    if not CSS.exists():
        print(f"cannot run: {CSS} is missing", file=sys.stderr)
        return 2
    text = CSS.read_text(encoding="utf-8")

    # 1. every custom property, in document order, with the selector that declared it
    vars_by_block = []
    for sel, body in blocks():
        found = VAR.findall(body)
        if found:
            vars_by_block.append((sel, found))

    # 2. the wanted classes
    matched = {}
    wanted = [(cls, group) for group, classes in CLASSES.items() for cls in classes]
    for m in BLOCK.finditer(text):
        sel, body = m.group("sel").strip(), m.group("body")
        for cls, group in wanted:
            if cls not in sel:
                continue
            decls = wanted_decls(body)
            if not decls:
                continue
            matched.setdefault((group, cls), []).append(
                (sel, line_of(text, m.group(0)), decls)
            )

    lines = []
    add = lines.append
    add("# CSS tokens, extracted from the source")
    add("")
    add("Generated by `python3 apps/macos/tools/csstokens.py --write` from")
    add("`src/styles/extension.css`. **Do not edit by hand** -- regenerate.")
    add("")
    add(f"- custom properties declared: **{sum(len(f) for _, f in vars_by_block)}**")
    add(f"- declaring blocks: **{len(vars_by_block)}**")
    total = sum(len(classes) for classes in CLASSES.values())
    add(f"- classes asked for: **{total}**; matched: **{len(matched)}**")
    add("")

    add("## Custom properties")
    add("")
    for sel, found in vars_by_block:
        add(f"### `{sel}`")
        add("")
        add("| name | value |")
        add("|---|---|")
        for name, value in found:
            add(f"| `{name}` | `{value.strip()}` |")
        add("")

    add("## Class measurements")
    add("")
    for group, classes in CLASSES.items():
        add(f"### {group}")
        add("")
        for cls in classes:
            hits = matched.get((group, cls))
            if not hits:
                add(f"- `{cls}` -- **not found**")
                continue
            for sel, ln, decls in hits[:2]:
                add(f"- `{cls}` (line {ln}, selector `{sel}`): " + "; ".join(decls))
        add("")

    missing = [
        f"{g}/{c}" for g, cs in CLASSES.items() for c in cs if (g, c) not in matched
    ]
    add("## What this run did not find")
    add("")
    add(f"{len(missing)} of {total} selectors were not matched. A selector that does not exist in")
    add("the source is not an error -- it may be named differently -- but it must not be assumed.")
    add("")
    for m in missing:
        add(f"- `{m}`")
    add("")

    body = "\n".join(lines) + "\n"

    if "--write" in argv:
        OUT.parent.mkdir(parents=True, exist_ok=True)
        OUT.write_text(body, encoding="utf-8")
        print(f"wrote {OUT.relative_to(ROOT)} "
              f"({sum(len(f) for _, f in vars_by_block)} vars, "
              f"{len(matched)}/{total} selectors)")
    else:
        print(body)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
