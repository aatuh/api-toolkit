# API Review Checklist

Audience: maintainers reviewing new exported identifiers or stable behavior
changes.

Public API additions are long-lived. The default answer to a new exported
identifier is "no" until the review proves the package, name, behavior, tests,
docs, and compatibility story are stable enough.

## Required Review

| Area | Questions |
| --- | --- |
| Package fit | Does the symbol belong in a stable root package, contrib, generated app code, or app-owned code? |
| Naming | Is the name short, clear, lower-case package scoped, and consistent with Go naming conventions? |
| Zero value | Is the zero value useful, safe, or explicitly invalid with deterministic validation? |
| Options validation | Do `Options` or config structs reject invalid values and document defaults? |
| Context and cancellation | Does long-running, network, storage, clock, lock, retry, or goroutine behavior accept or propagate `context.Context`? |
| Error behavior | Are sentinel errors, typed errors, wrapping, Problem Details mapping, and matching examples clear? |
| Concurrency | Are values immutable, request-scoped, synchronized, or documented as not safe for concurrent use? |
| Resource lifecycle | Does anything that opens network, files, timers, goroutines, caches, stores, or background workers expose ownership or shutdown behavior? |
| Return types | Are return values concrete enough for callers but not overfit to one adapter or provider? |
| Exported interface necessity | Is the interface implemented by at least two real implementations, user-owned, adapter-owned, or test-only? |
| Dependency impact | Does the symbol pull in new modules, auth/crypto/network behavior, provider SDKs, database drivers, or generated-app dependencies? |
| Examples and docs | Is there an example or an explicit exception, package docs, and a docs/api-inventory.md update? |
| Compatibility | Does the change preserve same-import-path compatibility or document a major-version-only migration? |
| Release notes | Does `docs/release-notes.md` mention behavior, compatibility, security, dependency, or generated scaffold impact? |

## Required Evidence

- `GOTOOLCHAIN=local make api-inventory-check`
- `GOTOOLCHAIN=local make docs-check`
- package tests for owned behavior
- compatibility check when stable API changes are in scope
- benchmark or benchmark-deferred rationale for hot paths
- dependency report when imports change

## Review Outcomes

| Outcome | Meaning |
| --- | --- |
| Accept stable | The symbol belongs in root stable API and has docs, tests, examples or exception, compatibility notes, and release notes. |
| Accept compatibility-only | The symbol is preserved for v3 users but should not guide new designs. |
| Move to contrib | The behavior depends on providers, databases, routers, telemetry exporters, or generated-app ownership. |
| Keep app-owned | The behavior is business-specific or has only one real implementation. |
| Reject | The symbol widens the public promise without enough evidence. |
