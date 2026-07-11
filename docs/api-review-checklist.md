# API Review Checklist

Audience: maintainers reviewing new exported identifiers or stable behavior
changes.

Public API additions are long-lived. The default answer to a new exported
identifier is "no" until the review proves the package, name, behavior, tests,
docs, and compatibility story are stable enough.

New stable root packages and promotions into stable root API must first pass the
stable API review board process in `docs/governance.md`: public design issue,
at least 7 calendar days for comment, and maintainer approval before
implementation is accepted.

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
| API additions gate | Does every new stable or compatibility-only exported identifier have a doc comment, inventory entry, compile-checked example or exception, and package-tied release note? |
| Stable API review board | For a new stable package or promotion, does the linked design issue show the public comment window and maintainer approval required by governance? |

## Required Evidence

- `GOTOOLCHAIN=local make api-inventory-check`
- `GOTOOLCHAIN=local make api-additions-check`
- `GOTOOLCHAIN=local make docs-check`
- package tests for owned behavior
- compatibility check when stable API changes are in scope
- benchmark or benchmark-deferred rationale for hot paths
- dependency report when imports change

## API Additions Are Forever

`make api-additions-check` compares the current generated stable API inventory
with `API_ADDITIONS_BASE_REF`, `API_BASE_REF`, `GITHUB_BASE_REF`, or `HEAD~1`.
For every new exported identifier in a `stable` or `compatibility-only` root
package, the gate requires all of these:

- a source doc comment on the exported identifier, field, method, const, or var;
- a current `docs/api-inventory.md` row from `make api-inventory`;
- a compile-checked Go example in the package or an exact
  `docs/api-addition-exceptions.tsv` row explaining why an example would be
  misleading or redundant;
- a package-tied `docs/release-notes.md` entry naming the symbol or
  package-qualified symbol.

For a new `github.com/aatuh/api-toolkit/v4/ports` export, the gate also
requires an exact `docs/ports-export-exceptions.tsv` row pointing to an
accepted ADR. The ADR must prove adapter neutrality, at least two real
implementations, and why the application should not own the interface.

## Review Outcomes

| Outcome | Meaning |
| --- | --- |
| Accept stable | The symbol belongs in root stable API and has docs, tests, examples or exception, compatibility notes, and release notes. |
| Accept compatibility-only | The symbol is preserved for v3 users but should not guide new designs. |
| Move to contrib | The behavior depends on providers, databases, routers, telemetry exporters, or generated-app ownership. |
| Keep app-owned | The behavior is business-specific or has only one real implementation. |
| Reject | The symbol widens the public promise without enough evidence. |
