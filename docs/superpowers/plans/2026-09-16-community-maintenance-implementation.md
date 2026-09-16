# Community Maintenance Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn `a408665321/sub2api:custom/main` into a legally clear, CI-gated, community-maintained distribution that publishes traceable GitHub Releases and GHCR images.

**Architecture:** Keep `main` as the protected upstream mirror and make `custom/main` the default maintained branch. Replace the upstream-specific CLA with a repository-owned DCO validator, retain LGPL-3.0-or-later with accurate commercial-use language, and isolate community release behavior in dedicated workflow and GoReleaser files. GitHub repository settings are changed only after the new checks pass on the implementation pull request.

**Tech Stack:** GitHub Actions, Python 3 standard library, `unittest`, Go 1.27, pnpm 9, Vue/TypeScript/Vitest, GoReleaser v2, Docker Buildx, GitHub API via `gh`.

**Spec:** `docs/superpowers/specs/2026-09-16-community-maintenance-design.md`

## Global Constraints

- Retain `LICENSE` as LGPL-3.0-or-later and preserve upstream attribution.
- Do not claim ownership of upstream code or copy upstream CLA signatures.
- Branch names, workflow names, commits, and generated content must not add `codex` branding.
- Community release tags use `v<upstream-version>-community.<revision>`; first planned tag is `v0.2.5-community.1`.
- Publish only GitHub Releases and `ghcr.io/a408665321/sub2api`; do not require Docker Hub, Telegram, or production credentials.
- Pull request workflows use read-only tokens; release write permissions exist only in the release job.
- Release workflows never run on pull request events and never write commits back to protected branches.
- All implementation commits use `Signed-off-by: a408665321 <408665321@qq.com>`.

---

### Task 1: Repository-Owned DCO Validation

**Files:**
- Create: `tools/check_dco.py`
- Create: `tools/tests/test_check_dco.py`
- Create: `.github/workflows/dco.yml`

**Interfaces:**
- Produces: `parse_identity(value: str) -> tuple[str, str] | None`.
- Produces: `has_matching_signoff(message: str, author_name: str, author_email: str) -> bool`.
- Produces: `check_range(base: str, head: str, allowed_authors: set[str]) -> list[str]`, returning human-readable failures.
- CLI: `python tools/check_dco.py --base <sha> --head <sha> [--allow-author <name>]` exits `0` on success and `1` with one line per invalid commit.
- Workflow check context: `DCO / signoff`.

- [ ] **Step 1: Write pure-function and temporary-repository tests**

```python
class SignoffTests(unittest.TestCase):
    def test_accepts_matching_signoff(self):
        self.assertTrue(has_matching_signoff(
            "change\n\nSigned-off-by: Alice Example <alice@example.com>\n",
            "Alice Example",
            "alice@example.com",
        ))

    def test_rejects_missing_or_mismatched_signoff(self):
        self.assertFalse(has_matching_signoff("change", "Alice", "alice@example.com"))
        self.assertFalse(has_matching_signoff(
            "Signed-off-by: Bob <bob@example.com>", "Alice", "alice@example.com"
        ))
```

Add temporary Git repository cases proving that `check_range` checks every non-merge commit, reports the short SHA, and permits `dependabot[bot]` only when explicitly allowlisted.

- [ ] **Step 2: Run the tests and verify the missing module failure**

Run: `python3 -m unittest -v tools.tests.test_check_dco`

Expected: FAIL because `tools.check_dco` does not exist.

- [ ] **Step 3: Implement the minimal validator**

Use `git rev-list --reverse --no-merges <base>..<head>` and, for each SHA,
`git show -s --format=%H%x00%an%x00%ae%x00%B <sha>`. Match trailers with a
case-insensitive multiline expression:

```python
SIGNOFF_RE = re.compile(r"^Signed-off-by:\s*(.+?)\s*<([^<>]+)>\s*$", re.I | re.M)
```

Names compare after collapsing whitespace; emails compare case-insensitively.
Print `DCO check passed for N commit(s).` on success.

- [ ] **Step 4: Run validator unit tests and repository-range smoke tests**

Run:

```bash
python3 -m unittest -v tools.tests.test_check_dco
python3 tools/check_dco.py --base mine/custom/main --head HEAD
```

