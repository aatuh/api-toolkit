package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRunnerConcurrentStartAndRecorderFailureHandlerRace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	failureObserved := make(chan struct{}, 1)
	runner := New(nil, RecorderFunc(func(context.Context, string, time.Time, time.Time, bool, string) error {
		return errors.New("persist failed")
	}), nil, Job{
		Name:     "sync",
		Interval: time.Hour,
		Run: func(context.Context) error {
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
			return nil
		},
	})

	stopSetters := make(chan struct{})
	var setters sync.WaitGroup
	for i := 0; i < 8; i++ {
		setters.Add(1)
		go func() {
			defer setters.Done()
			for {
				select {
				case <-stopSetters:
					return
				default:
					runner.SetRecorderFailureHandler(RecorderFailureHandlerFunc(func(context.Context, RecorderFailure) {
						select {
						case failureObserved <- struct{}{}:
						default:
						}
					}))
				}
			}
		}()
	}

	var starters sync.WaitGroup
	for i := 0; i < 16; i++ {
		starters.Add(1)
		go func() {
			defer starters.Done()
			runner.Start(ctx)
		}()
	}
	starters.Wait()

	select {
	case <-started:
	case <-time.After(time.Second):
		close(release)
		close(stopSetters)
		setters.Wait()
		t.Fatal("timed out waiting for scheduled job to start")
	}
	close(release)

	select {
	case <-failureObserved:
	case <-time.After(time.Second):
		close(stopSetters)
		setters.Wait()
		t.Fatal("timed out waiting for recorder failure handler")
	}
	close(stopSetters)
	setters.Wait()
}
