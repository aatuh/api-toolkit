// Package secure provides stable security header middleware.
//
// APIOnly, DocsUI, and WebApp profiles provide starting points for common HTTP
// surfaces. Cross-origin isolation and CSP template customization are explicit
// options because they can affect browser embeds, documentation UIs, and asset
// loading.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/middleware/secure`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package secure
