package pgxpool

import (
	"context"
	"errors"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v3/contracts"
)

type fakeDatabasePool struct {
	pingErr error
}

func (fakeDatabasePool) Acquire(context.Context) (contracts.DatabaseConnection, error) {
	return nil, errors.New("not implemented")
}
func (p fakeDatabasePool) Ping(context.Context) error { return p.pingErr }
func (fakeDatabasePool) Close()                       {}

func TestNewWithStartupTimeoutReturnsFactoryPool(t *testing.T) {
	pool, err := newWithStartupTimeout("postgres://db.example/app", func(ctx context.Context, dsn string) (contracts.DatabasePool, error) {
		assertDeadlineWithin(t, ctx, defaultStartupTimeout)
		if dsn != "postgres://db.example/app" {
			t.Fatalf("dsn = %q", dsn)
		}
		return fakeDatabasePool{}, nil
	})
	if err != nil {
		t.Fatalf("newWithStartupTimeout() error = %v", err)
	}
	if pool == nil {
		t.Fatal("expected fake pool")
	}
}

func TestTransactionPropagatesUnderlyingErrors(t *testing.T) {
	wantErr := errors.New("query failed")
	tx := &fakePgxTx{err: wantErr}
	wrapped := &Transaction{Tx: tx}
	if _, err := wrapped.Exec(context.Background(), "update widgets set seen=true"); !errors.Is(err, wantErr) {
		t.Fatalf("Exec() error = %v", err)
	}
	if _, err := wrapped.Query(context.Background(), "select 1"); !errors.Is(err, wantErr) {
		t.Fatalf("Query() error = %v", err)
	}
}
