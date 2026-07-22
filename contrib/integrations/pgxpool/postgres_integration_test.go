//go:build postgres

package pgxpool

import (
	"context"
	"testing"

	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
)

func TestNewConnectsIntegrationPoolToRealPostgres(t *testing.T) {
	h := testpostgres.New(t)
	pool, err := New(h.DatabaseURL())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}
