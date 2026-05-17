package pgxpool

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v3/ports"
)

func TestNewUsesBoundedStartupContext(t *testing.T) {
	wantErr := errors.New("stop after context capture")

	_, err := newWithStartupTimeout("postgres://db.example/internal", func(ctx context.Context, dsn string) (ports.DatabasePool, error) {
		assertDeadlineWithin(t, ctx, defaultStartupTimeout)
		if dsn != "postgres://db.example/internal" {
			t.Fatalf("dsn = %q", dsn)
		}
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("newWithStartupTimeout() error = %v, want %v", err, wantErr)
	}
}

func assertDeadlineWithin(t *testing.T, ctx context.Context, want time.Duration) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context missing deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		t.Fatalf("context deadline already expired: %s", remaining)
	}
	if remaining > want || remaining < want-time.Second {
		t.Fatalf("context deadline = %s from now, want roughly %s", remaining, want)
	}
}
