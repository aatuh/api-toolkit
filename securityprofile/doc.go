// Package securityprofile composes stable secure middleware defaults.
//
// Profiles bundle common HTTP hardening choices such as body limits, timeouts,
// security headers, and related middleware. Applications should still review
// browser, auth, metrics, and system endpoint settings before deploying a
// profile unchanged. Use StreamingRouteOverride for streaming, SSE, websocket,
// or large-download routes that must opt out of timeout buffering.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/securityprofile`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package securityprofile
