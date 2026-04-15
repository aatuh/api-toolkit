package scheduler

import (
	"context"
	"errors"
	"sync"
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
