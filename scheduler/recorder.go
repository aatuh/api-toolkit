package scheduler

import (
	"context"
	"time"
)

// RecorderFunc turns a function into a Recorder.
type RecorderFunc func(ctx context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error

func (f RecorderFunc) Record(ctx context.Context, jobName string, startedAt, finishedAt time.Time, success bool, errMsg string) error {
	return f(ctx, jobName, startedAt, finishedAt, success, errMsg)
}
