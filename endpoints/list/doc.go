// Package list provides stable list-query parsing and list response helpers.
//
// Use ParseListQueryChecked when handlers need field-level validation errors for
// pagination, filtering, or sorting. The single-return ParseListQuery and parser
// helpers remain available as compatibility shims, but new examples should
// prefer the checked APIs when invalid input must produce Problem Details.
//
// See contrib/examples/pagination for a runnable limit/offset endpoint.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/endpoints/list`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package list