Expected: all unit tests PASS and the current signed-off design commits pass.

- [ ] **Step 5: Add the DCO workflow**

Create a workflow named `DCO` triggered only by pull requests targeting
`custom/main`. Use `actions/checkout@v6` with `fetch-depth: 0`, check out the PR
head SHA, and run:

```yaml
python3 tools/check_dco.py \
  --base "${{ github.event.pull_request.base.sha }}" \
  --head "${{ github.event.pull_request.head.sha }}" \
  --allow-author 'dependabot[bot]' \
  --allow-author 'renovate[bot]'
```

Set `permissions: contents: read`, job name `signoff`, timeout 5 minutes, and
concurrency keyed by workflow and PR number.

- [ ] **Step 6: Validate YAML and commit**

Run:

```bash
python3 -m unittest -v tools.tests.test_check_dco
git diff --check
```

Commit: `feat(ci): enforce DCO signoffs`

---

### Task 2: License, Attribution, and Contributor Experience

**Files:**
- Delete: `CLA.md`
- Delete: `.github/workflows/cla.yml`
- Create: `NOTICE`
- Create: `CONTRIBUTING.md`
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Create: `.github/ISSUE_TEMPLATE/feature_request.yml`
- Create: `.github/ISSUE_TEMPLATE/config.yml`
- Create: `.github/pull_request_template.md`
- Modify: `README.md`
- Modify: `README_CN.md`

**Interfaces:**
- Contributor certification: DCO 1.1 through `git commit -s`.
- License statement: commercial use is permitted under LGPL-3.0-or-later, subject to its conditions.
- Support routing: reproducible defects use Issues; configuration/help requests use Discussions.

- [ ] **Step 1: Add a failing repository policy test**

Create `tools/tests/test_repository_policy.py` with assertions that:

```python
self.assertFalse((ROOT / "CLA.md").exists())
self.assertFalse((ROOT / ".github/workflows/cla.yml").exists())
self.assertIn("Commercial use is permitted", read("README.md"))
self.assertIn("允许商业使用", read("README_CN.md"))
self.assertIn("Developer Certificate of Origin 1.1", read("CONTRIBUTING.md"))
self.assertIn("Wei-Shaw/sub2api", read("NOTICE"))
```

Also assert both Issue forms contain fields for version, branch, deployment
method, and sanitized diagnostics.

- [ ] **Step 2: Run the policy test and confirm it fails**

Run: `python3 -m unittest -v tools.tests.test_repository_policy`

Expected: FAIL on the inherited CLA and missing community files.

- [ ] **Step 3: Replace inherited CLA governance with LGPL + DCO documentation**

Remove the inherited CLA files. Add `NOTICE` preserving upstream project URL,
Wesley Liddick's existing copyright notice, LGPL-3.0-or-later, and the statement
that modifications are copyrighted by their respective contributors.

Add `CONTRIBUTING.md` containing:

```text
git commit -s
Signed-off-by: Your Name <your.email@example.com>
```

Explain inbound-equals-outbound LGPL licensing, no additional copyright
assignment, branch targeting, test expectations, security reporting, and the
permanence of public contribution records.

- [ ] **Step 4: Correct README licensing and fork identity**

Replace the English non-commercial statement with the approved commercial-use
language. Add matching Chinese text. Near the top of each README, identify
`custom/main` as an unofficial community-maintained distribution and link the
maintained releases, GHCR image, Issues, Discussions, and contribution guide.
Do not remove upstream authorship or history.

- [ ] **Step 5: Add Issue forms and pull request template**

Bug form required fields: version/tag, branch, deployment method, environment,
steps, expected behavior, actual behavior, and sanitized logs. Feature form
required fields: problem, proposal, alternatives, branch impact. Disable blank
Issues and route questions to `https://github.com/a408665321/sub2api/discussions`.

PR template checkboxes cover testing, DCO signoff, no secrets, LGPL-compatible
contribution rights, behavior/migration impact, and upstream applicability.

- [ ] **Step 6: Run policy tests and commit**

Run:

```bash
python3 -m unittest -v tools.tests.test_repository_policy
git diff --check
```

Commit: `docs: establish community contribution policy`

---

### Task 3: Community CI and Security Checks

