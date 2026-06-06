# Security Policy

## Supported Versions

We provide security updates for the latest release on the default branch. Older
versions may receive fixes at our discretion. The supported Go and platform
line is documented in `docs/support-policy.md`.

## Maintainer Availability

This project is currently maintained by a single maintainer. Security reports
are prioritized over routine issues, feature requests, and non-security pull
requests, but there is no 24/7 coverage or commercial support guarantee.

The acknowledgement and remediation targets below are best-effort maintainer
targets for supported versions. If the maintainer is unavailable and a fix is
urgent for your deployment, pin a safe release, apply a temporary fork, or carry
an application-level mitigation while coordinated disclosure continues.

## Reporting a Vulnerability

Please report security issues privately using GitHub Security Advisories:

1. Go to the repository Security tab.
2. Click "Report a vulnerability".

Do not open public issues for suspected vulnerabilities.

## What to Expect

- Acknowledgement within 3 business days.
- A remediation plan or request for more details as needed.
- Coordinated disclosure once a fix is available.

## Remediation Targets

Targets start when the maintainer confirms that the report affects a supported
version and has enough detail to reproduce or reason about the issue. Targets
may change for coordinated disclosure, upstream fixes, embargoed dependency
issues, or incomplete reports.

| Severity | Triage target | Remediation target |
| --- | --- | --- |
| Critical | Confirm impact and mitigation path within 1 business day after acknowledgement. | Patch, workaround, or advisory target within 7 calendar days. |
| High | Confirm impact and mitigation path within 3 business days after acknowledgement. | Patch, workaround, or advisory target within 14 calendar days. |
| Medium | Confirm impact and mitigation path within 7 business days after acknowledgement. | Patch, workaround, or advisory target within 30 calendar days. |
| Low | Confirm impact and next release path within 14 business days after acknowledgement. | Fix in the next suitable release, with a 90 calendar day target when a fix is needed. |

Severity is based on exploitability, affected supported versions, reachable
code paths, confidentiality/integrity/availability impact, and whether a safe
workaround exists. Reports stay private until a fix, workaround, or coordinated
publication date is ready.

## Dependency Updates (Dependabot)

Dependabot is enabled for:

- Go modules in `/` and `/contrib`
- GitHub Actions in `/`

Security updates are surfaced automatically as pull requests.

## Release Signing (Sigstore/cosign)

Release SBOMs are signed using Sigstore/cosign via GitHub OIDC.
Download the release assets into one directory before verifying signatures.
Replace `v3.1.2` with the release tag you are checking:

```sh
TAG=v3.1.2
```

Verify the root module SBOM:

```sh
cosign verify-blob \
  --certificate sbom-root.spdx.json.pem \
  --signature sbom-root.spdx.json.sig \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity "https://github.com/aatuh/api-toolkit/.github/workflows/release.yml@refs/tags/${TAG}" \
  sbom-root.spdx.json
```

Verify the contrib module SBOM:

```sh
cosign verify-blob \
  --certificate sbom-contrib.spdx.json.pem \
  --signature sbom-contrib.spdx.json.sig \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity "https://github.com/aatuh/api-toolkit/.github/workflows/release.yml@refs/tags/${TAG}" \
  sbom-contrib.spdx.json
```

To verify the full downloaded release asset bundle, including manifest
checksums, retained release logs, SBOM signatures, and GitHub provenance
attestations, run:

```sh
RELEASE_ASSET_DIR=/path/to/downloaded/assets \
  RELEASE_ARTIFACT_VERIFY_MODE=publication \
  RELEASE_TAG="${TAG}" \
  GITHUB_REPOSITORY=aatuh/api-toolkit \
  make release-artifact-verify
```
