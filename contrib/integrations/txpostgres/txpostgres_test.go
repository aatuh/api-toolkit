package txpostgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	adaptertx "github.com/aatuh/api-toolkit/contrib/v4/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/contracts"
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

func TestNewNilPoolFailsClosed(t *testing.T) {
	called := false
	err := New(nil).WithinTx(context.Background(), func(context.Context) error {
		called = true
		return nil
	})
	if called {
		t.Fatal("expected callback not to run when pool is missing")
	}
	if !errors.Is(err, adaptertx.ErrPoolNotConfigured) {
		t.Fatalf("WithinTx() error = %v, want %v", err, adaptertx.ErrPoolNotConfigured)
	}
}

func TestFromCtxNilPoolFailsClosed(t *testing.T) {
	db := FromCtx(context.Background(), nil)
	if db == nil {
		t.Fatal("expected DB facade")
	}

	_, err := db.Exec(context.Background(), "select 1")
	if !errors.Is(err, adaptertx.ErrPoolNotConfigured) {
		t.Fatalf("Exec() error = %v, want %v", err, adaptertx.ErrPoolNotConfigured)
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
func (*fakePool) Acquire(context.Context) (contracts.DatabaseConnection, error) {
	return nil, errors.New("unused")
}