**Files:**
- Modify: `.github/workflows/backend-ci.yml`
- Modify: `.github/workflows/security-scan.yml`
- Modify: `tools/tests/test_repository_policy.py`

**Interfaces:**
- Required check contexts: `CI / shell`, `CI / backend`, `CI / frontend`, `CI / golangci-lint`.
- Security checks remain non-required initially: `Security Scan / backend-security`, `Security Scan / frontend-security`.

- [ ] **Step 1: Extend policy tests for workflow triggers and permissions**

Load workflow files as text and assert they contain `custom/main`,
`workflow_dispatch`, `permissions`, `contents: read`, `concurrency`, and
`timeout-minutes`. Assert CI job identifiers are exactly `shell`, `backend`,
`frontend`, and `golangci-lint`.

- [ ] **Step 2: Run policy tests and confirm workflow assertions fail**

Run: `python3 -m unittest -v tools.tests.test_repository_policy`

Expected: FAIL because inherited workflows use unrestricted push/PR triggers,
the backend job is named `test`, and no concurrency or timeouts exist.

- [ ] **Step 3: Scope and harden CI**

Trigger on pushes and pull requests to `custom/main`, plus manual dispatch. Add:

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}
  cancel-in-progress: true
```

Rename `test` to `backend`. Retain existing commands, Go 1.27 verification,
pnpm 9 frozen installation, and pinned golangci-lint v2.13. Add timeouts of 10
minutes for shell, 30 for backend/frontend, and 35 for lint.

- [ ] **Step 4: Scope and harden security scanning**

Trigger on `custom/main` push/PR, weekly schedule, and manual dispatch. Add
concurrency and 20-minute timeouts. Retain `govulncheck` and
`tools/check_pnpm_audit_exceptions.py`. Keep permissions read-only.

- [ ] **Step 5: Validate workflows and run policy tests**

Run:

```bash
python3 -m unittest -v tools.tests.test_repository_policy
docker run --rm -v "$PWD:/repo" -w /repo rhysd/actionlint:latest
git diff --check
```

Expected: policy tests PASS and actionlint reports no workflow errors.

- [ ] **Step 6: Commit**

Commit: `ci: gate community branch changes`

---

### Task 4: Traceable Community Releases

**Files:**
- Create: `tools/validate_community_release.py`
- Create: `tools/tests/test_validate_community_release.py`
- Create: `.goreleaser.community.yaml`
- Replace: `.github/workflows/release.yml`
- Modify: `tools/tests/test_repository_policy.py`

**Interfaces:**
- CLI: `python3 tools/validate_community_release.py --tag <tag> --branch <ref>`.
- Valid tag expression: `^v(?P<base>\d+\.\d+\.\d+)-community\.(?P<revision>[1-9]\d*)$`.
- Release outputs: GitHub archives/checksum and `ghcr.io/a408665321/sub2api:<version>`, architecture tags, and `community-latest`.

- [ ] **Step 1: Write release validation tests**

Tests must reject upstream tags, zero revisions, missing tags, tags not reachable
from the maintained branch, and tags whose base version differs from
`backend/cmd/server/VERSION`. They must accept `v0.2.5-community.1` when the tag
points at an ancestor of the supplied branch.

- [ ] **Step 2: Run tests and confirm the missing module failure**

Run: `python3 -m unittest -v tools.tests.test_validate_community_release`

Expected: FAIL because the validator does not exist.

- [ ] **Step 3: Implement release validation**

Use `git rev-parse --verify refs/tags/<tag>^{commit}` and
`git merge-base --is-ancestor <tag-sha> <branch>`. Compare the regex `base`
group to the trimmed VERSION file. Print the resolved tag and SHA on success;
return a distinct error message for format, missing ref, reachability, and
version mismatch failures.

- [ ] **Step 4: Run validator tests**

Run: `python3 -m unittest -v tools.tests.test_validate_community_release`

Expected: all tests PASS.

- [ ] **Step 5: Add community GoReleaser configuration**

Derive from `.goreleaser.yaml` but remove Docker Hub and Telegram behavior.
Build Linux, macOS, and Windows archives for supported amd64/arm64 combinations,
SHA-256 checksums, GHCR amd64/arm64 images, a multi-arch version manifest, and
`community-latest`. Include `LICENSE*`, `NOTICE`, README files, and `deploy/*`
in archives. Set OCI labels for source, revision, version, and
`org.opencontainers.image.licenses=LGPL-3.0-or-later`.

- [ ] **Step 6: Replace release workflow**

Trigger only `v*-community.*` tag pushes and manual dispatch with an existing
tag input. Preflight checks out full history, fetches `custom/main` and tags,
then runs the validator before any registry login. Build the frontend and
version file as artifacts. The release job checks out the tag, downloads those
artifacts, logs into GHCR using `GITHUB_TOKEN`, and runs:

```text
goreleaser release --clean --config=.goreleaser.community.yaml
```

Set repository-level default permissions to read and job-level release
permissions to `contents: write` and `packages: write`. Do not update VERSION in
Git or push commits.

- [ ] **Step 7: Extend policy tests and validate release configuration**

Assert the release workflow contains no `DOCKERHUB`, `TELEGRAM`, `pull_request`,
or branch-push trigger; assert it references `.goreleaser.community.yaml`,
`custom/main`, and the validator. Assert the GoReleaser config contains only
the `ghcr.io/a408665321/sub2api` repository and `community-latest` floating tag.

Run:

```bash
python3 -m unittest -v tools.tests.test_repository_policy tools.tests.test_validate_community_release
docker run --rm -v "$PWD:/repo" -w /repo rhysd/actionlint:latest
docker run --rm -v "$PWD:/repo" -w /repo goreleaser/goreleaser:latest check --config=.goreleaser.community.yaml
git diff --check
```

- [ ] **Step 8: Commit**

Commit: `feat(release): publish community builds to GHCR`

---

### Task 5: Pull Request Validation and Repository Settings

**Files:**
- Modify only if validation finds defects in Tasks 1-4.
- GitHub settings changed through API after merge.

**Interfaces:**
- Default branch: `custom/main`.
- Required contexts: `DCO / signoff`, `CI / shell`, `CI / backend`, `CI / frontend`, `CI / golangci-lint`.
- Release package: `ghcr.io/a408665321/sub2api`.

- [ ] **Step 1: Run the complete repository-owned validation suite**

Run:

```bash
python3 -m unittest discover -s tools/tests -p 'test_*.py' -v
docker run --rm -v "$PWD:/repo" -w /repo rhysd/actionlint:latest
docker run --rm -v "$PWD:/repo" -w /repo goreleaser/goreleaser:latest check --config=.goreleaser.community.yaml
git diff --check
```

- [ ] **Step 2: Push signed-off commits and observe Draft PR checks**

Push `feature/community-governance`. Confirm DCO and all four CI contexts appear.
Use job logs to fix concrete repository-specific failures; do not weaken or
skip the required commands. Keep the PR draft during fix rounds.

- [ ] **Step 3: Mark PR ready and merge through branch protection**

Confirm the diff contains no credentials, no Docker Hub/Telegram publishing,
and no upstream copyright claim. Mark PR ready, wait for green checks, and
squash-merge or merge according to repository policy while preserving a DCO
signoff in the resulting commit.

- [ ] **Step 4: Change repository settings**

Use GitHub API to:

- set `default_branch` to `custom/main`;
- enable `delete_branch_on_merge`;
- retain Issues and Discussions;
- enable vulnerability alerts and automated security fixes;
- retain Actions default workflow permission `read`.

- [ ] **Step 5: Require successful checks on `custom/main`**

Update branch protection with strict status checks for the five exact contexts,
PR requirement with zero approvals, administrator enforcement, conversation
resolution, and force-push/deletion disabled. Preserve the existing `main`
mirror protection without adding PR or CI requirements.

- [ ] **Step 6: Verify remote state**

Query the GitHub API and confirm default branch, repository features, security
settings, workflow activation, required contexts, and branch protection. Confirm
no release or package was created during setup.

- [ ] **Step 7: Prepare but do not publish the first release**

Report the exact reviewed command for a later release:

```bash
git tag -s v0.2.5-community.1 <verified-custom-main-sha>
git push mine v0.2.5-community.1
```

Tag creation remains a separate explicit user decision because it publishes
public binaries and container images.
