// Package httpx provides stable core HTTP response and error helpers.
//
// Start with WriteJSON, WriteProblem, WriteErrorWithOptions, ProblemCatalog,
// TypeRegistry, and HeaderLimits when building JSON APIs that need consistent
// RFC 9457 Problem Details, bounded header checks, and safe error mapping.
// The package does not own routing, authentication, persistence, or logging.
//
// ProblemFromErrorWithOptions and WriteErrorWithOptions redact unexpected
// server errors by default; use strict options when internal details must stay
// out of client responses. For request identity and panic recovery helpers, use
// the httpx/identity and httpx/recover subpackages.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/httpx`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package httpx
