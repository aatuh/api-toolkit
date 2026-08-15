# Changelog

This is the concise, user-facing history for published api-toolkit releases.
For detailed dated engineering notes, supported-adapter dispositions, and
release evidence, read [docs/release-notes.md](docs/release-notes.md). The
stable root API contract remains [VERSIONING.md](VERSIONING.md).

## Unreleased

### Security

- `contrib/adapters/chi.Middleware.RealIP` no longer trusts client-supplied
  forwarding headers or rewrites `http.Request.RemoteAddr`. It now resolves the
  direct peer address safely; proxy deployments must use the new explicit
  `chi.ClientIPFromXFF` helper with their trusted proxy CIDRs.

### Upgrade

- If an application previously used `Middleware.RealIP()` behind a reverse
  proxy, replace assumptions about `RemoteAddr` with
  `middleware.GetClientIP(r.Context())` and configure
  `chi.ClientIPFromXFF` with the actual trusted proxy CIDRs.

## [4.0.0] - 2026-07-11

### Breaking

- Root imports move from `github.com/aatuh/api-toolkit/v3/...` to
  `github.com/aatuh/api-toolkit/v4/...`; contrib imports move from
  `github.com/aatuh/api-toolkit/contrib/v3/...` to
  `github.com/aatuh/api-toolkit/contrib/v4/...`.
- Root `ports` now retains only `Logger`, `Clock`, `IDGen`, `NopLogger`, and
  `SystemClock`. Endpoint, middleware, authorization, HTTP, and platform
  contracts use their documented local or contrib-owned replacements.
- JWT/JWK middleware and OAuth2 helpers move to the optional contrib module:
  `contrib/middleware/auth/jwt` and `contrib/oauth2`. Root v4 has no direct
  JWT/JWK dependencies.

### Upgrade

Follow [docs/migration/v4.md](docs/migration/v4.md) for import replacements,
the root-port migration table, and workspace guidance.

## [3.1.2] - 2026-05-21

### Changed

- Strengthened deterministic timeout and HTTP guardrail evidence, including
  observability response-recording behavior.
- Improved package, generated-service, and reference documentation guidance.
- Isolated local Makefile gates from parent Go workspaces.

### Upgrade

No stable root package migration is required. Run the v3 upgrade checks in
[docs/migration/v3.md](docs/migration/v3.md) after updating dependencies.

## [3.1.1] - 2026-05-21

### Changed

- Hardened v3 release, coverage, adapter-maturity, generated-integration, and
  documentation evidence.

### Upgrade

No stable root package migration is required. This release reinforces existing
v3 readiness and release-review checks.

## [3.1.0] - 2026-05-20

### Changed

- Moved the contrib validation adapter to `github.com/aatuh/validate/v3` and
  updated toolkit validation examples to its tag grammar.
- Refreshed supported adapter, OpenTelemetry, JWT/JWK, and workflow dependency
  evidence.

### Upgrade

Review validation tags and adapter error handling when using the contrib
validation adapter. See the detailed [2026-05-20 release notes](docs/release-notes.md#2026-05-20).

## [3.0.2] - 2026-05-19

### Fixed

- Made the contrib CLI publishable and strengthened the release artifact
  verifier contract.

## [3.0.1] - 2026-05-19

### Changed

- Refreshed generated integration fixtures, contrib dependencies, and v3
  maturity evidence.

## [3.0.0] - 2026-05-17

### Breaking

- The root and contrib module paths moved to
  `github.com/aatuh/api-toolkit/v3` and
  `github.com/aatuh/api-toolkit/contrib/v3`.
- Provider-shaped billing exports were removed from root `ports`; use
  `compat/billing` or app-owned billing ports.
- Generic database statistics and the public `response_writer` package were
  removed; use the documented snapshot APIs and `httpx`.
- Idempotency release is token-aware, role middleware construction validates
  configuration, and list helpers retain checked parser APIs.

### Upgrade

Follow [docs/migration/v3.md](docs/migration/v3.md) before moving a service or
generated application from v2 to v3.

## Older v1 and v2 Releases

Older tags remain available in Git history, but the supported line is the
latest release on the default branch. Read `SECURITY.md` for the current
security-backport policy and use the v3 migration guide before upgrading from
v2.

[3.1.2]: https://github.com/aatuh/api-toolkit/compare/v3.1.1...v3.1.2
[4.0.0]: https://github.com/aatuh/api-toolkit/compare/v3.1.2...v4.0.0
[3.1.1]: https://github.com/aatuh/api-toolkit/compare/v3.1.0...v3.1.1
[3.1.0]: https://github.com/aatuh/api-toolkit/compare/v3.0.2...v3.1.0
[3.0.2]: https://github.com/aatuh/api-toolkit/compare/v3.0.1...v3.0.2
[3.0.1]: https://github.com/aatuh/api-toolkit/compare/v3.0.0...v3.0.1
[3.0.0]: https://github.com/aatuh/api-toolkit/compare/v2.1.0...v3.0.0
