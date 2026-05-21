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
package queryparams
