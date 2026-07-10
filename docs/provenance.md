# Release Provenance

Audience: release consumers and reviewers who need to verify where a published
release asset came from before trusting it.

This project publishes **SLSA-style provenance** for its GitHub draft release
assets through GitHub artifact attestations. GitHub describes artifact
attestations as cryptographically signed provenance claims that connect an
artifact to its workflow, repository, commit, and triggering event. See
[GitHub's artifact attestation documentation](https://docs.github.com/en/actions/concepts/security/artifact-attestations).

This document describes the repository's actual controls. It does not claim
SLSA certification or an independently assessed SLSA level. GitHub documents
that artifact attestations by themselves provide SLSA v1.0 Build Level 2; this
repository does not claim Build Level 3 because it does not use a centralized,
trusted reusable build workflow for its releases.

## Provenance Scope

The tag-driven [release workflow](../.github/workflows/release.yml) runs only
for `v*` tags. It creates a GitHub draft release after clean publication
evidence, produces the listed assets, and uses
`actions/attest-build-provenance` with `attestations: write` and `id-token: write`
permissions.

Each published draft release asset below is an attestation subject:

| Asset | Role |
| --- | --- |
| `release-check-summary.json` | Machine-readable release evidence and source commit record. |
| `release-evidence-logs.tgz` | Retained logs for every required release-evidence command. |
| `release-asset-manifest.tsv` | SHA-256 manifest for the uploaded assets. |
| `sbom-root.spdx.json` | Root module SBOM. |
| `sbom-contrib.spdx.json` | Contrib module SBOM. |
| `sbom-root.spdx.json.sig`, `sbom-root.spdx.json.pem` | Keyless Sigstore signature and certificate for the root SBOM. |
| `sbom-contrib.spdx.json.sig`, `sbom-contrib.spdx.json.pem` | Keyless Sigstore signature and certificate for the contrib SBOM. |

The attestation verification policy binds each downloaded subject to the
expected repository `aatuh/api-toolkit` and source reference
`refs/tags/vX.Y.Z`. The local evidence summary records the release commit, but
the online attestation check is the provenance authority for draft-release
assets.

## Consumer Verification

Install authenticated `gh` and `cosign`, then download the draft release assets
into one directory. Replace the tag before running the commands:

```sh
TAG=vX.Y.Z
REPOSITORY=aatuh/api-toolkit
ASSET_DIR="$(mktemp -d)"

gh release download "$TAG" --repo "$REPOSITORY" --dir "$ASSET_DIR" --clobber

RELEASE_ASSET_DIR="$ASSET_DIR" \
RELEASE_TAG="$TAG" \
GITHUB_REPOSITORY="$REPOSITORY" \
make release-artifact-verify
```

Publication mode verifies all of the following before it succeeds:

1. The downloaded files match the asset names declared by
   `release-check-summary.json`.
2. `release-asset-manifest.tsv` matches every downloaded asset checksum.
3. Both SBOMs verify against their keyless Sigstore signatures and expected
   GitHub Actions OIDC workflow identity.
4. The evidence summary is clean publication evidence, including a passed
   provenance policy and no dirty working-tree state.
5. `gh attestation verify` succeeds for every draft-release asset and requires
   `--source-ref "refs/tags/$TAG"`.

The command rejects missing tags and accepts only stable `vX.Y.Z` or release
candidate `vX.Y.0-rc.N` identifiers. This is a tag identifier and source-ref
check, not a cryptographic signature on the Git tag.

For a direct inspection of one subject, run:

```sh
gh attestation verify "$ASSET_DIR/release-check-summary.json" \
  --repo "$REPOSITORY" \
  --source-ref "refs/tags/$TAG"
```

The repository verifier is preferred because it applies the same asset list and
source-reference policy to every subject. See [the release runbook](release-runbook.md)
for the release-side command sequence and [the release review checklist](release-review.md)
for the publication decision.

## What Provenance Does Not Prove

Artifact provenance does not establish that an artifact is secure, free of
vulnerabilities, or suitable for every deployment. It shows where and how an
attested asset was built; consumers still need to apply their own source,
dependency, and deployment policy.

In particular, this project does not make any of these claims:

- Git tags are not cryptographically signed. Tag rulesets are a repository
  control, not a GPG, SSH, or Sigstore tag signature.
- Release outputs are not claimed to be reproducible bit-for-bit.
- Provenance does not cover Go module-proxy zips, container images, downstream
  binaries, or artifacts that are not listed in the draft release asset set.
- Attestations do not replace checksum validation, SBOM signature verification,
  code review, vulnerability triage, or runtime hardening.

Do not publish a release when the required GitHub attestation verification,
checksum verification, or SBOM signature verification fails. Keep the release
draft unpublished, preserve the failed verifier output, and investigate the
release workflow before retrying.

## Maintenance Rule

When adding or removing a release asset, update the release workflow,
`publication_artifact_expectations`, the local artifact verifier, its contract
fixture, and this document in the same change. The docs contract keeps the
declared asset set and all-attestation requirement reviewable.
