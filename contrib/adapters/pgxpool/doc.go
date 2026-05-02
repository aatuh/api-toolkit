// Package pgxpool adapts pgxpool to api-toolkit database ports.
//
// New constructs a Postgres pool adapter for ports.DatabasePool. New
// observability code should prefer StatSnapshot and plain-value snapshot APIs;
// legacy pgx-shaped stats remain compatibility-sensitive in v2.
//
// This contrib adapter is maintained but not part of the stable core API gate.
package pgxpool
