package scheduler

import (
	"context"
	"time"
)

// RecorderFailure captures the context for a failed run-record write.
type RecorderFailure struct {
	JobName    string
	StartedAt  time.Time
	FinishedAt time.Time
	Success    bool
	ErrMsg     string
	Err        error
}

// RecorderFailureHandler receives recorder write failures.
type RecorderFailureHandler interface {
	OnRecorderFailure(ctx context.Context, failure RecorderFailure)
}

// RecorderFailureHandlerFunc turns a function into a RecorderFailureHandler.
type RecorderFailureHandlerFunc func(context.Context, RecorderFailure)

// OnRecorderFailure calls the underlying function.
func (f RecorderFailureHandlerFunc) OnRecorderFailure(
	ctx context.Context, failure RecorderFailure,
) {
	f(ctx, failure)
}

// RecorderFunc turns a function into a Recorder.
type RecorderFunc func(ctx context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error

// Record calls the underlying function.
func (f RecorderFunc) Record(ctx context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error {
	return f(ctx, jobName, startedAt, finishedAt, success, errMsg)
}
