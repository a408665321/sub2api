import os
import subprocess
import tempfile
import unittest
from contextlib import contextmanager
from pathlib import Path

from tools.check_dco import check_range, has_matching_signoff


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


def commit(repo: Path, subject: str, name: str, email: str, signed: bool) -> str:
    message = subject
    if signed:
        message += f"\n\nSigned-off-by: {name} <{email}>"
    git(
        repo,
        "-c",
        f"user.name={name}",
        "-c",
        f"user.email={email}",
        "commit",
        "--allow-empty",
        "-m",
        message,
    )
    return git(repo, "rev-parse", "HEAD")


@contextmanager
def working_directory(path: Path):
    previous = Path.cwd()
    os.chdir(path)
    try:
        yield
    finally:
        os.chdir(previous)


class SignoffTests(unittest.TestCase):
    def test_accepts_matching_signoff(self):
        self.assertTrue(
            has_matching_signoff(
                "change\n\nSigned-off-by: Alice Example <alice@example.com>\n",
                "Alice Example",
                "alice@example.com",
            )
        )

    def test_normalizes_name_whitespace_and_email_case(self):
        self.assertTrue(
            has_matching_signoff(
                "Signed-off-by: Alice   Example <ALICE@example.com>",
                "Alice Example",
                "alice@example.com",
            )
        )

    def test_rejects_missing_or_mismatched_signoff(self):
        self.assertFalse(has_matching_signoff("change", "Alice", "alice@example.com"))
        self.assertFalse(
            has_matching_signoff(
                "Signed-off-by: Bob <bob@example.com>",
                "Alice",
                "alice@example.com",
            )
        )


class CommitRangeTests(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.repo = Path(self.temp_dir.name)
        git(self.repo, "init", "-q")
        self.base = commit(
            self.repo,
            "base",
            "Base Author",
            "base@example.com",
            signed=True,
        )

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_reports_unsigned_commit_by_short_sha_and_author(self):
        invalid_sha = commit(
            self.repo,
            "unsigned",
            "Alice Example",
            "alice@example.com",
            signed=False,
        )
        commit(
            self.repo,
            "signed",
            "Bob Example",
            "bob@example.com",
            signed=True,
        )
        head = git(self.repo, "rev-parse", "HEAD")

        with working_directory(self.repo):
            failures = check_range(self.base, head, set())

        self.assertEqual(1, len(failures))
        self.assertIn(invalid_sha[:12], failures[0])
        self.assertIn("Alice Example <alice@example.com>", failures[0])

    def test_allows_explicitly_allowlisted_bot(self):
        commit(
            self.repo,
            "automated update",
            "dependabot[bot]",
            "49699333+dependabot[bot]@users.noreply.github.com",
            signed=False,
        )
        head = git(self.repo, "rev-parse", "HEAD")

        with working_directory(self.repo):
            without_allowlist = check_range(self.base, head, set())
            with_allowlist = check_range(self.base, head, {"dependabot[bot]"})

        self.assertEqual(1, len(without_allowlist))
        self.assertEqual([], with_allowlist)


if __name__ == "__main__":
    unittest.main()
