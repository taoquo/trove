"""Checks that the board still says what is being worked on, and with what evidence.

The loop's memory is docs/loop/board.md, which is only worth having if it cannot rot
silently. The rules this enforces are the ones that stop it rotting:

  * at most one slice is `doing` -- the WIP=1 rule; two is how a board becomes a wish list
  * a `doing` or `done` slice has a DoR and an acceptance command -- no "start and see"
  * a `done` slice has evidence -- "looks right" is not evidence
  * a `doing` slice was touched within seven days -- a row nobody has moved is a row
    nobody is working on
  * status and profile are from the fixed sets -- a typo must not read as a new state

It deliberately does not check that the evidence is *good*; that is the reviewer's job and
the reason evidence has levels. It checks that the claim is present and current, which is
the part a script can settle.

    python3 tools/statuscheck.py
"""
import datetime
import os
import sys

BOARD = os.path.join("docs", "loop", "board.md")
STATUSES = {"todo", "doing", "blocked", "done"}
PROFILES = {"behavior", "visual", "data"}
REQUIRED = ["ID", "链路", "切片", "Profile", "Status", "DoR", "验收命令", "证据", "Updated"]
STALE_DAYS = 7


def repo_root():
    here = os.path.dirname(os.path.abspath(__file__))
    while here != "/":
        if os.path.isdir(os.path.join(here, "src")) and os.path.isdir(os.path.join(here, "docs")):
            return here
        here = os.path.dirname(here)
    raise SystemExit("statuscheck: could not find the repository root above this file")


def section(text, heading):
    """The lines under `## <heading>`, up to the next `## `."""
    lines = text.splitlines()
    start = None
    for i, line in enumerate(lines):
        if line.strip() == "## " + heading:
            start = i + 1
            break
    if start is None:
        return []
    out = []
    for line in lines[start:]:
        if line.startswith("## "):
            break
        out.append(line)
    return out


def table(lines):
    """The first markdown table in `lines`, as a list of cell lists."""
    rows = []
    started = False
    for line in lines:
        stripped = line.strip()
        if not stripped.startswith("|"):
            if started:
                break
            continue
        cells = [cell.strip() for cell in stripped.strip("|").split("|")]
        if not started:
            started = True
            rows.append(cells)  # header
            continue
        if all(set(cell) <= set("-: ") and cell for cell in cells):
            continue  # separator
        rows.append(cells)
    return rows


def filled(cell):
    return cell not in ("", "—", "-", "–", "none", "None")


def parse_date(cell):
    try:
        return datetime.date.fromisoformat(cell)
    except ValueError:
        return None


def main():
    root = repo_root()
    path = os.path.join(root, BOARD)
    try:
        with open(path, encoding="utf-8") as handle:
            text = handle.read()
    except OSError as exc:
        print("board: cannot read %s: %s" % (BOARD, exc))
        return 1

    rows = table(section(text, "切片"))
    if not rows:
        print("board: no 切片 table; the loop has no current slice")
        return 1

    header = rows[0]
    missing = [name for name in REQUIRED if name not in header]
    if missing:
        print("board: 切片 table is missing column(s): %s" % ", ".join(missing))
        return 1
    index = {name: header.index(name) for name in REQUIRED}

    problems = []
    seen = set()
    doing = []
    today = datetime.date.today()

    for number, cells in enumerate(rows[1:], start=2):
        if len(cells) < len(header):
            problems.append("board: row %d has %d cells, the header has %d" % (number, len(cells), len(header)))
            continue
        row = {name: cells[index[name]] for name in REQUIRED}
        ident, status = row["ID"], row["Status"]

        if not ident:
            problems.append("board: row %d has no ID" % number)
        elif ident in seen:
            problems.append("board: two slices share the id %s" % ident)
        seen.add(ident)

        if status not in STATUSES:
            problems.append("board: %s has status %q, not one of todo/doing/blocked/done" % (ident, status))
            continue
        if row["Profile"] not in PROFILES:
            problems.append("board: %s has profile %q, not one of behavior/visual/data" % (ident, row["Profile"]))
        if status == "doing":
            doing.append(ident)

        if status in ("doing", "done"):
            if row["DoR"] != "✅":
                problems.append("board: %s is %s without a DoR" % (ident, status))
            if not filled(row["验收命令"]):
                problems.append("board: %s is %s without an acceptance command" % (ident, status))
        if status == "done" and not filled(row["证据"]):
            problems.append("board: %s is done without evidence" % ident)
        if status == "blocked" and not filled(row["证据"]):
            problems.append("board: %s is blocked without a reason (put it in 证据)" % ident)

        if status in ("doing", "done"):
            day = parse_date(row["Updated"])
            if day is None:
                problems.append("board: %s has no YYYY-MM-DD Updated date" % ident)
            elif status == "doing" and (today - day).days > STALE_DAYS:
                problems.append(
                    "board: %s has been doing since %s (%d days); split it, drop it or move it"
                    % (ident, row["Updated"], (today - day).days)
                )

    if len(doing) > 1:
        problems.append(
            "board: %d slices are doing (%s); exactly one Now is allowed" % (len(doing), ", ".join(doing))
        )

    for problem in problems:
        print(problem)
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
