// Package list provides stable list-query parsing and list response helpers.
//
// Use ParseListQueryChecked when handlers need field-level validation errors for
// pagination, filtering, or sorting. The single-return ParseListQuery and parser
// helpers remain available for v2 source compatibility, but new examples should
// prefer the checked APIs when invalid input must produce Problem Details.
//
// See contrib/examples/pagination for a runnable limit/offset endpoint.
package list
