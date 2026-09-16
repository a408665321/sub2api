#!/usr/bin/env python3
import argparse
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


TAG_RE = re.compile(r"^v(?P<base>\d+\.\d+\.\d+)-community\.(?P<revision>[1-9]\d*)$")
VERSION_FILE = Path("backend/cmd/server/VERSION")


@dataclass(frozen=True)
class ReleaseRef:
    tag: str
    commit: str


def git(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def resolve_commit(ref: str, description: str) -> str:
    result = git("rev-parse", "--verify", ref)
    if result.returncode != 0:
        raise ValueError(f"{description} does not exist: {ref}")
    return result.stdout.strip()


def validate_release(tag: str, branch: str) -> ReleaseRef:
    match = TAG_RE.fullmatch(tag)
    if match is None:
        raise ValueError(
            "invalid community release tag; expected v<version>-community.<positive-revision>"
        )

    tag_commit = resolve_commit(f"refs/tags/{tag}^{{commit}}", "tag")
    branch_commit = resolve_commit(f"{branch}^{{commit}}", "branch")
    reachable = git("merge-base", "--is-ancestor", tag_commit, branch_commit)
    if reachable.returncode != 0:
        raise ValueError(f"tag {tag} is not reachable from branch {branch}")

    try:
        version = VERSION_FILE.read_text(encoding="utf-8").strip()
    except OSError as error:
        raise ValueError(f"cannot read {VERSION_FILE}: {error}") from error
    if match.group("base") != version:
        raise ValueError(
            f"tag base version {match.group('base')} does not match VERSION {version}"
        )

    return ReleaseRef(tag=tag, commit=tag_commit)


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate a Sub2API community release tag")
    parser.add_argument("--tag", required=True)
    parser.add_argument("--branch", required=True)
    args = parser.parse_args()

    try:
        release = validate_release(args.tag, args.branch)
    except ValueError as error:
        print(f"release validation failed: {error}", file=sys.stderr)
        return 1

    print(f"Validated community release {release.tag} at {release.commit}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
