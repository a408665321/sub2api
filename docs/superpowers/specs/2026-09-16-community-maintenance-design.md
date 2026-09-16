# Community Maintenance, CI, and Release Design

Date: 2026-09-16
Repository: `a408665321/sub2api`
Maintained branch: `custom/main`

## Context

The repository is a public fork of `Wei-Shaw/sub2api`. The upstream `main`
branch remains the source mirror, while `custom/main` carries the maintained
community distribution. The fork currently inherits upstream CI and release
workflows, but its CLA workflow is hard-coded to the upstream repository, its
default branch still points at the mirror, and releases are not constrained to
the maintained branch.

The repository is licensed under LGPL-3.0-or-later. The English README also
contains a statement denying commercial authorization, which conflicts with
the permissions granted by the open-source license. The inherited CLA names an
upstream individual as Project Owner and must not be repurposed by only changing
repository URLs.

## Goals

- Make `custom/main` the clear community-maintained product branch.
- Preserve upstream copyright and LGPL-3.0-or-later licensing.
- State accurately that commercial use is allowed subject to LGPL and third-party obligations.
- Replace the inherited CLA with Developer Certificate of Origin 1.1 checks.
- Require repeatable CI before changes can merge into `custom/main`.
- Publish traceable GitHub Releases and multi-architecture GHCR images.
- Keep release and CI permissions minimal and avoid long-lived publishing secrets.
- Give contributors structured Issue, Discussion, and pull request entry points.

## Non-Goals

- Relicensing upstream code as MIT, Apache, or proprietary software.
- Claiming ownership of upstream code or contributor copyrights.
- Granting rights to third-party APIs, subscriptions, accounts, trademarks, or payment services.
- Publishing to Docker Hub before separate credentials and ownership are configured.
- Automatically deploying a release to a production Sub2API instance.
- Automatically merging upstream changes into `custom/main` without review.

## Legal and Contribution Model

The fork will retain the upstream `LICENSE` unchanged. README files will say
that commercial use is permitted under LGPL-3.0-or-later and distinguish the
software license from third-party service terms and regulated operations. The
existing non-commercial statement will be removed.

A `NOTICE` file will preserve the upstream attribution and state that fork
modifications remain owned by their respective contributors. The fork will be
described as an unofficial community-maintained distribution, without implying
endorsement by the upstream project.

The inherited `CLA.md` and CLA workflow will be removed from `custom/main`.
Contributions will use Developer Certificate of Origin 1.1 instead. Every commit
in a pull request targeting `custom/main` must contain a matching
`Signed-off-by: Name <email>` trailer. `CONTRIBUTING.md` will document
`git commit -s`, the LGPL inbound-equals-outbound rule, and the permanence of
public contribution records.

The DCO check grants no additional ownership or relicensing rights to the
maintainer. It only records the contributor's certification that they have the
right to submit the contribution under the repository's license.

## Branch and Repository Model

- `main`: protected, fast-forward mirror of `Wei-Shaw/sub2api:main`.
- `custom/main`: protected default branch for community development and releases.
- `feature/*` and `fix/*`: short-lived branches merged into `custom/main` through pull requests.
- Upstream contributions: created from a clean upstream baseline and proposed directly to `Wei-Shaw/sub2api`.

The repository default branch will change to `custom/main`. Automatic deletion
of merged head branches will be enabled. Issues and Discussions remain enabled.

`custom/main` protection will continue to require pull requests, apply to
administrators, block force pushes and deletion, and require resolved review
conversations. It will additionally require the stable CI and DCO checks defined
below. The approval count stays at zero until another regular maintainer exists,
so a sole maintainer is not permanently blocked.

`main` remains protected against force pushes and deletion but permits normal
fast-forward synchronization without a pull request.

## Continuous Integration

CI will run on pull requests targeting `custom/main`, pushes to `custom/main`,
and manual dispatch. Workflows will use read-only repository permissions unless
a job has a documented reason for more access.

Required checks:

1. `DCO / signoff`: validate every pull request commit's sign-off trailer.
2. `CI / shell`: validate deployment shell scripts and their existing tests.
3. `CI / backend`: run backend unit and integration tests with the Go version from `backend/go.mod`.
4. `CI / frontend`: install with the locked pnpm version, run lint, type checking, and the repository's critical Vitest set.
5. `CI / golangci-lint`: run the pinned golangci-lint release.

