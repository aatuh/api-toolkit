package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testLastRunProvider struct {
	last time.Time
	ok   bool
	err  error
}

func (p testLastRunProvider) LastFinished(context.Context, string) (time.Time, bool, error) {
	return p.last, p.ok, p.err
}

type recordedRun struct {
	jobName    string
	success    bool
	errMsg     string
	startedAt  time.Time
	finishedAt time.Time
}

type loggedEntry struct {
	msg  string
	args []any
}

type testLogger struct {
	mu     sync.Mutex
	infos  []loggedEntry
	errors []loggedEntry
}

func (l *testLogger) Info(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, loggedEntry{msg: msg, args: append([]any(nil), args...)})
}

func (l *testLogger) Error(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, loggedEntry{msg: msg, args: append([]any(nil), args...)})
}

func TestStartRunsJobImmediatelyAndRepeatedly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	runs := make(chan struct{}, 4)
	runner := New(nil, nil, nil, Job{
		Name:     "sync",
		Interval: 10 * time.Millisecond,
		Run: func(context.Context) error {
			runs <- struct{}{}
			return nil
		},
	})

	runner.Start(ctx)

	waitForRuns(t, runs, 2, 250*time.Millisecond)
}

func TestMaybeRunSkipsWhenLastRunIsWithinInterval(t *testing.T) {
	called := false
	runner := New(nil, nil, testLastRunProvider{
		last: time.Now(),
		ok:   true,
	}, Job{})

	runner.maybeRun(context.Background(), Job{
		Name:     "sync",
		Interval: time.Minute,
		Run: func(context.Context) error {
			called = true
			return nil
		},
	})

	if called {
		t.Fatal("expected recent last run to skip execution")
	}
}

func TestMaybeRunExecutesWhenLastRunIsStale(t *testing.T) {
	called := false
	runner := New(nil, nil, testLastRunProvider{
		last: time.Now().Add(-2 * time.Minute),
		ok:   true,
	}, Job{})

	runner.maybeRun(context.Background(), Job{
		Name:     "sync",
		Interval: time.Minute,
		Run: func(context.Context) error {
			called = true
			return nil
		},
	})

	if !called {
		t.Fatal("expected stale last run to allow execution")
	}
}

func TestExecuteRecordsSuccessAndFailure(t *testing.T) {
	var (
		mu   sync.Mutex
		runs []recordedRun
	)
	rec := RecorderFunc(func(_ context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error {
		mu.Lock()
		defer mu.Unlock()
		runs = append(runs, recordedRun{
			jobName:    jobName,
			success:    success,
			errMsg:     errMsg,
			startedAt:  startedAt,
			finishedAt: finishedAt,
		})
		return nil
	})

	runner := New(nil, rec, nil)
	runner.execute(context.Background(), Job{
		Name:     "ok",
		Interval: time.Minute,
		Run: func(context.Context) error {
			return nil
		},
	})
	runner.execute(context.Background(), Job{
		Name:     "fail",
		Interval: time.Minute,
		Run: func(context.Context) error {
			return errors.New("boom")
		},
	})

	mu.Lock()
	defer mu.Unlock()
	if len(runs) != 2 {
		t.Fatalf("expected 2 recorded runs, got %d", len(runs))
	}
	if runs[0].jobName != "ok" || !runs[0].success || runs[0].errMsg != "" {
		t.Fatalf("unexpected success record: %+v", runs[0])
	}
	if runs[1].jobName != "fail" || runs[1].success || runs[1].errMsg != "boom" {
		t.Fatalf("unexpected failure record: %+v", runs[1])
	}
	if runs[0].finishedAt.Before(runs[0].startedAt) || runs[1].finishedAt.Before(runs[1].startedAt) {
		t.Fatal("expected finishedAt to be on or after startedAt")
	}
}

func TestExecuteRecoversPanicsAndRecordsFailure(t *testing.T) {
	var (
		mu   sync.Mutex
		runs []recordedRun
	)
	rec := RecorderFunc(func(_ context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error {
		mu.Lock()
		defer mu.Unlock()
		runs = append(runs, recordedRun{
			jobName:    jobName,
			success:    success,
			errMsg:     errMsg,
			startedAt:  startedAt,
			finishedAt: finishedAt,
		})
		return nil
	})
	log := &testLogger{}
	runner := New(log, rec, nil)

	runner.execute(context.Background(), Job{
		Name:     "panic",
		Interval: time.Minute,
		Run: func(context.Context) error {
			panic("boom")
		},
	})

	mu.Lock()
	defer mu.Unlock()
	if len(runs) != 1 {
		t.Fatalf("expected 1 recorded run, got %d", len(runs))
	}
	if runs[0].success {
		t.Fatalf("expected panic run to be recorded as failure: %+v", runs[0])
	}
	if runs[0].errMsg != "panic: boom" {
		t.Fatalf("expected panic error message to be recorded, got %q", runs[0].errMsg)
	}
	if len(log.errors) != 1 {
		t.Fatalf("expected one error log, got %d", len(log.errors))
	}
	if log.errors[0].msg != "scheduled job panicked" {
		t.Fatalf("expected panic log message, got %q", log.errors[0].msg)
	}
	if len(log.errors[0].args) < 6 {
		t.Fatalf("expected panic log args including stack, got %#v", log.errors[0].args)
	}
}

func TestStartDoesNotOverlapSameJobAcrossDuplicateStarts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var active atomic.Int32
	started := make(chan struct{}, 2)
	overlap := make(chan struct{}, 1)
	release := make(chan struct{})

	runner := New(nil, nil, nil, Job{
		Name:     "sync",
		Interval: time.Hour,
		Run: func(context.Context) error {
			if active.Add(1) > 1 {
				select {
				case overlap <- struct{}{}:
				default:
				}
			}
			started <- struct{}{}
			<-release
			active.Add(-1)
			return nil
		},
	})

	runner.Start(ctx)
	<-started
	runner.Start(ctx)

	select {
	case <-overlap:
		t.Fatal("expected same job to avoid overlap across duplicate starts")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
}

func waitForRuns(t *testing.T, runs <-chan struct{}, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	count := 0
	for count < want {
		select {
		case <-runs:
			count++
		case <-deadline:
			t.Fatalf("timed out waiting for %d runs, got %d", want, count)
		}
	}
}
