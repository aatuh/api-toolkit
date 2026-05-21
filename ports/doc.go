// Package ports provides stable core boundary contracts.
//
// Optional HTTP handler behavior is exposed through exported, narrow capability
// interfaces such as DetailedHealthManager, CachedHealthManager, and
// DocsHTMLModeProvider so external implementations can discover the full public
// contract without depending on package-private handler details.
//
// Contracts in this package stay adapter-neutral. Provider-shaped hosted
// checkout and invoicing contracts live in
// github.com/aatuh/api-toolkit/v3/compat/billing instead of ports. Response
// helpers live in httpx, and database pool statistics should be exposed through
// DatabasePoolSnapshotProvider, SnapshotDatabasePoolStats, or adapter
// StatSnapshot methods rather than driver-shaped contracts.
//
// See docs/ports-surface.md and docs/v3-compatibility-roadmap.md for the v3
// boundary record and replacement guidance.
package ports