Jobs will use concurrency cancellation for superseded commits and explicit
timeouts. Action versions will remain pinned to stable major releases already
used upstream; third-party actions will be minimized. DCO validation will be a
repository-owned script with focused tests instead of a privileged external app.

The security workflow will run on pull requests targeting `custom/main`, pushes
to `custom/main`, a weekly schedule, and manual dispatch. It will retain
`govulncheck` and the audited pnpm exception mechanism. Dependabot security
updates will be enabled after CI is required, so generated pull requests are
subject to the same checks.

## Release Design

Community releases use SemVer-compatible tags that do not collide with upstream:

```text
v<upstream-version>-community.<revision>
```

Example: `v0.2.5-community.1`.

The release workflow will accept tag pushes matching `v*-community.*` and manual
re-runs for an existing tag. Before publishing it must verify that:

- the tag is valid and resolves to a commit;
- the tagged commit is reachable from `custom/main`;
- the working release version matches the tag;
- CI has already passed for the tagged commit.

The workflow will not create tags or write version commits back to a protected
branch. It will update the embedded `VERSION` file only in the isolated build
workspace.

Publishing targets:

- GitHub Release in `a408665321/sub2api`;
- Linux, macOS, and Windows archives with SHA-256 checksums;
- `ghcr.io/a408665321/sub2api` images for `linux/amd64` and `linux/arm64`;
- a multi-architecture version tag and `community-latest` tag.

The workflow uses only the repository-scoped `GITHUB_TOKEN` with
`contents: write` and `packages: write` in the release job. No Docker Hub,
Telegram, or private deployment credentials are required. Release artifacts and
container labels will include the tag, commit SHA, source repository, license,
and source URL. Release notes will state the LGPL license and link to the exact
tagged source.

A dedicated community GoReleaser configuration will isolate fork-specific image
names and tags from upstream release behavior. The inherited upstream release
workflow will be replaced on `custom/main`, not edited on the mirror `main`.

## Contributor Experience

Issue forms will cover bug reports and feature requests and require the branch,
version or image tag, deployment method, and relevant logs with secrets removed.
A configuration-only support question will be directed to Discussions. The pull
request template will require a change summary, testing evidence, license/DCO
confirmation, and disclosure of behavior or migration impact.

README files will put the maintained branch, GHCR image, release channel,
commercial-use statement, contribution guide, Issues, and Discussions near the
top without removing upstream history.

## Failure Handling and Security

- Pull requests from forks receive read-only tokens and no publishing secrets.
- Release jobs never run for pull request events.
- A tag outside `custom/main` fails before package login or publication.
- Existing release tags are immutable; correction uses a new community revision.
- Failed multi-architecture publication does not update `community-latest`.
- DCO failures explain the exact commit and the `git commit --amend -s` remedy.
- Audit exceptions remain explicit, reviewable, and time-bounded where possible.
- Logs and Issue templates warn contributors not to submit credentials or account data.

## Rollout

1. Add documentation, DCO validation, templates, community CI, and release configuration on a feature branch.
2. Validate workflow syntax and repository-owned scripts locally.
3. Open a pull request into `custom/main` and observe unrequired CI checks once.
4. Fix any fork-specific failures without weakening the checks.
5. Merge the pull request, change the default branch to `custom/main`, and enable required status checks.
6. Enable Dependabot security updates.
7. Create a signed-off release preparation pull request.
8. Tag the first release as `v0.2.5-community.1` only after all required checks pass.
9. Verify the GitHub Release, checksums, GHCR architectures, labels, and public pull instructions.

## Verification

- YAML parsing and GitHub workflow validation for every workflow.
- Unit tests for DCO commit-range and trailer validation.
- Existing targeted backend/frontend checks used by CI.
- A dry-run GoReleaser build without publication.
- GitHub API verification of default branch, repository settings, branch protection, and required checks.
- After the first tag, inspect release assets and both container architectures before documenting the release as available.

## Residual Legal Boundary

This design describes repository licensing and engineering controls, not legal
advice for operating an AI gateway business. Operators remain responsible for
third-party provider terms, account and subscription restrictions, privacy,
content safety, payments, tax, consumer protection, export controls, and local
licensing or filing requirements.
