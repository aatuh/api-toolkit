// Package contracttest provides testing helpers for API contract drift checks.
//
// The helpers assert route contract coverage, OpenAPI operation metadata,
// stable operation identity, scoped security requirements, typed route policy
// metadata, Problem Details response sets, typed problem catalogs, deterministic
// OpenAPI golden output, and conservative operation, policy, security-scheme,
// inherited global-security, inline schema, and component schema compatibility
// in application tests.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/contracttest`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package contracttest
