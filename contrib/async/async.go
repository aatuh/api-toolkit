package async

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/v2/ports"
)

const (
	defaultBatchSize    = 1
	defaultConcurrency  = 1
	defaultPollInterval = time.Second
	maxLabelLength      = 64
	maxFailureLength    = 256
)

// Job is leased from a durable async queue or outbox.
type Job struct {
	ID       string
	Kind     string
	TenantID string
	Payload  []byte
	Attempts int
}

// Store leases jobs and records their final outcome.
type Store interface {
	Lease(ctx context.Context, limit int) ([]Job, error)
	Complete(ctx context.Context, id string) error
	Fail(ctx context.Context, id string, message string) error
}

// Handler executes one leased job.
type Handler interface {
	Handle(ctx context.Context, job Job) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(context.Context, Job) error

// Handle executes a job.
func (f HandlerFunc) Handle(ctx context.Context, job Job) error {
	if f == nil {
		return errors.New("async handler function is nil")
	}
	return f(ctx, job)
}

// Event captures low-cardinality worker observations.
type Event struct {
	Kind    string
	Outcome string
}

// MetricsRecorder records worker outcomes.
type MetricsRecorder interface {
	ObserveAsyncJob(ctx context.Context, event Event)
}

// MetricsRecorderFunc adapts a function to MetricsRecorder.
type MetricsRecorderFunc func(context.Context, Event)

// ObserveAsyncJob records an async job event.
func (f MetricsRecorderFunc) ObserveAsyncJob(ctx context.Context, event Event) {
	if f != nil {
		f(ctx, event)
	}
}

// Config configures a Runner.
type Config struct {
	Store        Store
	Handler      Handler
	Logger       ports.Logger
	Metrics      MetricsRecorder
	BatchSize    int
	Concurrency  int
	PollInterval time.Duration
}

// Runner leases and executes async jobs until the context is canceled.
type Runner struct {
	store        Store
	handler      Handler
	log          ports.Logger
	metrics      MetricsRecorder
	batchSize    int
	concurrency  int
	pollInterval time.Duration
}

// New constructs a Runner with safe defaults.
func New(cfg Config) (*Runner, error) {
	if cfg.Store == nil {
		return nil, errors.New("async store is required")
	}
	if cfg.Handler == nil {
		return nil, errors.New("async handler is required")
	}
	log := cfg.Logger
	if log == nil {
		log = ports.NopLogger{}
	}
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	return &Runner{
		store:        cfg.Store,
		handler:      cfg.Handler,
		log:          log,
		metrics:      cfg.Metrics,
		batchSize:    batchSize,
		concurrency:  concurrency,
		pollInterval: pollInterval,
	}, nil
}

// Run starts the worker loop and blocks until ctx is canceled and in-flight jobs finish.
func (r *Runner) Run(ctx context.Context) error {
	if r == nil {
		return errors.New("async runner is nil")
	}
	ctx = normalizeContext(ctx)
	sem := make(chan struct{}, r.concurrency)
	var wg sync.WaitGroup
	defer wg.Wait()

	r.poll(ctx, sem, &wg)
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.poll(ctx, sem, &wg)
		}
	}
}

func (r *Runner) poll(ctx context.Context, sem chan struct{}, wg *sync.WaitGroup) {
	if ctx.Err() != nil {
		return
	}
	jobs, err := r.store.Lease(ctx, r.batchSize)
	if err != nil {
		r.log.Warn("async lease failed", "outcome", "lease_error", "error_kind", errorKind(err))
		r.observe(ctx, Event{Outcome: "lease_error"})
		return
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return
		}
		select {
		case sem <- struct{}{}:
			wg.Add(1)
			go func(job Job) {
				defer func() {
					<-sem
					wg.Done()
				}()
				r.runJob(ctx, job)
			}(job)
		case <-ctx.Done():
			return
		}
	}
}

func (r *Runner) runJob(ctx context.Context, job Job) {
	event := Event{Kind: SafeLabel(job.Kind)}
	r.log.Debug("async job start", "job_id", job.ID, "job_kind", event.Kind)
	err := r.handleJob(ctx, job)
	if err == nil {
		if completeErr := r.store.Complete(ctx, job.ID); completeErr != nil {
			r.log.Error("async job complete record failed", "job_id", job.ID, "job_kind", event.Kind, "outcome", "complete_error", "error_kind", errorKind(completeErr))
			r.observe(ctx, Event{Kind: event.Kind, Outcome: "complete_error"})
			return
		}
		r.log.Info("async job complete", "job_id", job.ID, "job_kind", event.Kind, "outcome", "succeeded")
		r.observe(ctx, Event{Kind: event.Kind, Outcome: "succeeded"})
		return
	}
	message := SafeFailureMessage(err)
	if failErr := r.store.Fail(ctx, job.ID, message); failErr != nil {
		r.log.Error("async job failure record failed", "job_id", job.ID, "job_kind", event.Kind, "outcome", "fail_record_error", "error_kind", errorKind(failErr))
		r.observe(ctx, Event{Kind: event.Kind, Outcome: "fail_record_error"})
		return
	}
	outcome := "failed"
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		outcome = "canceled"
	}
	r.log.Warn("async job failed", "job_id", job.ID, "job_kind", event.Kind, "outcome", outcome, "error_kind", errorKind(err))
	r.observe(ctx, Event{Kind: event.Kind, Outcome: outcome})
}

func (r *Runner) handleJob(ctx context.Context, job Job) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %T", recovered)
		}
	}()
	return r.handler.Handle(ctx, job)
}

func (r *Runner) observe(ctx context.Context, event Event) {
	if r.metrics == nil {
		return
	}
	event.Kind = SafeLabel(event.Kind)
	event.Outcome = SafeLabel(event.Outcome)
	r.metrics.ObserveAsyncJob(ctx, event)
}

// SafeLabel returns a bounded label for metrics and logs.
func SafeLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			out.WriteRune(r)
		default:
			out.WriteRune('_')
		}
		if out.Len() >= maxLabelLength {
			break
		}
	}
	if out.Len() == 0 {
		return "unknown"
	}
	return out.String()
}

// SafeFailureMessage bounds failure messages before durable storage.
func SafeFailureMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 32 {
			return -1
		}
		return r
	}, err.Error())
	message = strings.TrimSpace(message)
	if len(message) > maxFailureLength {
		message = message[:maxFailureLength]
	}
	if message == "" {
		return "worker failed"
	}
	return message
}

func errorKind(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	default:
		return "error"
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
