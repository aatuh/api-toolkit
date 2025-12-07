package pgxpool

import (
	"context"
	"time"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Adapter wraps pgxpool.Pool to implement ports.DatabasePool.
type Adapter struct {
	*pgxpool.Pool
}

// New creates a new database pool adapter.
func New(dsn string) (ports.DatabasePool, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &Adapter{Pool: pool}, nil
}

// Acquire gets a connection from the pool.
func (a *Adapter) Acquire(ctx context.Context) (ports.DatabaseConnection, error) {
	conn, err := a.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	return &Connection{Conn: conn}, nil
}

// Stat returns pool statistics.
func (a *Adapter) Stat() ports.DatabaseStats {
	return &Stats{Stat: a.Pool.Stat()}
}

// Connection wraps pgxpool.Conn to implement ports.DatabaseConnection.
type Connection struct {
	*pgxpool.Conn
}

// Query executes a query and returns rows.
func (c *Connection) Query(ctx context.Context, sql string, args ...any) (ports.DatabaseRows, error) {
	rows, err := c.Conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: rows}, nil
}

// QueryRow executes a query and returns a single row.
func (c *Connection) QueryRow(ctx context.Context, sql string, args ...any) ports.DatabaseRow {
	row := c.Conn.QueryRow(ctx, sql, args...)
	return &Row{Row: row}
}

// Exec executes a query without returning rows.
func (c *Connection) Exec(ctx context.Context, sql string, args ...any) (ports.DatabaseResult, error) {
	result, err := c.Conn.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &Result{CommandTag: result}, nil
}

// Begin starts a transaction.
func (c *Connection) Begin(ctx context.Context) (ports.DatabaseTransaction, error) {
	tx, err := c.Conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &Transaction{Tx: tx}, nil
}

// Rows wraps pgx.Rows to implement ports.DatabaseRows.
type Rows struct {
	pgx.Rows
}

// Row wraps pgx.Row to implement ports.DatabaseRow.
type Row struct {
	pgx.Row
}

// Result wraps pgconn.CommandTag to implement ports.DatabaseResult.
type Result struct {
	pgconn.CommandTag
}

// Transaction wraps pgx.Tx to implement ports.DatabaseTransaction.
type Transaction struct {
	pgx.Tx
}

// Query executes a query and returns rows.
func (t *Transaction) Query(ctx context.Context, sql string, args ...any) (ports.DatabaseRows, error) {
	rows, err := t.Tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: rows}, nil
}

// QueryRow executes a query and returns a single row.
func (t *Transaction) QueryRow(ctx context.Context, sql string, args ...any) ports.DatabaseRow {
	row := t.Tx.QueryRow(ctx, sql, args...)
	return &Row{Row: row}
}

// Exec executes a query without returning rows.
func (t *Transaction) Exec(ctx context.Context, sql string, args ...any) (ports.DatabaseResult, error) {
	result, err := t.Tx.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &Result{CommandTag: result}, nil
}

// Stats wraps pgxpool.Stat to implement ports.DatabaseStats.
type Stats struct {
	*pgxpool.Stat
}

// AcquireCount returns the number of times a connection was acquired from the pool.
func (s *Stats) AcquireCount() int64 {
	return s.Stat.AcquireCount()
}

// AcquireDuration returns the total duration of all connection acquisitions.
func (s *Stats) AcquireDuration() time.Duration {
	return s.Stat.AcquireDuration()
}

// AcquiredConns returns the number of currently acquired connections.
func (s *Stats) AcquiredConns() int32 {
	return s.Stat.AcquiredConns()
}

// CanceledAcquireCount returns the number of times a connection acquisition was canceled.
func (s *Stats) CanceledAcquireCount() int64 {
	return s.Stat.CanceledAcquireCount()
}

// ConstructingConns returns the number of connections currently being constructed.
func (s *Stats) ConstructingConns() int32 {
	return s.Stat.ConstructingConns()
}

// EmptyAcquireCount returns the number of times a connection was requested when the pool was empty.
func (s *Stats) EmptyAcquireCount() int64 {
	return s.Stat.EmptyAcquireCount()
}

// IdleConns returns the number of idle connections.
func (s *Stats) IdleConns() int32 {
	return s.Stat.IdleConns()
}

// MaxConns returns the maximum number of connections in the pool.
func (s *Stats) MaxConns() int32 {
	return s.Stat.MaxConns()
}

// NewConnsCount returns the number of new connections created.
func (s *Stats) NewConnsCount() int64 {
	return s.Stat.NewConnsCount()
}

// TotalConns returns the total number of connections in the pool.
func (s *Stats) TotalConns() int32 {
	return s.Stat.TotalConns()
}
