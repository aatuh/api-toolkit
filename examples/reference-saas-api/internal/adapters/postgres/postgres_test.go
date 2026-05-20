package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCheckRequiredTablesPassesWhenTablesExist(t *testing.T) {
	db := fakeTableQuerier{tables: map[string]bool{
		"public.organizations": true,
		"public.widgets":       true,
	}}
	if err := CheckRequiredTables(context.Background(), db, []string{"organizations", "widgets"}); err != nil {
		t.Fatalf("CheckRequiredTables() error = %v", err)
	}
}

func TestCheckRequiredTablesFailsClosedForMissingTables(t *testing.T) {
	db := fakeTableQuerier{tables: map[string]bool{"public.organizations": true}}
	err := CheckRequiredTables(context.Background(), db, []string{"organizations", "widgets"})
	if !errors.Is(err, ErrMigrationsRequired) {
		t.Fatalf("CheckRequiredTables() error = %v, want %v", err, ErrMigrationsRequired)
	}
}

func TestHealthCheckerRequiresPool(t *testing.T) {
	if err := (HealthChecker{}).Check(context.Background()); !errors.Is(err, ErrPoolRequired) {
		t.Fatalf("HealthChecker.Check() error = %v, want %v", err, ErrPoolRequired)
	}
}

type fakeTableQuerier struct {
	tables map[string]bool
	err    error
}

func (f fakeTableQuerier) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	table, _ := args[0].(string)
	return fakeRow{exists: f.tables[table], err: f.err}
}

type fakeRow struct {
	exists bool
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("one destination is required")
	}
	exists, ok := dest[0].(*bool)
	if !ok {
		return errors.New("bool destination is required")
	}
	*exists = r.exists
	return nil
}
