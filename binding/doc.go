// Package binding decodes HTTP request body, query, and path values into typed
// structs and returns api-toolkit field errors.
//
// Use this package at transport boundaries when a handler needs a small,
// dependency-neutral decoder that produces the same validation Problem Details
// shape as the rest of the toolkit. Business validation and persistence rules
// should still live outside handlers.
//
// Required fields preserve v4's existing non-zero and source-specific defaults.
// Set a decoder config's RequiredMode to RequiredModePresent when a field may be
// explicitly supplied as false, zero, empty, or null. Applications should keep
// semantic non-null and non-zero rules in their own validation layer.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/binding`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package binding
