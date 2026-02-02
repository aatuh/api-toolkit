# Security Policy

## Supported Versions

We provide security updates for the latest release on the default branch.
Older versions may receive fixes at our discretion.

## Reporting a Vulnerability

Please report security issues privately using GitHub Security Advisories:

1. Go to the repository Security tab.
2. Click "Report a vulnerability".

Do not open public issues for suspected vulnerabilities.

## What to Expect

- Acknowledgement within 3 business days.
- A remediation plan or request for more details as needed.
- Coordinated disclosure once a fix is available.

## Dependency Updates (Dependabot)

Dependabot is enabled for:

- Go modules in `/` and `/contrib`
- GitHub Actions in `/`

Security updates are surfaced automatically as pull requests.

## Release Signing (Sigstore/cosign)

Release SBOMs are signed using Sigstore/cosign via GitHub OIDC.
To verify a signature for a release artifact:

```sh
cosign verify-blob \
  --certificate sbom-root.spdx.json.pem \
  --signature sbom-root.spdx.json.sig \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity "https://github.com/aatuh/api-toolkit/.github/workflows/release.yml@refs/tags/<tag>" \
  sbom-root.spdx.json
```
