// Package upload decodes multipart upload requests with validation errors.
//
// The package is stable core API. It owns transport-level multipart parsing,
// request and file size checks, required file fields, content-type allowlists,
// and Problem Details-compatible field errors. It does not own object storage,
// virus scanning, media processing, persistence, or authorization.
//
// Use DecodeMultipart with Config to parse a request into a Form, then
// RequireFile to fetch required file fields by name. AllowedContentTypes and
// MaxFileBytes are small helpers for constructing readable Config values.
// ValidationProblem and WriteValidationProblem preserve the same error shape as
// binding and httpx.
//
// Always apply authentication, authorization, total request limits, and
// application-owned scanning before trusting uploaded content. For examples, see
// docs/cookbook.md.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/upload`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package upload
