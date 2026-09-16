#!/usr/bin/env python3

import argparse
import re
import subprocess
import sys


IDENTITY_RE = re.compile(r"^\s*(.+?)\s*<([^<>]+)>\s*$")
SIGNOFF_RE = re.compile(
    r"^Signed-off-by:\s*(.+?)\s*<([^<>]+)>\s*$",
    re.IGNORECASE | re.MULTILINE,
)


def normalize_name(value: str) -> str:
    return " ".join(value.split()).casefold()


def parse_identity(value: str) -> tuple[str, str] | None:
    match = IDENTITY_RE.match(value)
    if match is None:
        return None
    return match.group(1).strip(), match.group(2).strip()


def has_matching_signoff(message: str, author_name: str, author_email: str) -> bool:
    expected_name = normalize_name(author_name)
    expected_email = author_email.strip().casefold()
    for name, email in SIGNOFF_RE.findall(message):
        if normalize_name(name) == expected_name and email.strip().casefold() == expected_email:
            return True
    return False


def run_git(*args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        check=True,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return result.stdout.strip()


def commits_in_range(base: str, head: str) -> list[str]:
    output = run_git("rev-list", "--reverse", "--no-merges", f"{base}..{head}")
    return output.splitlines() if output else []


def check_range(base: str, head: str, allowed_authors: set[str]) -> list[str]:
    failures: list[str] = []
    for sha in commits_in_range(base, head):
        raw = run_git("show", "-s", "--format=%H%x00%an%x00%ae%x00%B", sha)
        commit_sha, author_name, author_email, message = raw.split("\x00", 3)
        if author_name in allowed_authors:
            continue
        if not has_matching_signoff(message, author_name, author_email):
            failures.append(
                f"{commit_sha[:12]}: missing matching Signed-off-by for "
                f"{author_name} <{author_email}>"
            )
    return failures


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate DCO signoffs in a Git commit range.")
    parser.add_argument("--base", required=True)
    parser.add_argument("--head", required=True)
    parser.add_argument("--allow-author", action="append", default=[])
    args = parser.parse_args()

    try:
        commits = commits_in_range(args.base, args.head)
        failures = check_range(args.base, args.head, set(args.allow_author))
    except (subprocess.CalledProcessError, ValueError) as error:
        print(f"DCO check could not inspect the commit range: {error}", file=sys.stderr)
        return 2

    if failures:
        print("DCO check failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        print("Amend each commit with: git commit --amend -s", file=sys.stderr)
        return 1

    print(f"DCO check passed for {len(commits)} commit(s).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
