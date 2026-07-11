package pgxpool

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
)

const defaultStartupTimeout = 5 * time.Second

type poolFactory func(context.Context, string) (contracts.DatabasePool, error)

type poolStatsSnapshotSource interface {
	AcquireCount() int64
	AcquireDuration() time.Duration
	AcquiredConns() int32
	CanceledAcquireCount() int64
	ConstructingConns() int32
	EmptyAcquireCount() int64
	IdleConns() int32
	MaxConns() int32
	NewConnsCount() int64
	TotalConns() int32
}

// Adapter wraps pgxpool.Pool to implement contracts.DatabasePool.
type Adapter struct {
	*pgxpool.Pool
}

// New creates a new database pool adapter using a bounded startup context.
// Callers that need direct context control should use NewWithContext.
func New(dsn string) (contracts.DatabasePool, error) {
	return newWithStartupTimeout(dsn, NewWithContext)
}

// NewWithContext creates a new database pool adapter with a context.
func NewWithContext(ctx context.Context, dsn string) (contracts.DatabasePool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &Adapter{Pool: pool}, nil
}

func newWithStartupTimeout(dsn string, open poolFactory) (contracts.DatabasePool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultStartupTimeout)
	defer cancel()
	return open(ctx, dsn)
}

// Acquire gets a connection from the pool.
func (a *Adapter) Acquire(ctx context.Context) (contracts.DatabaseConnection, error) {
	conn, err := a.Pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	return &Connection{Conn: conn}, nil
}

// Stat returns the pgx-shaped database stats wrapper for adapter-specific callers.
func (a *Adapter) Stat() *Stats {
	return &Stats{Stat: a.Pool.Stat()}
}

// StatSnapshot returns pool statistics as plain values for new call sites.
func (a *Adapter) StatSnapshot() contracts.DatabasePoolSnapshot {
	if a == nil || a.Pool == nil {
		return contracts.DatabasePoolSnapshot{}
	}
	return snapshotFromPoolStats(a.Pool.Stat())
}

// Connection wraps pgxpool.Conn to implement contracts.DatabaseConnection.
type Connection struct {
	*pgxpool.Conn
}

// Query executes a query and returns rows.
func (c *Connection) Query(ctx context.Context, sql string, args ...any) (contracts.DatabaseRows, error) {
	rows, err := c.Conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: rows}, nil
}

// QueryRow executes a query and returns a single row.
func (c *Connection) QueryRow(ctx context.Context, sql string, args ...any) contracts.DatabaseRow {
	row := c.Conn.QueryRow(ctx, sql, args...)
	return &Row{Row: row}
}

// Exec executes a query without returning rows.
func (c *Connection) Exec(ctx context.Context, sql string, args ...any) (contracts.DatabaseResult, error) {
	result, err := c.Conn.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &Result{CommandTag: result}, nil
}

// Begin starts a transaction.
func (c *Connection) Begin(ctx context.Context) (contracts.DatabaseTransaction, error) {
	tx, err := c.Conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &Transaction{Tx: tx}, nil
}

// Rows wraps pgx.Rows to implement contracts.DatabaseRows.
type Rows struct {
	pgx.Rows
}

// Row wraps pgx.Row to implement contracts.DatabaseRow.
type Row struct {
	pgx.Row
}

// Result wraps pgconn.CommandTag to implement contracts.DatabaseResult.
type Result struct {
	pgconn.CommandTag
}

// Transaction wraps pgx.Tx to implement contracts.DatabaseTransaction.
type Transaction struct {
	pgx.Tx
}

// Query executes a query and returns rows.
func (t *Transaction) Query(ctx context.Context, sql string, args ...any) (contracts.DatabaseRows, error) {
	rows, err := t.Tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &Rows{Rows: rows}, nil
}

// QueryRow executes a query and returns a single row.
func (t *Transaction) QueryRow(ctx context.Context, sql string, args ...any) contracts.DatabaseRow {
	row := t.Tx.QueryRow(ctx, sql, args...)
	return &Row{Row: row}
}

// Exec executes a query without returning rows.
func (t *Transaction) Exec(ctx context.Context, sql string, args ...any) (contracts.DatabaseResult, error) {
	result, err := t.Tx.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &Result{CommandTag: result}, nil
}

// Stats wraps pgxpool.Stat for adapter-specific callers that need pgx-shaped counters.
type Stats struct {
	*pgxpool.Stat
}

func snapshotFromPoolStats(stats poolStatsSnapshotSource) contracts.DatabasePoolSnapshot {
	if stats == nil {
		return contracts.DatabasePoolSnapshot{}
	}

	return contracts.DatabasePoolSnapshot{
		AcquireCount:         stats.AcquireCount(),
		AcquireDuration:      stats.AcquireDuration(),
		AcquiredConns:        stats.AcquiredConns(),
		CanceledAcquireCount: stats.CanceledAcquireCount(),
		ConstructingConns:    stats.ConstructingConns(),
		EmptyAcquireCount:    stats.EmptyAcquireCount(),
		IdleConns:            stats.IdleConns(),
		MaxConns:             stats.MaxConns(),
		NewConnsCount:        stats.NewConnsCount(),
		TotalConns:           stats.TotalConns(),
	}
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
