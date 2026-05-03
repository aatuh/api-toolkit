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
package apitest
