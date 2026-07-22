//go:build postgres

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
	"github.com/aatuh/api-toolkit/contrib/v4/internal/testpostgres"
)

func TestRunsRepoPersistsAndOrdersRealPostgresRuns(t *testing.T) {
	h := testpostgres.New(t)
	ctx := context.Background()
	if err := h.ApplyMigrations(ctx, testpostgres.Migration{
		Name: "scheduler-runs",
		SQL: `CREATE TABLE scheduler_runs (
			job_name text NOT NULL,
			started_at timestamptz NOT NULL,
			finished_at timestamptz NOT NULL,
			success boolean NOT NULL,
			error text NOT NULL
		)`,
	}); err != nil {
		t.Fatalf("create scheduler schema: %v", err)
	}
	repo := NewRunsRepo(&pgxpooladapter.Adapter{Pool: h.Pool})
	start := h.FixedTime()
	first := start.Add(time.Minute)
	second := first.Add(time.Minute)
	if err := repo.Record(ctx, "reconcile", start, first, true, ""); err != nil {
		t.Fatalf("Record(first) error = %v", err)
	}
	if err := repo.Record(ctx, "reconcile", first, second, false, "retry"); err != nil {
		t.Fatalf("Record(second) error = %v", err)
	}
	got, found, err := repo.LastFinished(ctx, "reconcile")
	if err != nil || !found || !got.Equal(second) {
		t.Fatalf("LastFinished() = (%v, %t, %v)", got, found, err)
	}
	if _, found, err := repo.LastFinished(ctx, "missing"); err != nil || found {
		t.Fatalf("missing LastFinished() = (%t, %v)", found, err)
	}
	if err := repo.Record(h.CanceledContext(t), "reconcile", start, second, true, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("Record() canceled context error = %v", err)
	}
}
