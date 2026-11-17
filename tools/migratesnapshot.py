"""Checks that Migrate copies the database before it applies a migration.

A migration is not reversible. The one thing that makes an upgrade safe to try is the copy the
code takes beside the library first: when a later step has to be undone by hand, the copy is
what it is undone from.

The property is easy to lose while editing. The snapshot block sits between two early returns
and a loop that looks like it does the work, and deleting it leaves every test green -- the
schema still arrives at the right version, because the copy is not part of the schema. This is
the assertion that notices:

  * Migrate calls snapshotBeforeMigration for a library that already has rows;
  * the call happens before the first applyMigration, not after the loop;
  * the guard still excludes a fresh file, which has nothing to copy;
  * snapshotBeforeMigration still takes the copy through SnapshotInto.

    python3 tools/migratesnapshot.py

It reports candidates, not findings: a renamed function is a candidate the reader resolves.
"""
import os
import re
import sys

PATH = "internal/storage/sqlite/migrations.go"
MIGRATE = re.compile(r"^func \(db \*DB\) Migrate\(\) error \{", re.M)
SNAPSHOT_FUNC = re.compile(r"^func \(db \*DB\) snapshotBeforeMigration\(", re.M)

# The tools here read their inputs relative to the repository root. A wrong directory produces
# something that looks like a result, which is the shape this project keeps meeting.
if os.path.abspath(os.getcwd()) != os.path.dirname(os.path.dirname(os.path.abspath(__file__))):
    sys.stderr.write(
        "warning: %s is being run from %s; it reads paths relative to the repository root.\n"
        "         cd to the repository root first, or the numbers below will be about something else.\n"
        % (os.path.basename(__file__), os.getcwd()))


def body_after(source, match):
    """The text of the function starting at match, by brace depth."""
    start = source.index("{", match.end() - 1)
    depth = 0
    for i in range(start, len(source)):
        if source[i] == "{":
            depth += 1
        elif source[i] == "}":
            depth -= 1
            if depth == 0:
                return source[start:i + 1]
    return source[start:]


def main():
    found = 0
    try:
        source = open(PATH, encoding="utf-8").read()
    except OSError as err:
        sys.stderr.write("migratesnapshot: cannot read %s: %s\n" % (PATH, err))
        return 2

    migrate = MIGRATE.search(source)
    if not migrate:
        print("%s: no Migrate method" % PATH)
        sys.stderr.write("migratesnapshot: %d candidates\n" % 1)
        return 0

    body = body_after(source, migrate)
    snapshot_at = body.find("db.snapshotBeforeMigration(")
    apply_at = body.find("db.applyMigration(")
    guard_at = body.find("current > 0")

    if snapshot_at < 0:
        found += 1
        print("%s: Migrate applies migrations without a snapshot first" % PATH)
    elif apply_at >= 0 and snapshot_at > apply_at:
        found += 1
        print("%s: Migrate snapshots after it has already applied a migration" % PATH)
    elif guard_at < 0 or guard_at > snapshot_at:
        found += 1
        print("%s: the snapshot is not guarded by 'current > 0'" % PATH)

    helper = SNAPSHOT_FUNC.search(source)
    if not helper:
        found += 1
        print("%s: no snapshotBeforeMigration method" % PATH)
    elif "SnapshotInto(" not in body_after(source, helper):
        found += 1
        print("%s: snapshotBeforeMigration no longer copies through SnapshotInto" % PATH)

    print("migratesnapshot: %d candidates" % found, file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
