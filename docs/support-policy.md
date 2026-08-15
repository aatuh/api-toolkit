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

## Provider Sandbox Evidence

Provider adapters use hermetic fixture tests by default. Their optional live
sandbox evidence is current only when the protected `nightly / provider-sandbox`
workflow has a `passed` sanitized artifact no more than **30 days** old. Missing
credentials, `not_requested`, skipped, failed, or stale records are not
success. In those cases retain the explicitly dispositioned
`hermetic-provider-fixture+manual-real-service` realism status in
`docs/supported-adapter-test-realism.tsv`; do not claim current provider
sandbox verification.

## Module Support And Maintenance

This policy covers current v4 surfaces and the planned v5 module boundaries in
[ADR 0002](adr/0002-v5-module-decomposition.md). The ADR is not a released
module list: until a v5 module is published, its support tier is a planning
decision only. `docs/package-classification.tsv` remains the source of truth
for the current package tier.

| Surface | Current or planned tier | Maintenance boundary |
| --- | --- | --- |
| Current v4 stable root packages | Stable | SemVer compatibility and release API checks apply to the released root module. |
| Current v4 compatibility-only packages | Compatibility-only | Kept for the documented v4 compatibility window; not a recommended new abstraction. |
| Current contrib supported adapters | Supported adapter | Require a named owner route, package/docs evidence, behavior contract, direct tests, and current release-drift review. They remain outside the stable-root promise. |
| Current contrib experimental, tooling, generated, example, and test-support packages | Non-stable | Best-effort maintenance only; no implied API compatibility or production guarantee. |
| Generated services and reference applications | Application-owned | Application teams own deployment, provider workflow, data, load, incident, and upgrade decisions. Repository examples are starters and evidence fixtures, not universal production guarantees. |
| Planned v5 core and optional modules | Proposed | Their module-specific owner, support tier, baseline, and migration path must be published before they can be described as supported. |

Routine maintenance is best effort and fits the currently recorded maintainer
capacity in [MAINTAINERS.md](../MAINTAINERS.md). There is no 24/7 support,
commercial response commitment, or promise to accept a feature, provider, or
module request. Security intake and severity targets are defined once in
[SECURITY.md](../SECURITY.md); that policy also defines the limited exceptional
previous-minor backport decision.

## Lifecycle And Deprecation

A stable or compatibility-only public API is deprecated only with a replacement,
reason, earliest removal major, and migration guidance in
[deprecations.md](deprecations.md). A removal occurs only in the documented
next major release after the compatibility period, unless a security exception
requires a narrower emergency change.

An optional module or adapter can be deprecated when its owner route is vacant,
its dependency or provider cannot be maintained safely, its replacement is
ready, or its evidence no longer supports the published tier. The deprecation
record must say whether users should migrate, pin a final release, or adopt an
application-owned implementation. Archival is a repository decision after the
module is deprecated and its read-only migration, security, license, and release
records remain available; archive status is not a claim that an old release is
safe to deploy.

## Objective Support Triggers

| Trigger | Required action |
| --- | --- |
| A supported adapter lacks its owner route, direct-test/contract evidence, required release-drift review, or safe dependency posture. | Downgrade it to experimental or block the release until the evidence is restored; update package classification, maturity docs, and release notes together. |
| Provider sandbox evidence is required by an adapter contract but becomes stale, skipped, or failed. | Do not claim current live-provider verification; retain the recorded realism status and require a fresh protected run before promotion or release claim. |
| An owner is unavailable or a critical route has no accepted backup. | Freeze new scope for that route, document the staffing gap, and require another maintainer before expanding the support promise. |
| A provider integration needs business workflow, billing, consent, regional, or incident policy beyond transport safety. | Refuse it from the toolkit surface or keep it application-owned; do not turn product operations into a generic adapter. |
| An optional module repeatedly imports unrelated provider/domain dependencies or cannot release on its own cadence. | Split it only after ADR evidence defines the owner, compatibility baseline, migration path, and test/release burden. |
| A deprecated module has a published destination, migration record, and no remaining supported obligation. | Archive it with its final support/EOL notice; preserve documentation and avoid deleting release verification material. |

Streaming, SSE, WebSocket, and large-download behavior are not a general
toolkit abstraction. Keep them route-specific and follow
[middleware-safety.md](middleware-safety.md); hard-timeout buffering, response
validation, and idempotency replay capture must not be applied globally to
those routes.

## End Of Life

An end-of-life notice names the final supported version, effective date,
security-backport decision, migration destination or lack of one, and the
archive/read-only location. It must be linked from the module's release notes,
support documentation, and package classification where applicable. An EOL
notice is made only by an accepted maintainer with the authority recorded in
EXT-010; a local policy edit cannot create or revoke a support commitment.
