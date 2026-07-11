// Package apiclient provides small client-side helpers for api-toolkit APIs.
//
// The package is stable core API for consumers that want composable helpers
// without generated service-specific SDKs. It decodes Problem Details responses,
// parses Retry-After, builds precondition headers, adds API key credentials,
// signs webhook requests, performs simple JSON requests, and iterates
// cursor-paginated resources.
//
// Start with DoJSON for small JSON request/response flows, DecodeProblem when
// handling non-2xx responses, APIKeyTransport for service-to-service API key
// calls, WebhookSignerTransport for signed outbound webhook calls, and
// CursorIterator for cursor pages.
//
// apiclient does not implement persistence, background retries, circuit
// breaking, generated resource clients, or provider-specific authentication.
// Keep those concerns in application clients or generated SDKs. For examples,
// see docs/cookbook.md.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/apiclient`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package apiclient
