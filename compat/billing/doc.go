// Package billing contains the v3 compatibility model for hosted checkout,
// billing portal, invoicing, and generic payment-provider webhooks.
//
// These contracts are defined here directly so provider-shaped billing
// boundaries stay out of the generic ports package. Use this package when the
// hosted-checkout, webhook, invoicing, and billing-portal shape fits the
// application; otherwise define an app-owned port or use a dedicated adapter
// contract.
//
// This package is compatibility-sensitive, not provider-neutral. Applications
// that need a different billing model should define an app-owned port or use a
// dedicated adapter contract instead of widening the stable ports package.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/compat/billing`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Compatibility-only API under VERSIONING.md; stable for v3 migration compatibility but not a model for new generic designs.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package billing
