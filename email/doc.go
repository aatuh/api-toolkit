// Package email defines the stable core email port.
//
// Message is the provider-neutral payload shape used by outbound email senders.
// Sender is intentionally narrow so applications can adapt Resend, SMTP, test,
// or no-op delivery without depending on a provider package from the stable
// core module.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/email`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package email
