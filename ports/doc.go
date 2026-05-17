// Package ports provides toolkit-wide boundary contracts.
//
// Optional HTTP handler behavior is exposed through exported, narrow capability
// interfaces such as DetailedHealthManager, CachedHealthManager, and
// DocsHTMLModeProvider so external implementations can discover the full public
// contract without depending on package-private handler details.
//
// Most contracts in this package are intended to stay adapter-neutral.
// Compatibility-sensitive exceptions remain stable for v2 source compatibility
// only: the provider-shaped billing contracts in billing.go are deprecated in
// favor of github.com/aatuh/api-toolkit/v3/compat/billing, and the driver-shaped
// database stats contracts in database.go should be replaced in new code by
// DatabasePoolSnapshotProvider, SnapshotDatabasePoolStats, or adapter
// StatSnapshot methods. The legacy response writer package is similarly
// retained for v2 callers while new response code should use httpx.
// See docs/ports-surface.md and docs/v3-compatibility-roadmap.md for the
// containment and removal plan.
package ports
