// Package pgxpool provides convenience wiring around the pgxpool adapter.
//
// The package aliases adapter types and returns contracts.DatabasePool from New.
// Prefer direct adapter imports when you need a smaller dependency surface or
// adapter-specific options.
package pgxpool
