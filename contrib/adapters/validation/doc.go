// Package validation provides validation utilities backed by
// go-playground/validator.
//
// Public contract:
//   - Validate and ValidateStruct accept structs or pointers to structs.
//   - Unsupported targets return toolkit-owned ValidationError values instead
//     of silently succeeding or leaking upstream validator errors.
//   - ValidateField accepts either Go struct field names or JSON tag names.
//   - Nil contexts are normalized to context.Background().
package validation
