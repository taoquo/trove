#!/usr/bin/env python3
"""Audit the reference picture against the shell: does every visible thing have a render point?

    python3 tools/targetaudit.py            # table on stdout
    python3 tools/targetaudit.py --strict   # exit 1 if anything is unrendered

Why this exists
---------------
The objective's fourth working rule says the question "does the reference's every surface have
somewhere that draws it" has to be answered as its own criterion, not inferred from the geometry
being right. Prose answers rot: a section can be added to this document and later deleted from the
shell, and nothing notices.

So the answer is a table whose two claims are both machine-checked:

  render  the Go symbol named here exists as a drawing method, or as the renderer of that part
  size    the metric constant or the stylesheet selector named here exists

An element whose row names no render point fails, which is what "unrendered" means here.

What this does NOT check: that the size is the *right* number. That is what tools/csstokens.py and
the spec type table are for. This one answers "is it drawn, and does its size have a source".

Exit 0 all rendered, 1 something unrendered (only with --strict), 2 cannot run.
"""

import argparse
import re
import sys
from pathlib import Path

APP = Path(__file__).resolve().parent.parent
UI = APP / "internal/ui"
METRICS = UI / "metrics.go"
CSS_TOKENS = APP.parent.parent / "docs/css-tokens.md"

# element as the reference labels or shows it, the Go symbol that draws it, and where its size comes
# from. `render` entries are checked against the Go source; `size` entries are checked as a constant
# in metrics.go (bare name) or as a selector in docs/css-tokens.md (leading dot).
#
# Deliberately one row per thing a person can point at in the picture, in reading order.
ROWS = [
    # -- sidebar -------------------------------------------------------------------------------
    #
    # Round 506 reshaped this column: the 工作区 overline and the two section headers (with their
    # caret) are gone, and the two lists answer to one segmented control. What the reference
    # picture labels "标签 分区头" and "作者 分区头" is now the pair of tabs, so those two rows
    # name the control rather than a header that no longer exists -- the audit asks "is it
    # drawn", and the answer has to be the thing that draws it.
    #
    # The workspace's state moved out of the title bar and the hero: it is the foot's status
    # row now, above four 26-point commands (app.css:397, :513-517). The brand stays in the
    # title bar, and the tabs took `.segs`.
    ("工作区状态徽章 圆点+文字", "sidebarFooter", "StatusDotSize"),
    ("brand 书签", "brandLockup", "FontSidebarTitleCompact"),
    ("大模型 / 120/2000 / 进度条 / 约剩", "runProgressBar", "ProgressTrackHeight"),
    ("上次同步 10月17日 18:58", "sidebarFooter", "SideFooterStatusFont"),
    ("立即同步", "sidebarFooter", "FooterIconSize"),
    ("标签 / 作者 分段控件", "sidebarFacetTabs", "SidebarTabRowHeight"),
    # Round 507 moved the field out of both bodies and behind one magnifier in the tab row, so
    # the renderer is now the shared field, not either body.
    ("搜索标签名或别名", "sidebarSearchField", "SidebarSearchHeight"),
    # Round 508 removed the reference's two navigation rows from this tab (全部书签 and
    # 待确认标签): the user asked for a tags tab that is the tags, and both rows repeated
    # something already on screen -- the results header names and counts 全部书签, and pending
    # work now sits with the library's 待确认 filter. Nothing above the tag rows is expected
    # here any more, so there is no element to audit.
    ("标签树的每个标签行", "tagRow", "SubRowMinHeight"),
    ("作者 行 (头像 + 名字 + @handle)", "authorRow", "AuthorAvatarSize"),
    ("footer 状态行 + 语言/主题/同步/设置", "sidebarFooter", "FooterIconSize"),
    ("侧栏与主区的分隔线", "sidebar", "WidthSidebarDef"),
    # -- main pane -----------------------------------------------------------------------------
    #
    # Round 509 compacted this block at the user's request (shell-contract.md S24): the 资料库
    # overline, the stacked count under the title, and the toolbar's repeated "N 结果" are
    # gone. These rows name what the compact header draws instead -- the scope title and one
    # count line beside it.
    #
    # Round 510 then moved the active filters out of their own full-width row and into the
    # search's row, on its right (S25). The reference's "活跃筛选:" label is now the group's
    # accessibility label rather than a printed word, so the row below the search is the chip
    # itself: it is what a person points at to remove one condition.
    #
    # The page's name then left the content entirely: it is the title bar's crumb at
    # 13.5/600 (app.css:522), and the count moved to the end of the toolbar's control row.
    ("全部书签 标题", "pageCrumb", "FontPageCrumb"),
    ("68 结果", "toolbar", "FontTiny"),
    ("搜索框 + 放大镜", "searchField", "SearchFieldHeight"),
    ("筛选 (图标+文字)", "toolbar", "IconButtonSize"),
    ("最近保存 (图标+文字)", "toolbar", "IconButtonSize"),
    ("视图切换 grid / list", "toolbar", "IconButtonSize"),
    ("活跃筛选 芯片 (名称 + ×)", "conditionChip", "RadiusPill"),
    ("1144 待确认标签 + 接受全部建议", "pendingSuggestionsBar", "BulkBarPadding"),
    # -- cards ---------------------------------------------------------------------------------
    ("卡片外框与圆角", "bookmarkCard", "RadiusResultCard"),
    ("卡片复选框", "cardSelectToggle", "CardSelectSize"),
    ("卡片顶部满宽媒体", "bookmarkCard", "CardMediaAspect"),
    ("无图时的媒体占位", "bookmarkCard", "CardMediaAspect"),
    ("头像", "bookmarkCard", "CardAvatarSize"),
    ("作者名 / handle / 右上时间戳", "bookmarkCard", "CardAuthorFont"),
    ("正文", "bookmarkCard", "CardCopyFont"),
    ("卡片操作行 💬 ♡ ⤴ ↗", "cardActions", "CardActionSize"),
]

