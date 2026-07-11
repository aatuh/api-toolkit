package txpostgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v3/contracts"
)

// DBer aliases the transaction-aware query interface.
type DBer = txpostgres.DBer

// Manager aliases the transaction manager type.
type Manager = txpostgres.Manager

// New constructs a transaction manager over a pgx pool.
func New(pool contracts.DatabasePool) contracts.TxManager { return txpostgres.New(pool) }

// FromCtx retrieves the active connection/tx from context.
func FromCtx(ctx context.Context, pool contracts.DatabasePool) DBer {
	return txpostgres.FromCtx(ctx, pool)
}

// IsNoRows reports whether the error is pgx.ErrNoRows.
func IsNoRows(err error) bool { return txpostgres.IsNoRows(err) }

// AsPgError converts an error to a pgconn.PgError when possible.
func AsPgError(err error) (*pgconn.PgError, bool) {
	return txpostgres.AsPgError(err)
}
