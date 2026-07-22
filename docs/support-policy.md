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

The required root-module CI platform is:

| OS | Architecture | Status | Evidence |
| --- | --- | --- | --- |
| Linux | amd64 | Supported | `platform-core` on `ubuntu-24.04` runs root build, tests, examples, and a generated-service build; the Linux quality workflow also runs race, vuln, lint, docs, API, and fuzz gates. |
| Linux | arm64 | Supported | `platform-core` on `ubuntu-24.04-arm` runs root build, tests, examples, and a generated-service build. |
| macOS | arm64 | Supported | `platform-core` on `macos-14` runs root build, tests, examples, and a generated-service build. |
| Windows | amd64 | Supported | `platform-core` on `windows-2022` runs root build, tests, examples, and a generated-service build. |

Root and generated-service compilation are required on every supported platform
with current tested Go (`1.26.x`). Race testing remains a Linux amd64 gate.
Contrib remains Linux-only for full unit and integration verification; the
portable generated service is the cross-platform CLI evidence.

The following pairs are not supported release targets and must not be claimed
in README, release notes, or package documentation without matching CI:

| OS | Architecture | Status | Notes |
| --- | --- | --- | --- |
| macOS | amd64 | Not supported | No required platform workflow. |
| Windows | arm64 | Not supported | No required platform workflow. |

Do not claim broad OS/architecture support beyond the supported table without
adding matching CI smoke checks and updating this policy.

## Generated Services

Generated services are app-owned. Their platform support depends on the
application's deployment target, provider choices, Docker image, and database or
cache adapters. The toolkit only supports the generated defaults that are
exercised by repository CI and release evidence.
