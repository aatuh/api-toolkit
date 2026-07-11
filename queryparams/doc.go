// Package queryparams parses stable core collection query parameters.
//
// Start with ParseSortChecked, ParseFiltersChecked, ParseFieldsChecked, and
// ParseIncludesChecked when HTTP handlers need structured field errors for
// sort, filter, sparse fieldset, and include parameters. The unchecked helpers
// remain available when callers prefer ordinary errors.
//
// The package intentionally stops at parsing and validation. Applications map
// parsed fields to database, search, or service-layer queries and must maintain
// their own allowlists through SortConfig and FilterConfig.
//
// Purpose: See the package summary above.
// Import: `github.com/aatuh/api-toolkit/v4/queryparams`.
// Example: See docs/api-reference.md for package example links and docs/cookbook.md for task recipes.
// Errors: Constructors, parsers, and handlers return or write documented errors according to their signatures; packages with plain data types do not add hidden error channels.
// Concurrency: Treat configured middleware and helpers as immutable after construction; request and response values remain request-scoped unless a type documents stronger guarantees.
// Stability: Stable core API under VERSIONING.md and scripts/apicheck.sh.
// When not to use: Prefer net/http, application-owned types, or narrower helpers when this package contract is not needed.
package queryparams
