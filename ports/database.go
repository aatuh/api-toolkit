package ports

import (
	"context"
	"time"
)

// DatabasePool defines the interface for database connection pooling.
type DatabasePool interface {
	Ping(ctx context.Context) error
	Close()
	Acquire(ctx context.Context) (DatabaseConnection, error)
	Stat() DatabaseStats
}

// DatabaseConnection defines the interface for individual database connections.
type DatabaseConnection interface {
	Query(ctx context.Context, sql string, args ...any) (DatabaseRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DatabaseRow
	Exec(ctx context.Context, sql string, args ...any) (DatabaseResult, error)
	Begin(ctx context.Context) (DatabaseTransaction, error)
	Release()
}

// DatabaseTransaction defines the interface for database transactions.
type DatabaseTransaction interface {
	Query(ctx context.Context, sql string, args ...any) (DatabaseRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DatabaseRow
	Exec(ctx context.Context, sql string, args ...any) (DatabaseResult, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// DatabaseRows defines the interface for query result rows.
type DatabaseRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// DatabaseRow defines the interface for a single query result row.
type DatabaseRow interface {
	Scan(dest ...any) error
}

// DatabaseResult defines the interface for query execution results.
type DatabaseResult interface {
	RowsAffected() int64
}

// DatabaseStats defines the interface for database pool statistics.
type DatabaseStats interface {
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
