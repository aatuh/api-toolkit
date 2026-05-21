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
package httpx
