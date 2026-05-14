package async

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunnerProcessesJobsAndRecordsSuccess(t *testing.T) {
	t.Parallel()

	store := &fakeStore{leases: [][]Job{{{ID: "job_1", Kind: "email.send"}}}}
	var handled []string
	ctx, cancel := context.WithCancel(context.Background())
	runner, err := New(Config{
		Store: store,
		Handler: HandlerFunc(func(_ context.Context, job Job) error {
			handled = append(handled, job.ID)
			cancel()
			return nil
		}),
		PollInterval: time.Hour,
		Metrics:      store,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := strings.Join(handled, ","); got != "job_1" {
		t.Fatalf("handled = %q, want job_1", got)
	}
	if got := strings.Join(store.completed, ","); got != "job_1" {
		t.Fatalf("completed = %q, want job_1", got)
	}
	if got := store.events[0].Outcome; got != "succeeded" {
		t.Fatalf("metric outcome = %q, want succeeded", got)
	}
}

func TestRunnerRecordsFailureWithoutLoggingPayloadOrRawError(t *testing.T) {
	t.Parallel()

	secret := "secret-token-123"
	store := &fakeStore{leases: [][]Job{{{
		ID:      "job_1",
		Kind:    "billing.sync",
		Payload: []byte(secret),
	}}}}
	log := &captureLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	runner, err := New(Config{
		Store:  store,
		Logger: log,
		Handler: HandlerFunc(func(context.Context, Job) error {
			cancel()
			return errors.New("provider failed with " + secret)
		}),
		PollInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(store.failed) != 1 {
		t.Fatalf("failed records = %d, want 1", len(store.failed))
	}
	if !strings.Contains(store.failed[0].message, secret) {
		t.Fatalf("stored failure message = %q, want bounded handler error", store.failed[0].message)
	}
	for _, entry := range log.entries {
		if strings.Contains(entry, secret) {
			t.Fatalf("log entry leaked secret: %s", entry)
		}
	}
}

func TestRunnerBoundsConcurrency(t *testing.T) {
	t.Parallel()

	store := &fakeStore{leases: [][]Job{{{ID: "job_1"}, {ID: "job_2"}, {ID: "job_3"}, {ID: "job_4"}}}}
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var (
		mu        sync.Mutex
		active    int
		maxActive int
	)
	runner, err := New(Config{
		Store: store,
		Handler: HandlerFunc(func(context.Context, Job) error {
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			mu.Unlock()
			started <- struct{}{}
			<-release
			mu.Lock()
			active--
			mu.Unlock()
			return nil
		}),
		Concurrency:  2,
		BatchSize:    4,
		PollInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	waitForStarted(t, started, 2)
	mu.Lock()
	gotMax := maxActive
	mu.Unlock()
	if gotMax > 2 {
		t.Fatalf("max active = %d, want <= 2", gotMax)
	}
	cancel()
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestNewValidatesRequiredDependencies(t *testing.T) {
	t.Parallel()

	if _, err := New(Config{}); err == nil {
		t.Fatal("New() without store error = nil")
	}
	if _, err := New(Config{Store: &fakeStore{}}); err == nil {
		t.Fatal("New() without handler error = nil")
	}
}

func TestSafeHelpers(t *testing.T) {
	t.Parallel()

	if got := SafeLabel(" Email Send:Tenant=123 "); got != "email_send_tenant_123" {
		t.Fatalf("SafeLabel() = %q", got)
	}
	if got := SafeLabel(""); got != "unknown" {
		t.Fatalf("SafeLabel(empty) = %q", got)
	}
	if got := SafeFailureMessage(errors.New("line1\nline2")); got != "line1 line2" {
		t.Fatalf("SafeFailureMessage() = %q", got)
	}
}

func waitForStarted(t *testing.T, ch <-chan struct{}, want int) {
	t.Helper()
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	for i := 0; i < want; i++ {
		select {
		case <-ch:
		case <-timer.C:
			t.Fatalf("started workers = %d, want %d", i, want)
		}
	}
}

type fakeStore struct {
	mu        sync.Mutex
	leases    [][]Job
	completed []string
	failed    []failedRecord
	events    []Event
}

func (s *fakeStore) Lease(context.Context, int) ([]Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.leases) == 0 {
		return nil, nil
	}
	jobs := s.leases[0]
	s.leases = s.leases[1:]
	return jobs, nil
}

func (s *fakeStore) Complete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completed = append(s.completed, id)
	return nil
}

func (s *fakeStore) Fail(_ context.Context, id string, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, failedRecord{id: id, message: message})
	return nil
}

func (s *fakeStore) ObserveAsyncJob(_ context.Context, event Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

type failedRecord struct {
	id      string
	message string
}

type captureLogger struct {
	mu      sync.Mutex
	entries []string
}

func (l *captureLogger) Debug(msg string, kv ...any) { l.add(msg, kv...) }
func (l *captureLogger) Info(msg string, kv ...any)  { l.add(msg, kv...) }
func (l *captureLogger) Warn(msg string, kv ...any)  { l.add(msg, kv...) }
func (l *captureLogger) Error(msg string, kv ...any) { l.add(msg, kv...) }

func (l *captureLogger) add(msg string, kv ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, msg+" "+strings.TrimSpace(strings.ReplaceAll(fmt.Sprint(kv...), "\n", " ")))
}
