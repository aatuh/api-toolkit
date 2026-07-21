// Package binding decodes HTTP request body, query, and path values into typed
// structs and returns api-toolkit field errors.
//
// Use this package at transport boundaries when a handler needs a small,
// dependency-neutral decoder that produces the same validation Problem Details
// shape as the rest of the toolkit. Business validation and persistence rules
// should still live outside handlers.
//
// Required fields use non-zero decoded values by default for v4 compatibility.
// Set RequiredModePresent in a JSONConfig, QueryConfig, or PathConfig to accept
// explicit false and zero values. JSON null counts as present in that mode;
// use application-level nullable validation when null is not allowed. A path
// resolver must provide PathConfig.ParamPresent to distinguish an empty route
// parameter from an absent one.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/binding`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package binding
