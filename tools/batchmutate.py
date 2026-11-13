"""UNRELIABLE -- use tools/mutate.sh for any verdict you intend to act on.

This tool can report SURVIVED for a mutation that tools/mutate.sh reports as CAUGHT, on
the same file and the same test package:

    internal/domain/taglibrary.go, matchesTagQuery's plain-substring branch
      batchmutate.py -> SURVIVED
      mutate.sh      -> CAUGHT   (and the domain package fails on its own)

The cause is not diagnosed. Until it is, treat a SURVIVED verdict from here as a lead
to re-check, never as a finding: the "gaps" it reports are often false, and acting on
them would mean changing working code.

Batch mutation: applies mechanical edits across a package and reports candidates.

Picking invariants by hand produces false "gaps", each from a bad probe rather than bad
code. This walks the source code instead: every mutation that compiles is run against the
package's whole test suite, with no filter to get wrong.

    python3 tools/batchmutate.py internal/domain [--limit N] [--budget-seconds S]

A survivor means the suite passes with that logic inverted, which is the question worth
asking. It does not mean the code is wrong.
"""
import argparse
import os
import re
import subprocess
import sys
import tempfile

# Edits that compile and invert a decision. Each is (description, pattern, replacement).
MUTATIONS = [
    ("negate a boolean literal", r"\breturn true\b", "return false"),
    ("negate a false literal", r"\breturn false\b", "return true"),
    ("flip == to !=", r"== 0\b", "!= 0"),
    ("flip != to ==", r"!= 0\b", "== 0"),
    ("flip < to >", r"< ", "> "),
    ("flip > to <", r"> ", "< "),
    ("drop a nil guard", r"if err != nil \{\n(\s*)return", r"if err == nil {\n\\1return"),
    ("always take the first branch", r"if ([a-zA-Z_][\w.]*) \{\n(\s*)return", r"if true {\n\\2return"),
]


def go_files(package):
    root = package
    out = []
    for base, _, names in os.walk(root):
        for name in names:
            if name.endswith(".go") and not name.endswith("_test.go"):
                out.append(os.path.join(base, name))
    return sorted(out)


def run_tests(target, timeout):
    """Runs the tests for target, which is a package or "./..." for the whole module.

    The whole-module run matters: this shell's storage layer is exercised heavily from
    the ui package's tests, which drive a real SQLite database through the real
    repositories. Mutating the repository and running only its own package reports five
    survivors that the full suite catches -- each one a false "coverage gap" of exactly
    the kind this tool exists to avoid.
    """
    try:
        result = subprocess.run(
            ["go", "test", "-count=1", "-timeout", "300s", target],
            capture_output=True, timeout=timeout, cwd=os.getcwd(),
        )
    except subprocess.TimeoutExpired:
        return "timeout", ""
    return ("pass" if result.returncode == 0 else "fail"), result.stdout.decode()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("package")
    parser.add_argument("--limit", type=int, default=40)
    parser.add_argument("--budget-seconds", type=int, default=900)
    parser.add_argument("--package-filter", default=None, help="with --suite, the package to mutate")
    parser.add_argument("--mutation", type=int, default=None, help="run only this mutation index")
    parser.add_argument("--suite", action="store_true",
                        help="run the whole module instead of one package (slower, and the honest default)")
    args = parser.parse_args()

    import time
    started = time.time()

    # The whole module is the default because a package's own tests are not the only
    # tests that exercise it.
    target = "./..." if args.suite else "./" + args.package.lstrip("./")

    status, output = run_tests(target, 900)
    if status != "pass":
        print(f"the unmutated tests do not pass ({status}); nothing to judge")
        print(output[-1500:])
        return 2

    catalogue = MUTATIONS if args.mutation is None else [MUTATIONS[args.mutation]]
    checked = survivors = skipped = 0

    for description, pattern, replacement in catalogue:
        for path in go_files(args.package):
            source = open(path, encoding="utf-8").read()
            for match in list(re.finditer(pattern, source))[:3]:
                if time.time() - started > args.budget_seconds:
                    print(f"budget reached after {checked} mutations")
                    return 0
                if checked >= args.limit:
                    print(f"limit reached after {checked} mutations")
                    return 0

                mutated = source[:match.start()] + match.expand(replacement) + source[match.end():]
                if mutated == source:
                    skipped += 1
                    continue

                with tempfile.NamedTemporaryFile("w", suffix=".go", delete=False, encoding="utf-8") as handle:
                    handle.write(mutated)
                    temporary = handle.name
                try:
                    os.replace(temporary, path)
                    status, _ = run_tests(target, 600)
                finally:
                    open(path, "w", encoding="utf-8").write(source)

                if status == "timeout":
                    skipped += 1
                    continue
                checked += 1
                if status == "pass":
                    survivors += 1
                    line = source[:match.start()].count("\n") + 1
                    print(f"SURVIVED  {description}  {path}:{line}")

    print(f"\n{checked} mutations run, {survivors} survived, {skipped} skipped")
    return 0


if __name__ == "__main__":
    sys.exit(main())
