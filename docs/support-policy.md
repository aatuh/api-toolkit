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

## Release Baseline

The supported v4 root-module baseline is `v4.0.1`. `v4.0.0`,
`contrib/v4.0.0`, and `contrib/v4.0.1` are withdrawn and receive no support or
security-backport commitment. The [v4 release-identity incident](release-incident-v4-release-identity.md)
records the immutable evidence and the required paired contrib repair path.

Future supported root releases require a matching `contrib/vX.Y.Z` tag at the
same commit. The release workflow's `make release-tag-consistency-check` gate
rejects mismatched tags, module-major paths, release documentation, or an
older-major API baseline before publication evidence is produced.

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

## PostgreSQL Adapter Test Support

PostgreSQL `18.x` is the declared real-service test major for supported contrib
PostgreSQL adapters. The `postgres-contract` CI job uses a PostgreSQL 18 service
container and runs `make test-postgres`; it is the reusable harness baseline,
not proof that every adapter has already received a direct real-database
contract.

Locally, `GOWORK=off GOTOOLCHAIN=local make test-postgres` starts an isolated
loopback PostgreSQL 18 container when no test DSN is configured. A supplied
`API_TOOLKIT_TEST_POSTGRES_DSN` is accepted only with explicit opt-in and the
dedicated loopback/service-container test user, password, and `postgres`
administration database. It never reads `DATABASE_URL`, so the test target
cannot fall back to an application or production database.

## Generated Services

Generated services are app-owned. Their platform support depends on the
application's deployment target, provider choices, Docker image, and database or
cache adapters. The toolkit only supports the generated defaults that are
exercised by repository CI and release evidence.
