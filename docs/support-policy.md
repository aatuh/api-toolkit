# Support Policy

Audience: adopters deciding whether the repository supports their Go toolchain
and runtime platform.

## Go Version

Root and contrib target Go `1.25.x`.

The repository intentionally uses one supported Go line right now:

- `go.mod` and `contrib/go.mod` declare `go 1.25.0`.
- GitHub Actions install `1.25.x`.
- Local release and audit commands should run with `GOTOOLCHAIN=local` so a
  mismatched local toolchain fails visibly instead of silently downloading a
  different toolchain.

Supporting the two most recent Go releases would require adding an explicit CI
matrix, release evidence, and compatibility notes. Until that exists, users
should treat `1.25.x` as the supported line.

## Release Baseline

The supported v4 root-module baseline is `v4.0.1`. `v4.0.0`,
`contrib/v4.0.0`, and `contrib/v4.0.1` are withdrawn and receive no support or
security-backport commitment. The [v4 release-identity incident](release-incident-v4-release-identity.md)
records the immutable evidence and the required paired contrib repair path.

## Platform

The required CI platform is:

| OS | Architecture | Status | Evidence |
| --- | --- | --- | --- |
| Linux | amd64 | Supported | GitHub-hosted Ubuntu CI runs unit, race, vuln, lint, docs, API, and fuzz smoke gates. |

Other platforms are portability goals, not supported release gates:

| OS | Architecture | Status | Notes |
| --- | --- | --- | --- |
| macOS | amd64/arm64 | Best effort | Expected for pure Go packages, but not a required release gate. |
| Windows | amd64/arm64 | Best effort | Expected for pure Go packages that avoid Unix assumptions, but not a required release gate. |
| Linux | arm64 | Best effort | Expected for pure Go packages, but generated deployment assets are not release-gated on arm64. |

Do not claim broad OS/architecture support in README, release notes, or package
docs until CI includes matching smoke checks.

## Generated Services

Generated services are app-owned. Their platform support depends on the
application's deployment target, provider choices, Docker image, and database or
cache adapters. The toolkit only supports the generated defaults that are
exercised by repository CI and release evidence.
