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
package apiclient
