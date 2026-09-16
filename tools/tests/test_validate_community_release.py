import os
import subprocess
import tempfile
import unittest
from contextlib import contextmanager
from pathlib import Path

from tools.validate_community_release import validate_release


def git(repo: Path, *args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=repo,
        check=True,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return result.stdout.strip()


@contextmanager
def repository(version: str = "0.2.5"):
    with tempfile.TemporaryDirectory() as directory:
        repo = Path(directory)
        git(repo, "init", "-q", "-b", "custom/main")
        git(repo, "config", "user.name", "Release Test")
        git(repo, "config", "user.email", "release@example.com")
        version_file = repo / "backend/cmd/server/VERSION"
        version_file.parent.mkdir(parents=True)
        version_file.write_text(version + "\n", encoding="utf-8")
        git(repo, "add", ".")
        git(repo, "commit", "-q", "-m", "initial")
        old_cwd = Path.cwd()
        try:
            os.chdir(repo)
            yield repo
        finally:
            os.chdir(old_cwd)


class CommunityReleaseValidationTests(unittest.TestCase):
    def test_accepts_reachable_tag_matching_version(self):
        with repository() as repo:
            git(repo, "tag", "v0.2.5-community.1")
            result = validate_release("v0.2.5-community.1", "custom/main")
            self.assertEqual("v0.2.5-community.1", result.tag)
            self.assertEqual(git(repo, "rev-parse", "HEAD"), result.commit)

    def test_rejects_upstream_tag_and_zero_revision(self):
        with repository() as repo:
            git(repo, "tag", "v0.2.5")
            git(repo, "tag", "v0.2.5-community.0")
            for tag in ("v0.2.5", "v0.2.5-community.0"):
                with self.subTest(tag=tag):
                    with self.assertRaisesRegex(ValueError, "invalid community release tag"):
                        validate_release(tag, "custom/main")

    def test_rejects_missing_tag(self):
        with repository():
            with self.assertRaisesRegex(ValueError, "tag does not exist"):
                validate_release("v0.2.5-community.1", "custom/main")

    def test_rejects_tag_not_reachable_from_maintained_branch(self):
        with repository() as repo:
            git(repo, "tag", "v0.2.5-community.1")
            tagged = git(repo, "rev-parse", "HEAD")
            tree = git(repo, "rev-parse", "HEAD^{tree}")
            unrelated = subprocess.run(
                ["git", "commit-tree", tree],
                cwd=repo,
                check=True,
                input="unrelated\n",
                text=True,
                stdout=subprocess.PIPE,
            ).stdout.strip()
            git(repo, "branch", "unrelated", unrelated)
            self.assertNotEqual(tagged, unrelated)
            with self.assertRaisesRegex(ValueError, "not reachable from branch"):
                validate_release("v0.2.5-community.1", "unrelated")

    def test_rejects_tag_base_version_mismatch(self):
        with repository(version="0.2.6") as repo:
            git(repo, "tag", "v0.2.5-community.1")
            with self.assertRaisesRegex(ValueError, "does not match VERSION 0.2.6"):
                validate_release("v0.2.5-community.1", "custom/main")


if __name__ == "__main__":
    unittest.main()
