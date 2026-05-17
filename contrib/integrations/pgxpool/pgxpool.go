package pgxpool

import (
	"github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/v3/ports"
)

// Adapter aliases the pgxpool adapter type.
type Adapter = pgxpool.Adapter

// Connection aliases the pgxpool connection wrapper type.
type Connection = pgxpool.Connection

// Rows aliases the pgxpool rows wrapper type.
type Rows = pgxpool.Rows

// Row aliases the pgxpool row wrapper type.
type Row = pgxpool.Row

// Result aliases the pgxpool result wrapper type.
type Result = pgxpool.Result

// Transaction aliases the pgxpool transaction wrapper type.
type Transaction = pgxpool.Transaction

// Stats aliases the legacy pgxpool stats wrapper type.
// New code should prefer Adapter.StatSnapshot and ports.DatabasePoolSnapshot.
type Stats = pgxpool.Stats

// Snapshot aliases the preferred plain-value pool stats type for new code.
type Snapshot = ports.DatabasePoolSnapshot

// New constructs a pgx-backed database pool adapter.
func New(dsn string) (ports.DatabasePool, error) {
	return pgxpool.New(dsn)
}
