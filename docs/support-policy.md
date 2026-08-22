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

The required root-module CI platforms are:

| OS | Architecture | Status | Evidence |
| --- | --- | --- | --- |
| Linux | amd64 | Supported | `platform-core (linux-amd64)` on `ubuntu-24.04` runs root verification, build, tests, examples, and a generated-service build; the Linux quality workflow also runs race, vulnerability, lint, docs, API, and fuzz gates. |
| Linux | arm64 | Supported | `platform-core (linux-arm64)` on `ubuntu-24.04-arm` runs root verification, build, tests, examples, and a generated-service build. |
| macOS | arm64 | Supported | `platform-core (macos-arm64)` on `macos-15` runs root verification, build, tests, examples, and a generated-service build. |
| Windows | amd64 | Supported | `platform-core (windows-amd64)` on `windows-2022` runs root verification, build, tests, examples, and a generated-service build. |

Root and generated-service compilation are required on every supported platform
with current tested Go (`1.26.x`). Race testing remains a Linux amd64 gate.
Contrib remains Linux-only for full unit and integration verification; the
portable generated service is the cross-platform CLI evidence.

Git normalizes repository-owned text files to LF through `.gitattributes` so
byte-sensitive fixtures and policy manifests have identical content on Linux,
macOS, and Windows checkouts. Generated applications remain free to choose
their own line-ending policy after generation.

Generator manifests use canonical slash-form relative paths. The CLI rejects
absolute, traversal, backslash, volume/alternate-stream, and NUL-bearing names
before mapping a validated manifest path to the host separator and writing it
through a rooted filesystem handle.

The following pairs are portability goals, not supported release targets:

| OS | Architecture | Status | Notes |
| --- | --- | --- | --- |
| macOS | amd64 | Not supported | No required platform workflow. |
| Windows | arm64 | Not supported | No required platform workflow. |

Do not claim broader OS/architecture support in README, release notes, or
package docs without adding matching required CI checks and updating this
policy.

## PostgreSQL Adapter Test Support

PostgreSQL `18.x` is the declared real-service test major for supported contrib
PostgreSQL adapters. The `postgres-contract` CI job uses a PostgreSQL 18 service
container and runs `make test-postgres` on every pull request. That target
directly covers the supported PostgreSQL adapters and generated reference-service
persistence paths against the real service.

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
