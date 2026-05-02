// Package binding decodes HTTP request body, query, and path values into typed
// structs and returns api-toolkit field errors.
//
// Use this package at transport boundaries when a handler needs a small,
// dependency-neutral decoder that produces the same validation Problem Details
// shape as the rest of the toolkit. Business validation and persistence rules
// should still live outside handlers.
package binding
