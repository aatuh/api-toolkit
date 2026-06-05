// Package apitest provides HTTP response assertions for application tests.
//
// The package is stable core API for tests. It builds on
// httptest.ResponseRecorder semantics and complements contracttest: contracttest
// checks route registries and OpenAPI metadata, while apitest checks concrete
// HTTP responses emitted by handlers.
//
// Use AssertProblem, AssertProblemCode, AssertValidationFields, AssertJSON,
// AssertHeader, AssertRateLimitHeaders, AssertETag, AssertDeprecationHeaders,
// AssertPagination, AssertOperationAccepted, AssertWebhookSignature, and
// AssertOpenAPIGolden to keep API tests focused on externally visible behavior.
//
// These helpers call testing.TB.Fatal when assertions fail. They are not runtime
// middleware and should not be imported by production handlers. For examples,
// see docs/cookbook.md.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/apitest`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package apitest
