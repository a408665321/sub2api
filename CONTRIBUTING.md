# Contributing to the Community Distribution

Thank you for contributing to the unofficial community-maintained Sub2API
distribution. Reproducible defects belong in
[Issues](https://github.com/a408665321/sub2api/issues); setup and support
questions belong in
[Discussions](https://github.com/a408665321/sub2api/discussions).

## Branch and pull request workflow

Open pull requests against `custom/main`. Keep each change focused, explain
behavior and migration impact, and run the relevant backend and frontend tests.
Never include credentials, access tokens, private logs, or personal data.

## License and certification

Contributions are accepted under the same LGPL-3.0-or-later terms that apply to
the project (inbound equals outbound). You retain copyright in your work; this
project does not require a copyright transfer, a Contributor License Agreement,
or additional relicensing rights.

Every commit must certify the Developer Certificate of Origin 1.1. Add the
certification with Git's signoff option:

```bash
git commit -s
```

The resulting commit message must contain:

```text
Signed-off-by: Your Name <your.email@example.com>
```

By adding that line, you certify the
[Developer Certificate of Origin 1.1](https://developercertificate.org/): you
have the right to submit the contribution under the project's license and
understand that the contribution and signoff are public, permanent records.

## Tests and review

Before requesting review, run the checks relevant to your change. The pull
request must pass DCO validation, shell checks, backend tests, frontend tests,
and Go lint. Maintainers may request focused regression tests or documentation.

## Security reports

Do not disclose an exploitable vulnerability in a public Issue. Use GitHub's
private vulnerability reporting when it is available. If that channel is not
available, open a Discussion without exploit details and ask a maintainer for a
private reporting route.
