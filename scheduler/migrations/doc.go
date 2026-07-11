// Package migrations provides stable scheduler migration helpers.
//
// The package supports scheduler storage evolution without coupling scheduler
// users to a specific database adapter. Database-specific execution remains in
// contrib packages.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/scheduler/migrations`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Compatibility-only API under VERSIONING.md; stable for v3 migration compatibility but not a model for new generic designs.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package migrations
