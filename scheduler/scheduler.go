package scheduler

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

const recorderCleanupTimeout = 5 * time.Second

// Job represents a periodic task.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

// Runner executes jobs on a ticker until ctx is cancelled.
type Runner struct {
	jobs      []Job
	log       Logger
	rec       Recorder
	lastRuns  LastRunProvider
	recFail   RecorderFailureHandler
	mu        sync.Mutex
	inFlight  map[string]bool
	scheduled map[string]bool
}

// Logger is a minimal interface for logging.
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

// Recorder persists job runs.
//
// Persistence errors are observability failures. Runner implementations should
// surface them, but a failed write must not retroactively change the outcome of
// a job function that already ran to completion.
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
		jobs:      jobs,
		log:       log,
		rec:       rec,
		lastRuns:  last,
		inFlight:  make(map[string]bool),
		scheduled: make(map[string]bool),
	}
}

// SetRecorderFailureHandler configures a callback for recorder persistence
// failures. Set it before calling Start.
func (r *Runner) SetRecorderFailureHandler(handler RecorderFailureHandler) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recFail = handler
}

// Start launches all jobs in separate goroutines.
func (r *Runner) Start(ctx context.Context) {
	for i, job := range r.jobs {
		j := job
		if j.Interval <= 0 || j.Run == nil {
			continue
		}
		key := scheduleKey(i, j)
		if !r.beginSchedule(key) {
			continue
		}
		go func() {
			defer r.finishSchedule(key)
			r.runJob(ctx, j)
		}()
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
	var (
		err        error
		panicStack string
	)
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("panic: %v", recovered)
				panicStack = string(debug.Stack())
			}
		}()
		err = job.Run(ctx)
	}()
	end := time.Now()
	if err != nil && r.log != nil {
		if panicStack != "" {
			r.log.Error("scheduled job panicked", "job", job.Name, "error", err.Error(), "stack", panicStack)
		} else {
			r.log.Error("scheduled job failed", "job", job.Name, "error", err.Error())
		}
	}
	if err == nil && r.log != nil {
		r.log.Info("scheduled job complete", "job", job.Name, "elapsed_ms", end.Sub(start).Milliseconds())
	}
	if r.rec != nil {
		runErrMsg := errMsg(err)
		recordCtx, cancel := recorderContext(ctx)
		defer cancel()
		if recErr := r.rec.Record(recordCtx, job.Name, start, end, err == nil, runErrMsg); recErr != nil {
			if r.log != nil {
				r.log.Error(
					"scheduled job record failed",
					"job", job.Name,
					"success", err == nil,
					"job_error", runErrMsg,
					"record_error", recErr.Error(),
					"started_at", start.String(),
					"finished_at", end.String(),
					"elapsed_ms", end.Sub(start).Milliseconds(),
				)
			}
			if handler := r.recorderFailureHandler(); handler != nil {
				handler.OnRecorderFailure(recordCtx, RecorderFailure{
					JobName:    job.Name,
					StartedAt:  start,
					FinishedAt: end,
					Success:    err == nil,
					ErrMsg:     runErrMsg,
					Err:        recErr,
				})
			}
		}
	}
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func recorderContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(
		context.WithoutCancel(ctx), recorderCleanupTimeout,
	)
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

func (r *Runner) beginSchedule(jobKey string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.scheduled == nil {
		r.scheduled = make(map[string]bool)
	}
	if r.scheduled[jobKey] {
		return false
	}
	r.scheduled[jobKey] = true
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

func (r *Runner) finishSchedule(jobKey string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.scheduled, jobKey)
}

func scheduleKey(index int, job Job) string {
	if job.Name != "" {
		return "job:" + job.Name
	}
	return fmt.Sprintf("job#%d", index)
}

func (r *Runner) recorderFailureHandler() RecorderFailureHandler {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recFail
}
