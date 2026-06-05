// Package swagstub provides stable stubs for swagger-based workflows.
//
// The package exists so projects can keep optional Swagger tooling isolated from
// runtime HTTP wiring. Prefer the docs endpoint and specs packages for normal
// OpenAPI serving paths.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v3/swagstub`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Compatibility-only API under VERSIONING.md; stable for v3 migration compatibility but not a model for new generic designs.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package swagstub
