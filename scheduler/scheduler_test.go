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

func TestStartAcceptsNilContext(t *testing.T) {
	runner := New(nil, nil, nil, Job{
		Name:     "disabled",
		Interval: 0,
		Run: func(context.Context) error {
			t.Fatal("job with non-positive interval should not run")
			return nil
		},
	})

	assertNotPanics(t, func() {
		runner.Start(nil) //nolint:staticcheck // Regression coverage for public nil-context handling.
	})
}

func TestExecuteNormalizesNilContext(t *testing.T) {
	runner := New(nil, nil, nil)

	runner.execute(nil, Job{ //nolint:staticcheck // Regression coverage for nil-context run paths.
		Name:     "sync",
		Interval: time.Minute,
		Run: func(ctx context.Context) error {
			if ctx == nil {
				t.Fatal("expected non-nil job context")
			}
			return nil
		},
	})
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

func TestExecuteSurfacesRecorderFailuresWithoutChangingJobOutcome(t *testing.T) {
	tests := []struct {
		name            string
		run             func(context.Context) error
		wantSuccess     bool
		wantErrMsg      string
		wantErrorLogMsg []string
		wantInfoLogMsg  []string
	}{
		{
			name:        "successful job",
			run:         func(context.Context) error { return nil },
			wantSuccess: true,
			wantErrMsg:  "",
			wantErrorLogMsg: []string{
				"scheduled job record failed",
			},
			wantInfoLogMsg: []string{
				"scheduled job start",
				"scheduled job complete",
			},
		},
		{
			name:        "failed job",
			run:         func(context.Context) error { return errors.New("boom") },
			wantSuccess: false,
			wantErrMsg:  "boom",
			wantErrorLogMsg: []string{
				"scheduled job failed",
				"scheduled job record failed",
			},
			wantInfoLogMsg: []string{
				"scheduled job start",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &testLogger{}
			recErr := errors.New("persist failed")
			runner := New(log, RecorderFunc(func(context.Context, string, time.Time, time.Time, bool, string) error {
				return recErr
			}), nil)

			var failures []RecorderFailure
			runner.SetRecorderFailureHandler(RecorderFailureHandlerFunc(
				func(_ context.Context, failure RecorderFailure) {
					failures = append(failures, failure)
				},
			))

			runner.execute(context.Background(), Job{
				Name:     "sync",
				Interval: time.Minute,
				Run:      tt.run,
			})

			if len(failures) != 1 {
				t.Fatalf("expected 1 recorder failure callback, got %d", len(failures))
			}
			failure := failures[0]
			if failure.JobName != "sync" {
				t.Fatalf("failure job = %q, want sync", failure.JobName)
			}
			if failure.Success != tt.wantSuccess {
				t.Fatalf("failure success = %t, want %t", failure.Success, tt.wantSuccess)
			}
			if failure.ErrMsg != tt.wantErrMsg {
				t.Fatalf("failure errMsg = %q, want %q", failure.ErrMsg, tt.wantErrMsg)
			}
			if !errors.Is(failure.Err, recErr) {
				t.Fatalf("failure error = %v, want %v", failure.Err, recErr)
			}
			if failure.FinishedAt.Before(failure.StartedAt) {
				t.Fatal("expected recorder failure timestamps to stay ordered")
			}

			assertLoggedMessages(t, log.infos, tt.wantInfoLogMsg)
			assertLoggedMessages(t, log.errors, tt.wantErrorLogMsg)

			recordLog := log.errors[len(log.errors)-1]
			if got := valueForKey(recordLog.args, "record_error"); got != recErr.Error() {
				t.Fatalf("record error log field = %v, want %q", got, recErr.Error())
			}
			if got := valueForKey(recordLog.args, "job_error"); got != tt.wantErrMsg {
				t.Fatalf("job error log field = %v, want %q", got, tt.wantErrMsg)
			}
		})
	}
}

func TestExecuteRecordsWithCleanupContextAfterCancellation(t *testing.T) {
	type contextKey string

	const key contextKey = "trace_id"

	baseCtx, cancel := context.WithCancel(context.WithValue(context.Background(), key, "trace-123"))
	recorderCalled := false
	runner := New(nil, RecorderFunc(func(ctx context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error {
		recorderCalled = true
		if err := ctx.Err(); err != nil {
			t.Fatalf("recorder context err = %v, want nil", err)
		}
		if got := ctx.Value(key); got != "trace-123" {
			t.Fatalf("recorder context value = %v, want trace-123", got)
		}
		if jobName != "sync" {
			t.Fatalf("recorder job = %q, want sync", jobName)
		}
		if !success {
			t.Fatal("expected successful job to stay successful")
		}
		if errMsg != "" {
			t.Fatalf("recorder errMsg = %q, want empty", errMsg)
		}
		if finishedAt.Before(startedAt) {
			t.Fatal("expected finishedAt to be on or after startedAt")
		}
		return nil
	}), nil)

	runner.execute(baseCtx, Job{
		Name:     "sync",
		Interval: time.Minute,
		Run: func(context.Context) error {
			cancel()
			return nil
		},
	})

	if !recorderCalled {
		t.Fatal("expected recorder to be called")
	}
}

func TestExecuteSurfacesRecorderFailureWithCleanupContextAfterCancellation(t *testing.T) {
	type contextKey string

	const key contextKey = "trace_id"

	log := &testLogger{}
	baseCtx, cancel := context.WithCancel(context.WithValue(context.Background(), key, "trace-123"))
	recErr := errors.New("persist failed")
	runner := New(log, RecorderFunc(func(ctx context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error {
		if err := ctx.Err(); err != nil {
			t.Fatalf("recorder context err = %v, want nil", err)
		}
		if got := ctx.Value(key); got != "trace-123" {
			t.Fatalf("recorder context value = %v, want trace-123", got)
		}
		if jobName != "sync" {
			t.Fatalf("recorder job = %q, want sync", jobName)
		}
		if !success {
			t.Fatal("expected successful job to stay successful")
		}
		if errMsg != "" {
			t.Fatalf("recorder errMsg = %q, want empty", errMsg)
		}
		if finishedAt.Before(startedAt) {
			t.Fatal("expected finishedAt to be on or after startedAt")
		}
		return recErr
	}), nil)

	var (
		handlerCalled bool
		failureCtxErr error
		failureValue  any
		failure       RecorderFailure
	)
	runner.SetRecorderFailureHandler(RecorderFailureHandlerFunc(func(ctx context.Context, got RecorderFailure) {
		handlerCalled = true
		failureCtxErr = ctx.Err()
		failureValue = ctx.Value(key)
		failure = got
	}))

	runner.execute(baseCtx, Job{
		Name:     "sync",
		Interval: time.Minute,
		Run: func(context.Context) error {
			cancel()
			return nil
		},
	})

	if !handlerCalled {
		t.Fatal("expected recorder failure handler to be called")
	}
	if failureCtxErr != nil {
		t.Fatalf("failure handler context err = %v, want nil", failureCtxErr)
	}
	if failureValue != "trace-123" {
		t.Fatalf("failure handler context value = %v, want trace-123", failureValue)
	}
	if failure.JobName != "sync" {
		t.Fatalf("failure job = %q, want sync", failure.JobName)
	}
	if !failure.Success {
		t.Fatal("expected recorder failure to preserve successful job outcome")
	}
	if failure.ErrMsg != "" {
		t.Fatalf("failure errMsg = %q, want empty", failure.ErrMsg)
	}
	if !errors.Is(failure.Err, recErr) {
		t.Fatalf("failure err = %v, want %v", failure.Err, recErr)
	}
	if failure.FinishedAt.Before(failure.StartedAt) {
		t.Fatal("expected recorder failure timestamps to stay ordered")
	}

	assertLoggedMessages(t, log.infos, []string{
		"scheduled job start",
		"scheduled job complete",
	})
	assertLoggedMessages(t, log.errors, []string{
		"scheduled job record failed",
	})

	recordLog := log.errors[0]
	if got := valueForKey(recordLog.args, "record_error"); got != recErr.Error() {
		t.Fatalf("record error log field = %v, want %q", got, recErr.Error())
	}
	if got := valueForKey(recordLog.args, "job_error"); got != "" {
		t.Fatalf("job error log field = %v, want empty", got)
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

func TestStartIsIdempotentPerJob(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const interval = 80 * time.Millisecond
	runs := make(chan struct{}, 4)
	runner := New(nil, nil, nil, Job{
		Name:     "sync",
		Interval: interval,
		Run: func(context.Context) error {
			runs <- struct{}{}
			return nil
		},
	})

	runner.Start(ctx)
	waitForRuns(t, runs, 1, 100*time.Millisecond)

	runner.Start(ctx)

	select {
	case <-runs:
		t.Fatal("expected duplicate Start not to trigger another immediate run")
	case <-time.After(interval / 3):
	}

	waitForRuns(t, runs, 1, 2*interval)
}

func TestStartContinuesSchedulingAfterJobPanic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	var runs atomic.Int32
	completed := make(chan struct{}, 1)
	runner := New(nil, nil, nil, Job{
		Name:     "panic-once",
		Interval: 40 * time.Millisecond,
		Run: func(context.Context) error {
			if runs.Add(1) == 1 {
				panic("boom")
			}
			select {
			case completed <- struct{}{}:
			default:
			}
			return nil
		},
	})

	runner.Start(ctx)

	select {
	case <-completed:
	case <-ctx.Done():
		t.Fatal("expected scheduler to continue after panic and run job again")
	}
}

func TestStartSkipsDuplicateNamedJobsInSameRunner(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runs := make(chan string, 4)
	runner := New(nil, nil, nil,
		Job{
			Name:     "sync",
			Interval: time.Hour,
			Run: func(context.Context) error {
				runs <- "first"
				return nil
			},
		},
		Job{
			Name:     "sync",
			Interval: time.Hour,
			Run: func(context.Context) error {
				runs <- "second"
				return nil
			},
		},
	)

	runner.Start(ctx)

	select {
	case <-runs:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for first named job run")
	}

	select {
	case name := <-runs:
		t.Fatalf("expected duplicate named job to be skipped, got extra run from %q", name)
	case <-time.After(50 * time.Millisecond):
	}
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

func assertLoggedMessages(t *testing.T, entries []loggedEntry, want []string) {
	t.Helper()
	if len(entries) != len(want) {
		t.Fatalf("log entries = %d, want %d (%v)", len(entries), len(want), want)
	}
	for i, msg := range want {
		if entries[i].msg != msg {
			t.Fatalf("log entry %d = %q, want %q", i, entries[i].msg, msg)
		}
	}
}

func valueForKey(args []any, key string) any {
	for i := 0; i+1 < len(args); i += 2 {
		name, ok := args[i].(string)
		if ok && name == key {
			return args[i+1]
		}
	}
	return nil
}

func assertNotPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
