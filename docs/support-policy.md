# Support Policy

Audience: adopters deciding whether the repository supports their Go toolchain
and runtime platform.

## Go Version

Minimum supported Go is `1.25.x`. Current tested Go is `1.26.x`. Root and
contrib are required to pass CI on both lines.

The module directives remain at the minimum supported version:

- `go.mod` and `contrib/go.mod` declare `go 1.25.0`.
- GitHub Actions run root and contrib module verification, builds, unit tests,
  examples, and API compatibility checks on `1.25.x` and `1.26.x`.
- Release preflight repeats root and contrib module verification, builds, unit
  tests, and examples on both supported Go lines before publication evidence is
  generated.
- Local release and audit commands should run with `GOTOOLCHAIN=local` so a
  mismatched local toolchain fails visibly instead of silently downloading a
  different toolchain.

The release-evidence summary records the required matrix and whether its
release-workflow dependency passed. Do not treat an untested future Go release
as supported merely because the module directives still parse on it.

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
