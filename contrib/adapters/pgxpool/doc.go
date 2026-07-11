// Package pgxpool adapts pgxpool to api-toolkit database ports.
//
// New constructs a Postgres pool adapter for contracts.DatabasePool. New
// observability code should prefer StatSnapshot and plain-value snapshot APIs;
// pgx-shaped stats remain compatibility-sensitive and outside the stable core
// ports surface.
//
// This contrib adapter is maintained but not part of the stable core API gate.
package pgxpool
