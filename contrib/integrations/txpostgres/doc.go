// Package txpostgres provides convenience wiring around the Postgres transaction adapter.
//
// The package aliases transaction helpers for applications that want short import
// paths during bootstrap. Missing pool wiring fails closed through the underlying
// adapter rather than panicking.
package txpostgres
