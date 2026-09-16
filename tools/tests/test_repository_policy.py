import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def read(relative_path: str) -> str:
    return (ROOT / relative_path).read_text(encoding="utf-8")


class GovernanceFilesTests(unittest.TestCase):
    def test_inherited_cla_is_not_active_on_the_community_branch(self):
        self.assertFalse((ROOT / "CLA.md").exists())
        self.assertFalse((ROOT / ".github/workflows/cla.yml").exists())

    def test_contribution_policy_uses_dco_without_assignment(self):
        contributing = read("CONTRIBUTING.md")
        self.assertIn("Developer Certificate of Origin 1.1", contributing)
        self.assertIn("git commit -s", contributing)
        self.assertIn("LGPL-3.0-or-later", contributing)
        self.assertNotIn("copyright assignment", contributing.casefold())

    def test_notice_preserves_upstream_attribution(self):
        notice = read("NOTICE")
        self.assertIn("Wei-Shaw/sub2api", notice)
        self.assertIn("Wesley Liddick", notice)
        self.assertIn("LGPL-3.0-or-later", notice)
        self.assertIn("respective contributors", notice)

    def test_readmes_do_not_deny_commercial_use(self):
        english = read("README.md")
        chinese = read("README_CN.md")
        self.assertNotIn("No Commercial Authorization", english)
        self.assertIn("Commercial use is permitted", english)
        self.assertIn("允许商业使用", chinese)


class ContributionEntryPointTests(unittest.TestCase):
    def issue_form_ids(self, relative_path: str) -> set[str]:
        ids = set()
        for line in read(relative_path).splitlines():
            stripped = line.strip()
            if stripped.startswith("id:"):
                ids.add(stripped.partition(":")[2].strip())
        return ids

    def test_bug_form_collects_reproduction_context(self):
        ids = self.issue_form_ids(".github/ISSUE_TEMPLATE/bug_report.yml")
        self.assertTrue(
            {"version", "branch", "deployment", "environment", "steps", "expected", "actual", "logs"}
            <= ids
        )

    def test_feature_form_collects_decision_context(self):
        ids = self.issue_form_ids(".github/ISSUE_TEMPLATE/feature_request.yml")
        self.assertTrue({"problem", "proposal", "alternatives", "branch_impact"} <= ids)

    def test_questions_are_routed_to_discussions(self):
        config = read(".github/ISSUE_TEMPLATE/config.yml")
        self.assertIn("blank_issues_enabled: false", config)
        self.assertIn("https://github.com/a408665321/sub2api/discussions", config)

    def test_pull_request_template_exists(self):
        template = read(".github/pull_request_template.md")
        self.assertIn("Signed-off-by", template)
        self.assertIn("LGPL-3.0-or-later", template)


if __name__ == "__main__":
    unittest.main()
