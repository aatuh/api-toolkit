// Package migrate adapts migration execution to contrib database tooling.
//
// Use this package when wiring migration behavior around the contrib migrator
// and database adapters. Migration safety rules, duplicate version handling, and
// uncertain commit acknowledgement are documented in contrib/migrator.
package migrate
