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
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/ports`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package ports
