// Package validation provides validation utilities backed by
// github.com/aatuh/validate/v3.
//
// Public contract:
//   - Validate and ValidateStruct accept structs or pointers to structs.
//   - Unsupported targets return toolkit-owned ValidationError values instead
//     of silently succeeding or leaking upstream validator errors.
//   - ValidateField accepts either Go struct field names or JSON tag names.
//   - Field errors expose JSON field paths, stable validation codes, and safe
//     human-readable messages.
//   - Nil contexts are normalized to context.Background().
package validation
