package postgres

import (
	"context"
	"time"

	"github.com/aatuh/api-toolkit-contrib/adapters/txpostgres"
	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/scheduler"
)

// RunsRepo stores scheduler job executions.
type RunsRepo struct {
	Pool ports.DatabasePool
}

// NewRunsRepo creates a runs repository backed by Postgres.
func NewRunsRepo(pool ports.DatabasePool) *RunsRepo {
	return &RunsRepo{Pool: pool}
}

// Record stores a completed job run.
func (r *RunsRepo) Record(ctx context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error {
	if r == nil {
		return nil
	}
	db := txpostgres.FromCtx(ctx, r.Pool)
	const q = `
	  insert into scheduler_runs (job_name, started_at, finished_at, success, error)
	  values ($1,$2,$3,$4,$5)
	`
	_, err := db.Exec(ctx, q, jobName, startedAt, finishedAt, success, errMsg)
	return err
}

// LastFinished returns the most recent finished timestamp for a job.
func (r *RunsRepo) LastFinished(ctx context.Context, jobName string) (time.Time, bool, error) {
	if r == nil {
		return time.Time{}, false, nil
	}
	db := txpostgres.FromCtx(ctx, r.Pool)
	const q = `
	  select finished_at
	  from scheduler_runs
	  where job_name=$1
	  order by finished_at desc
	  limit 1
	`
	var ts time.Time
	err := db.QueryRow(ctx, q, jobName).Scan(&ts)
	if err != nil {
		if txpostgres.IsNoRows(err) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return ts, true, nil
}

var _ scheduler.Recorder = (*RunsRepo)(nil)
var _ scheduler.LastRunProvider = (*RunsRepo)(nil)
