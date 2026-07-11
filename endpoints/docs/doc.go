// Package docs registers stable API documentation and OpenAPI endpoints.
//
// Without a registered provider, the default manager discovers OpenAPI files
// from fixed relative paths under the service working directory, such as
// ./swagger/openapi.json and ./docs/openapi.json. Production services should
// prefer RegisterProvider when the OpenAPI source is known at wiring time.
//
// Custom docs managers can expose HTML-mode-specific handler behavior by
// implementing HTMLModeProvider in addition to ManagerContract. These
// package-local aliases remain exactly source-compatible with their root ports
// counterparts during v3.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/endpoints/docs`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package docs
