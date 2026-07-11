// Package specs provides stable OpenAPI registry utilities and endpoint
// constants. The registry supports deterministic code-first operation
// registration, reusable components, operation-level security requirements, and
// top-level security defaults for generated OpenAPI documents. NewRegistry
// preserves OpenAPI 3.0 output, while NewRegistryWithOptions can opt services
// into OpenAPI 3.1 output. Schema helpers cover nullable fields, examples,
// enums, reusable references, media examples, and reusable Problem Details
// response components.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/specs`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package specs
