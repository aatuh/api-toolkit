# Stable Ports Boundary

This note explains which parts of `github.com/aatuh/api-toolkit/v2/ports` are
meant to stay broadly reusable and which parts remain in v2 only as
compatibility-sensitive surfaces.

## What stays in core

- Generic application boundaries such as logging, clocks, IDs, HTTP, rate limiting, validation, docs, health management, and transaction/query lifecycles stay in core.
- These contracts are the normal template for new `ports` additions: narrow, adapter-neutral, and safe to implement across multiple backends.

## Compatibility-sensitive v2 surfaces

- `ports/billing.go` is stable in v2, but it is currently Stripe-shaped. Hosted checkout, provider customer IDs, billing portal flows, and invoice operations reflect the existing adapter rather than a provider-neutral billing model.
- `ports/database.go` query and transaction interfaces stay in core, but `DatabasePool.Stat` and `DatabaseStats` are stable only as a compatibility surface. They currently mirror pgxpool-style counters.
- `response_writer` is another example of a stable but compatibility-sensitive surface: retained for v2 users, not a template for new design.

## Guidance for new code

- If your application needs billing behavior, prefer an app-owned billing port or a dedicated contrib adapter contract unless the existing v2 billing surface is the exact shape you want.
- If you only need database observability data, depend on `DatabasePoolSnapshotProvider` or `SnapshotDatabasePoolStats` instead of the legacy `DatabaseStats` interface.
- Do not add more provider-specific billing fields or more driver-specific database counters to core `ports` in v2 unless compatibility requires it.

## V3 extraction path

1. Keep the current billing and database-stats exports source-compatible for the rest of v2.
2. Prefer narrow additions in v2 minors, such as plain-value snapshot capabilities, instead of widening the legacy surfaces.
3. In v3, move provider-shaped billing contracts out of core `ports` into a dedicated billing package or adapter-owned module.
4. In v3, replace `DatabasePool.Stat` and `DatabaseStats` in generic call sites with snapshot-based or driver-specific contracts, while keeping query and transaction lifecycles in core.
5. Mark the legacy v2 surfaces deprecated once the v3 replacement packages and migration guidance exist.
