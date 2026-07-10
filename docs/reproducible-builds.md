# Reproducible Build Status

Audience: release consumers and maintainers who need to understand whether a
locally built executable can be compared with a project release artifact.

## Status

This project does **not** claim reproducible binaries. GitHub draft releases do
not publish compiled executables for the root module, contrib CLI, or generated
service templates, and the release workflow does not independently rebuild a
binary and compare its hash with a published binary. A local `go build` result
or generated-service build output therefore must not be described as a
bit-for-bit reproduction of an api-toolkit release.

Some generated-service build commands use `-trimpath` and `-buildvcs=false`,
but those flags alone do not establish reproducibility. Generated-service
builds can also receive `VERSION`, `BUILD_COMMIT`, and `BUILD_DATE` metadata,
and the project does not fix every compiler, operating-system image,
architecture, dependency-fetch, or build-environment input for an executable
comparison.

## What Release Verification Establishes

The release workflow publishes and verifies release evidence rather than
executables. Its attested draft-release asset set is:

- `release-check-summary.json`
- `release-evidence-logs.tgz`
- `release-asset-manifest.tsv`
- Root and contrib SPDX SBOMs with their Sigstore signatures and certificates
- Root and contrib SPDX-derived dependency license reports

For this asset set, the published verification command checks:

1. SHA-256 checksums in `release-asset-manifest.tsv`.
2. Keyless Sigstore signatures on both SBOMs and the expected GitHub Actions
   OIDC workflow identity.
3. Clean, passed release evidence and the retained logs it declares.
4. GitHub artifact attestations for every release asset, bound to
   `refs/tags/vX.Y.Z`.

Run the command in [release provenance](provenance.md) to perform those checks.
It establishes the integrity and GitHub build provenance of the listed release
assets. It does not establish that a locally built executable has the same
bytes as an unpublished project binary, because no such reference binary
exists.

`make ci-build-smoke` is also recorded in release evidence. It proves that the
root and contrib modules build in the release workflow; it is a buildability
check, not a deterministic-output comparison.

## Consumer Guidance

Use the tagged source, its module checksums, and the release verification
command when evaluating a release. Build deployment binaries from that source
using the application-specific build instructions, then test and approve them
under the consumer's own platform and configuration policy. Do not use a local
binary hash as a release-verification signal for this project today.

## Future Claim Requirements

Do not claim reproducible binaries until all of the following are in place for
each published binary:

1. Publish the binary and include it in the release manifest and attestation
   subject set.
2. Define and pin the compiler version, target platform, dependency inputs,
   build image, build flags, and any build metadata values.
3. Run a documented independent rebuild that compares the resulting bytes with
   the published binary and fails on a mismatch.
4. Retain the comparison result in release evidence and update the consumer
   verification command and [release runbook](release-runbook.md).

Until then, use the terms "buildability", "checksum verification", "SBOM
signature verification", and "artifact provenance" rather than
"reproducible binary".
