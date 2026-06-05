// Package operations provides HTTP contracts for asynchronous API operations.
//
// It standardizes 202 Accepted responses, pollable operation resources,
// repository contracts, and lifecycle transition helpers while leaving queues,
// workers, persistence, and retry policy to application code or contrib
// adapters.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/operations`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package operations
