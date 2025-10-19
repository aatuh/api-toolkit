// File: api/api-toolkit/txpostgres/txpostgres.go
package txpostgres

import (
	"context"
	"errors"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBer is satisfied by transactions and the pool facade below.
type DBer interface {
	Exec(ctx context.Context, sql string, args ...any) (ports.DatabaseResult, error)
	Query(ctx context.Context, sql string, args ...any) (ports.DatabaseRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) ports.DatabaseRow
}

type txKeyType struct{}

var txKey txKeyType

// Manager implements ports.TxManager using a pgx-like pool.
type Manager struct {
	Pool ports.DatabasePool
}

func New(pool ports.DatabasePool) *Manager { return &Manager{Pool: pool} }

func (m *Manager) WithinTx(
	ctx context.Context, fn func(ctx context.Context) error,
) error {
	conn, err := m.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txCtx := context.WithValue(ctx, txKey, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FromCtx returns the active transaction if present; otherwise a
// facade that acquires/releases a connection per call (no leaks).
func FromCtx(ctx context.Context, pool ports.DatabasePool) DBer {
	if v := ctx.Value(txKey); v != nil {
		if tx, ok := v.(ports.DatabaseTransaction); ok {
			return tx
		}
	}
	return pooledFacade{pool: pool}
}

// pooledFacade acquires/releases a connection for each operation.
type pooledFacade struct {
	pool ports.DatabasePool
}

func (p pooledFacade) Exec(
	ctx context.Context, sql string, args ...any,
) (ports.DatabaseResult, error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	return conn.Exec(ctx, sql, args...)
}

func (p pooledFacade) Query(
	ctx context.Context, sql string, args ...any,
) (ports.DatabaseRows, error) {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		conn.Release()
		return nil, err
	}
	// Release the conn when rows.Close() is called.
	return &rowsWithRelease{rows: rows, conn: conn}, nil
}

func (p pooledFacade) QueryRow(
	ctx context.Context, sql string, args ...any,
) ports.DatabaseRow {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return errRow{err: err}
	}
	row := conn.QueryRow(ctx, sql, args...)
	// Release after Scan() completes.
	return &rowWithRelease{row: row, conn: conn}
}

// rowsWithRelease releases the connection when Close is called.
type rowsWithRelease struct {
	rows ports.DatabaseRows
	conn ports.DatabaseConnection
}

func (r *rowsWithRelease) Next() bool             { return r.rows.Next() }
func (r *rowsWithRelease) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *rowsWithRelease) Err() error             { return r.rows.Err() }
func (r *rowsWithRelease) Close() {
	r.rows.Close()
	r.conn.Release()
}

// rowWithRelease releases the connection after Scan returns.
type rowWithRelease struct {
	row  ports.DatabaseRow
	conn ports.DatabaseConnection
}

func (r *rowWithRelease) Scan(dest ...any) error {
	defer r.conn.Release()
	return r.row.Scan(dest...)
}

// errRow always returns the stored error on Scan.
type errRow struct{ err error }

func (e errRow) Scan(_ ...any) error { return e.err }

// Convenience helpers.

func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func AsPgError(err error) (*pgconn.PgError, bool) {
	var pgErr *pgconn.PgError
	ok := errors.As(err, &pgErr)
	return pgErr, ok
}
