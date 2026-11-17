"""Checks that every sync command asks for its resume cursor through ResumeOptions.

A sync that stopped at the request limit leaves a cursor. The window resumes from it; the
command-line `trove sync` was written separately and did not, so the same command in two places
started a full walk from the top. The fix is one constructor, syncer.ResumeOptions, and the
rule this checker enforces is that every RunOptions a command builds passes through it:

    options := s.syncer.ResumeOptions(ctx, options)     // the window
    options := syncService.ResumeOptions(ctx, syncer.RunOptions{...})  // the CLI

Both shapes are accepted: the call before the literal, and the call on the variable after it.
What is not accepted is a variable that reaches Run without either.

    python3 tools/runcursor.py

It reports candidates, not findings: a test may build the struct directly, and test files are
not scanned.
"""
import os
import re
import sys

ROOTS = ["cmd/trove", "internal/ui"]
LITERAL = re.compile(r"syncer\.RunOptions\{")
ASSIGNED = re.compile(r"^\s*([A-Za-z_]\w*)\s*(?::=|=)\s*")
HELPER_FILE = "internal/usecase/syncer/service.go"
HELPER = re.compile(r"^func \(s \*Service\) ResumeOptions\(", re.M)

if os.path.abspath(os.getcwd()) != os.path.dirname(os.path.dirname(os.path.abspath(__file__))):
    sys.stderr.write(
        "warning: %s is being run from %s; it reads paths relative to the repository root.\n"
        "         cd to the repository root first, or the numbers below will be about something else.\n"
        % (os.path.basename(__file__), os.getcwd()))


def go_files(root):
    found = []
    for base, dirs, names in os.walk(root):
        dirs[:] = [d for d in dirs if d not in (".git", ".toolchain", "build")]
        for name in sorted(names):
            if name.endswith(".go") and not name.endswith("_test.go"):
                found.append(os.path.join(base, name))
    return found


def body_after(source, match):
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


def continued(lines, start, var):
    """Whether the options in var reach ResumeOptions before the function ends.

    The window assigns the struct and then continues it one statement later, under a comment
    that explains why the rule lives in the service. Scanning to the next top-level closing
    brace is what lets that shape count while a run that simply uses the struct does not.
    """
    pattern = re.compile(r"ResumeOptions\([^)]*\b" + re.escape(var) + r"\b")
    for line in lines[start:]:
        if line.startswith("}"):
            return False
        if pattern.search(line):
            return True
    return False


def main():
    found = 0
    examined = 0
    for root in ROOTS:
        for path in go_files(root):
            lines = open(path, encoding="utf-8").read().split("\n")
            for index, line in enumerate(lines):
                if not LITERAL.search(line):
                    continue
                examined += 1
                if "ResumeOptions(" in line:
                    continue
                assigned = ASSIGNED.match(line)
                if assigned and continued(lines, index + 1, assigned.group(1)):
                    continue
                found += 1
                print("%s:%d: RunOptions built without syncer.ResumeOptions" % (path, index + 1))

    source = open(HELPER_FILE, encoding="utf-8").read()
    helper = HELPER.search(source)
    if not helper:
        found += 1
        print("%s: no ResumeOptions method" % HELPER_FILE)
    elif "StartCursor" not in body_after(source, helper):
        found += 1
        print("%s: ResumeOptions no longer sets StartCursor" % HELPER_FILE)

    print("runcursor: %d candidates in %d constructions" % (found, examined), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
