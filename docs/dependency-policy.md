# Dependency Policy

Audience: maintainers reviewing new dependencies, dependency PRs, and release
evidence.

The root module should remain small enough for existing Go HTTP services to
adopt deliberately. Contrib can carry provider, database, observability, and
generator dependencies, but each supported adapter still needs a documented
owner and test strategy.

## Allowed Dependency Classes

| Class | Allowed location | Review requirement |
| --- | --- | --- |
| Go standard library | root and contrib | No special review. |
| Small root helper dependency | root | Must be needed by a stable package and justified against app-owned code. |
| Auth, crypto, JWT, and JWK dependencies | root or contrib | Requires maintainer review, security impact note, vuln check, and package-level tests. |
| Provider SDKs | contrib only | Requires supported-adapter or experimental classification and provider failure-mode tests. |
| Database, cache, queue, and object storage drivers | contrib only | Requires adapter ownership, contract tests, and dependency-risk review. |
| Observability libraries | contrib preferred | Root may expose metadata, but exporters and provider integrations stay contrib. |
| Generator and release tooling | contrib CLI, scripts, or CI | Pin versions, document refresh process, and keep generated code app-owned. |
| Test-only dependencies | package tests or contrib test helpers | Must not leak into root runtime imports. |

## Banned Patterns

- Root stable packages importing contrib packages.
- Root stable packages importing provider SDKs, database drivers, Redis clients,
  OpenTelemetry exporters, router adapters, or generated application code.
- Adding a dependency for a helper smaller than clear app-owned code unless it
  removes real security, parsing, or compatibility risk.
- Introducing network, crypto, auth, archive, or parser dependencies without a
  security review note and a rollback plan.
- Expanding root `ports` to accommodate one provider, one adapter, or one
  generated scaffold.
- Treating generated scaffold dependencies as part of the root library promise.

## Review Gates

Every dependency change must answer:

1. Is the dependency root, contrib, tooling, test-only, or generated-app-owned?
2. Which package imports it, and why is that package the owner?
3. Does it add auth, crypto, parsing, archive, network, database, telemetry, or
   provider behavior?
4. What test covers the behavior or wrapper contract?
5. Does `GOTOOLCHAIN=local make dependency-report` show expected footprint
   movement?
6. Does `GOTOOLCHAIN=local make vuln` report zero called vulnerabilities?
7. If imported-but-not-called findings exist, are they dispositioned in
   `docs/vulnerability-dispositions.tsv` and explained in
   `docs/dependency-risk.md`?

## Update SLA

Use the active SLA in [dependency-risk.md](dependency-risk.md). In short:

- critical or high security updates are reviewed immediately,
- medium or low security updates are triaged within 7 days,
- routine dependency PRs open 14 days need an owner disposition,
- routine dependency PRs open 30 days are refreshed or closed.

## Release Evidence

Release evidence should include:

- root and contrib dependency report from `make dependency-report`,
- vulnerability evidence from `make vuln` and `release-check-summary.json`,
- dependency PR SLA review when external state is relevant,
- release notes for dependency changes that affect supported adapters,
  generated scaffolds, security posture, or runtime behavior.
