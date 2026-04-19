package txpostgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aatuh/api-toolkit/v2/ports"
)

func TestNewReturnsManager(t *testing.T) {
	manager := New(&fakePool{})
	if manager == nil {
		t.Fatal("expected tx manager")
	}
	if _, ok := manager.(*Manager); !ok {
		t.Fatalf("manager type = %T, want *Manager", manager)
	}
}

func TestHelperFunctions(t *testing.T) {
	if !IsNoRows(pgx.ErrNoRows) {
		t.Fatal("expected pgx.ErrNoRows to be recognized")
	}
	if IsNoRows(errors.New("other")) {
		t.Fatal("did not expect unrelated error to be recognized")
	}

	want := &pgconn.PgError{Code: "23505"}
	got, ok := AsPgError(want)
	if !ok {
		t.Fatal("expected pg error cast to succeed")
	}
	if got != want {
		t.Fatalf("AsPgError() = %v, want %v", got, want)
	}
}

type fakePool struct{}

func (*fakePool) Ping(context.Context) error { return nil }
func (*fakePool) Close()                     {}
func (*fakePool) Acquire(context.Context) (ports.DatabaseConnection, error) {
	return nil, errors.New("unused")
}
func (*fakePool) Stat() ports.DatabaseStats { return nil }
