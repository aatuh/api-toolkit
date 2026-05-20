package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDatabaseURLRequired = errors.New("DATABASE_URL is required")
	ErrPoolRequired        = errors.New("postgres pool is required")
	ErrMigrationsRequired  = errors.New("postgres migrations are not applied")
)

var RequiredTables = []string{
	"organizations",
	"memberships",
	"invitations",
	"api_keys",
	"widgets",
	"operations",
	"outbox_events",
	"audit_events",
	"objects",
	"webhook_endpoints",
	"webhook_deliveries",

	// api-toolkit:postgres-required-tables
}

type Pinger interface {
	Ping(context.Context) error
}

type TableQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, ErrDatabaseURLRequired
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

type HealthChecker struct {
	Pool Pinger
}

func (h HealthChecker) Check(ctx context.Context) error {
	if h.Pool == nil {
		return ErrPoolRequired
	}
	return h.Pool.Ping(ctx)
}

func CheckRequiredTables(ctx context.Context, db TableQuerier, tables []string) error {
	if db == nil {
		return ErrPoolRequired
	}
	if len(tables) == 0 {
		tables = RequiredTables
	}
	for _, table := range tables {
		table = strings.TrimSpace(table)
		if table == "" {
			continue
		}
		qualified := "public." + table
		var exists bool
		if err := db.QueryRow(ctx, "select to_regclass($1) is not null", qualified).Scan(&exists); err != nil {
			return fmt.Errorf("check postgres table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("%w: missing table %s", ErrMigrationsRequired, table)
		}
	}
	return nil
}