BARE = re.compile(r"(?m)^\s*[A-Z][A-Za-z0-9]*\s*(?:[A-Za-z0-9\[\]*. ]*)?=\s")


def go_methods():
    """Every func in internal/ui as `name` -> file, so a symbol cannot be invented."""
    found = {}
    func = re.compile(r"(?m)^func (?:\([^)]*\) )?([A-Za-z0-9_]+)\(")
    for path in sorted(UI.glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        for m in func.finditer(path.read_text(encoding="utf-8")):
            found.setdefault(m.group(1), path.name)
    return found


def metrics_constants():
    """Every package-level constant, wherever it lives.

    The first version read only metrics.go and reported ProgressTrackHeight missing, which was the
    tool being wrong: the 6px track height is declared in runprogress.go beside the bar that uses
    it. A check that assumes where things live will report a real constant as absent.
    """
    found = set()
    for path in sorted(UI.glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        text = path.read_text(encoding="utf-8")
        # Both forms: a name inside a `const (...)` block, and a one-line `const Name = 6`.
        # The first version knew only the grouped form and reported StatusDotSize missing, which
        # is theme.go:175 written as `const StatusDotSize = 6` -- the tool's second wrong
        # assumption about where a constant can live, in the same round it was added.
        found |= set(re.findall(r"(?m)^\s*([A-Z][A-Za-z0-9]*)\s*(?:[A-Za-z0-9\[\]*. ]*)?=", text))
        found |= set(re.findall(r"(?m)^\s*const\s+([A-Z][A-Za-z0-9]*)\s", text))
    return found


def css_selectors():
    if not CSS_TOKENS.exists():
        return None
    text = CSS_TOKENS.read_text(encoding="utf-8")
    return set(re.findall(r"selector `([^`]+)`", text))


def main(argv):
    parser = argparse.ArgumentParser()
    parser.add_argument("--strict", action="store_true",
                        help="exit 1 when something visible has no render point")
    args = parser.parse_args(argv)

    methods = go_methods()
    constants = metrics_constants()
    selectors = css_selectors()

    missing_render = []
    missing_size = []
    print(f"{'element':46} {'render':28} {'size'}")
    print("-" * 96)
    for element, render, size in ROWS:
        where = methods.get(render)
        render_ok = where is not None
        if size.startswith("."):
            if selectors is None:
                size_ok = None
            else:
                size_ok = size in selectors
        else:
            size_ok = size in constants
        if not render_ok:
            missing_render.append(element)
        if size_ok is False:
            missing_size.append((element, size))
        mark_r = f"{render} ({where})" if render_ok else f"{render}  MISSING"
        mark_s = size if size_ok else f"{size}  MISSING"
        print(f"{element:46} {mark_r:28} {mark_s}")

    print()
    if missing_render:
        print(f"FAIL {len(missing_render)} visible element(s) have no render point:")
        for element in missing_render:
            print(f"       {element}")
    if missing_size:
        print(f"FAIL {len(missing_size)} element(s) name a size that does not exist:")
        for element, size in missing_size:
            print(f"       {element}: {size}")
    if not missing_render and not missing_size:
        print(f"  ok   all {len(ROWS)} visible elements have a render point and a size with a source")
        return 0
    return 1 if args.strict else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
