// Package queryparams parses common collection query parameters.
//
// It handles sort, filter, sparse fieldset, and include parameters without
// translating them to storage-specific query builders. Applications decide how
// parsed fields map to database, search, or service-layer queries.
package queryparams
