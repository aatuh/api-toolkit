# Dependency License Policy

Audience: maintainers reviewing dependency additions, dependency updates, and
release evidence.

This policy covers third-party dependencies in the root module, contrib module,
tooling, tests, generated scaffolds, and GitHub Actions. The repository license
is Apache-2.0; this document is about licenses introduced by dependencies.

## Enforcement Source

`.github/dependency-review-config.yml` is the automated pull-request enforcement
source. It enables license checking in the dependency review workflow and uses
an `allow-licenses` list plus narrowly scoped `allow-dependencies-licenses`
exceptions. This document explains how maintainers decide whether to add a
dependency or request an exception; keep the allowed-license table and
exception register below in sync with the config file.

## Allowed Licenses

Dependencies may be added without a license exception when the detected SPDX
license is one of these IDs and the dependency still passes
[dependency-policy.md](dependency-policy.md):

| SPDX ID | Default decision | Notes |
| --- | --- | --- |
| `Apache-2.0` | Allowed | Compatible with the project license and common Go ecosystem dependencies. |
| `BSD-2-Clause` | Allowed | Permissive license. Preserve notices. |
| `BSD-3-Clause` | Allowed | Permissive license. Preserve notices. |
| `ISC` | Allowed | Permissive license. Preserve notices. |
| `MIT` | Allowed | Permissive license. Preserve notices. |
| `MPL-2.0` | Allowed with review | File-level copyleft. Confirm the dependency is linked or used in a way that does not require relicensing project code. |
| `BlueOak-1.0.0` | Allowed | Permissive license. Preserve notices when supplied. |
| `Unlicense` | Allowed | Public-domain-style license. Confirm the package has clear provenance before adding it to stable root code. |

## Disallowed Without Exception

Do not add dependencies with these license classes unless an exception is
recorded before merge:

- strong copyleft licenses such as `GPL-2.0`, `GPL-3.0`, `AGPL-3.0`, or similar,
- network-copyleft or source-availability licenses,
- proprietary, commercial, trial, evaluation, field-of-use, or no-redistribution
  licenses,
- unknown, missing, non-SPDX, or ambiguous license data,
- licenses detected as `LicenseRef-clearlydefined-OTHER` or equivalent
  human-reviewed-but-nonstandard metadata,
- dual-licensed packages unless at least one chosen license is explicitly
  allowed and the dependency record identifies that choice.

## Exception Process

License exceptions are rare. Before adding a dependency outside the allowed
list, record the decision in the pull request and update the policy or config
only after maintainer review.

The review must include:

1. Package name, module path, version, ecosystem, and owning package.
2. Detected license string and source of license evidence.
3. Whether the dependency is root, contrib, tooling, test-only, generated
   scaffold, or GitHub Actions.
4. Why an allowed-license alternative or app-owned code is not sufficient.
5. Runtime distribution impact, including whether the dependency is shipped in
   release artifacts, generated code, binaries, Docker images, or examples.
6. Required notice, attribution, source-offer, or redistribution obligations.
7. Security and dependency-risk review, including `make vuln` when Go code is
   affected.
8. Expiry or re-review trigger for non-permissive or unclear licenses.

If the exception is approved, make the enforcement explicit:

- for a project-wide allowed license, add the SPDX ID to
  `.github/dependency-review-config.yml` and this document in the same commit,
- for a package-specific exception, prefer a narrow
  `allow-dependencies-licenses` entry in `.github/dependency-review-config.yml`
  and document the package, owner, and re-review trigger here,
- for a one-time blocked PR, leave the CI failure in place until the exception
  is documented or the dependency is removed.

## Active Package Exceptions

The following exclusions are exact package-version PURLs, not an allowance for
the detected non-SPDX expression. GitHub dependency review reports
`BSD-3-Clause AND LicenseRef-scancode-google-patent-license-golang` for these
modules, while the corresponding upstream `LICENSE` file in the Go module cache
contains the BSD-3-Clause text.

| Package/version | Scope and owner | Evidence and reason | Distribution and obligations | Re-review trigger |
| --- | --- | --- | --- | --- |
| `pkg:golang/golang.org/x/net@0.57.0` | Contrib runtime; maintainer | The upstream Go module `LICENSE` is BSD-3-Clause. It is a transitive security update required by `google.golang.org/grpc@1.82.1`; substituting an older release would reintroduce a vulnerability. | Linked Go dependency; preserve supplied BSD notice in release-license review. No source offer required. | Any version change, dependency-review metadata change, or 2027-08-15, whichever is first. |
| `pkg:golang/golang.org/x/sync@0.22.0` | Contrib runtime and reference example; maintainer | The upstream Go module `LICENSE` is BSD-3-Clause. It is selected transitively by the patched dependency set. | Linked Go dependency; preserve supplied BSD notice in release-license review. No source offer required. | Any version change, dependency-review metadata change, or 2027-08-15, whichever is first. |
| `pkg:golang/golang.org/x/sys@0.47.0` | Contrib runtime and reference example; maintainer | The upstream Go module `LICENSE` is BSD-3-Clause. It is selected transitively by the patched dependency set. | Linked Go dependency; preserve supplied BSD notice in release-license review. No source offer required. | Any version change, dependency-review metadata change, or 2027-08-15, whichever is first. |
| `pkg:golang/golang.org/x/text@0.40.0` | Contrib runtime and reference example; maintainer | The upstream Go module `LICENSE` is BSD-3-Clause. It is a transitive security update required by the patched dependency set. | Linked Go dependency; preserve supplied BSD notice in release-license review. No source offer required. | Any version change, dependency-review metadata change, or 2027-08-15, whichever is first. |

`REL-000A` recorded this exception together with `make vuln` passing for the
patched dependency set. The exception does not cover other versions, packages,
or a general `LicenseRef-scancode-google-patent-license-golang` allowance.

## Release Review

Before publishing a release, reviewers should confirm:

- dependency-review CI was enabled for pull requests,
- `.github/dependency-review-config.yml` and this document list the same allowed
  licenses,
- every dependency exception has an owner and re-review trigger,
- inspect `dependency-licenses-root.tsv` and
  `dependency-licenses-contrib.tsv` in the draft release. Review every
  `needs_review` or `missing_from_sbom` row against the SBOM, upstream license,
  and any recorded exception before publication,
- release notes mention dependency license changes that affect generated
  scaffolds, supported adapters, or distributed artifacts.
