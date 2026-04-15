package scheduler

import (
	"context"
	"sync"
	"time"
)

// Job represents a periodic task.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

// Runner executes jobs on a ticker until ctx is cancelled.
type Runner struct {
	jobs     []Job
	log      Logger
	rec      Recorder
	lastRuns LastRunProvider
	mu       sync.Mutex
	inFlight map[string]bool
}

// Logger is a minimal interface for logging.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// Recorder persists job runs.
type Recorder interface {
	Record(ctx context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error
}

// LastRunProvider exposes last finished run for a job.
type LastRunProvider interface {
	LastFinished(ctx context.Context, jobName string) (time.Time, bool, error)
}

// New creates a Runner.
func New(log Logger, rec Recorder, last LastRunProvider, jobs ...Job) *Runner {
	return &Runner{
		jobs:     jobs,
		log:      log,
		rec:      rec,
		lastRuns: last,
		inFlight: make(map[string]bool),
	}
}

// Start launches all jobs in separate goroutines.
func (r *Runner) Start(ctx context.Context) {
	for _, job := range r.jobs {
		j := job
		if j.Interval <= 0 || j.Run == nil {
			continue
		}
		go r.runJob(ctx, j)
	}
}

func (r *Runner) runJob(ctx context.Context, job Job) {
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	// Run once on start if allowed by interval/last run.
	r.maybeRun(ctx, job)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.maybeRun(ctx, job)
		}
	}
}

func (r *Runner) maybeRun(ctx context.Context, job Job) {
	if job.Interval > 0 && r.lastRuns != nil {
		if last, ok, err := r.lastRuns.LastFinished(ctx, job.Name); err == nil && ok {
			if time.Since(last) < job.Interval {
				return
			}
		}
	}
	if !r.beginRun(job.Name) {
		return
	}
	defer r.finishRun(job.Name)
	r.execute(ctx, job)
}

func (r *Runner) execute(ctx context.Context, job Job) {
	start := time.Now()
	if r.log != nil {
		r.log.Info("scheduled job start", "job", job.Name, "start", start.String())
	}
	err := job.Run(ctx)
	end := time.Now()
	if err != nil && r.log != nil {
		r.log.Error("scheduled job failed", "job", job.Name, "error", err.Error())
	}
	if err == nil && r.log != nil {
		r.log.Info("scheduled job complete", "job", job.Name, "elapsed_ms", end.Sub(start).Milliseconds())
	}
	if r.rec != nil {
		_ = r.rec.Record(ctx, job.Name, start, end, err == nil, errMsg(err))
	}
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (r *Runner) beginRun(jobName string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.inFlight == nil {
		r.inFlight = make(map[string]bool)
	}
	if r.inFlight[jobName] {
		return false
	}
	r.inFlight[jobName] = true
	return true
}

func (r *Runner) finishRun(jobName string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.inFlight, jobName)
}
