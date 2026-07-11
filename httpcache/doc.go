// Package httpcache provides stable core conditional request helpers for REST APIs.
//
// Start with StrongETag, WeakETag, HashETag, Validators, EvaluateRead, and
// EvaluateWrite to implement ETag and Last-Modified handling without tying a
// handler to a storage backend. SetValidators, WriteNotModified, and
// WritePreconditionFailed provide the common HTTP response path.
//
// The package only evaluates standard validators: ETag, Last-Modified,
// If-None-Match, If-Modified-Since, If-Match, and If-Unmodified-Since. Callers
// still own resource versioning, authorization, and persistence updates.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/httpcache`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package httpcache
