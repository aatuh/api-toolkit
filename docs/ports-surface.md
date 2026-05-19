# Stable Ports Boundary

Audience: maintainers and advanced API consumers reviewing the v3 stable ports
surface and the compatibility-shaped history that should not be copied into new
generic contracts.

This note explains which parts of `github.com/aatuh/api-toolkit/v3/ports` are
broadly reusable and which historical shapes were moved out of generic core
contracts during the v3 cleanup.

## What Stays In Core

- Generic application boundaries such as logging, clocks, IDs, HTTP, rate
  limiting, validation, docs, health management, and transaction/query
  lifecycles stay in core.
- These contracts are the template for new `ports` additions: narrow,
  adapter-neutral, and safe to implement across multiple backends.
- Database observability should use `DatabasePoolSnapshotProvider`,
  `SnapshotDatabasePoolStats`, or adapter `StatSnapshot()` methods.

## Compatibility-Sensitive History

These compatibility-sensitive examples are historical guardrails for new
design, not current invitations to widen generic core ports.

- Historical `ports/billing.go` hosted-checkout and invoicing contracts now live in
  `github.com/aatuh/api-toolkit/v3/compat/billing`. Keep app-owned billing
  ports when the hosted-checkout model is not an exact fit.
- Driver-shaped database counters, including `DatabasePool.Stat` and
  `DatabaseStats`, were removed from the generic pool contract. Keep
  backend-specific counters in adapters and expose plain-value snapshots to
  generic observability code.
- Legacy response helpers were removed from the stable core surface. Use
  `httpx` for JSON and Problem Details responses and package-local response
  recorders for middleware internals.

## Guidance For New Code

- Do not add provider-specific billing fields or driver-specific database
  counters to generic `ports`.
- New examples must use `compat/billing` for the existing hosted-checkout model
  or define an app-owned port for provider-neutral billing.
- New database examples must prefer `DatabasePoolSnapshotProvider`,
  `SnapshotDatabasePoolStats`, `SnapshotDatabaseStats`, or adapter
  `StatSnapshot()` methods.
- New response examples must use `httpx`; middleware that needs capture should
  keep package-local recorders.

## Governance

If the stable ports surface changes, update `VERSIONING.md`,
`docs/package-classification.tsv`, `docs/v3-compatibility-roadmap.md`,
docscheck coverage, and `docs/release-notes.md` in the same change.
