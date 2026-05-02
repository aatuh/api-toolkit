package operations

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

// State describes the lifecycle state of an asynchronous operation.
type State string

const (
	// StatePending means work has been accepted but not started.
	StatePending State = "pending"
	// StateRunning means work is in progress.
	StateRunning State = "running"
	// StateSucceeded means work completed successfully.
	StateSucceeded State = "succeeded"
	// StateFailed means work completed with a failure problem.
	StateFailed State = "failed"
	// StateCanceled means work was canceled before successful completion.
	StateCanceled State = "canceled"
)

// Operation is a pollable asynchronous operation resource.
type Operation[T any] struct {
	ID      string         `json:"id"`
	State   State          `json:"state"`
	Result  *T             `json:"result,omitempty"`
	Problem *httpx.Problem `json:"problem,omitempty"`
}

// Store loads operation resources for polling handlers.
type Store[T any] interface {
	GetOperation(ctx context.Context, id string) (Operation[T], bool, error)
}

// StoreFunc adapts a function to Store.
type StoreFunc[T any] func(context.Context, string) (Operation[T], bool, error)

// GetOperation loads an operation resource.
func (f StoreFunc[T]) GetOperation(ctx context.Context, id string) (Operation[T], bool, error) {
	if f == nil {
		return Operation[T]{}, false, fmt.Errorf("operation store function is nil")
	}
	return f(ctx, id)
}

// Accepted is the JSON body for a 202 Accepted operation response.
type Accepted struct {
	ID       string `json:"id,omitempty"`
	State    State  `json:"state"`
	Location string `json:"location,omitempty"`
}

// AcceptedConfig configures WriteAccepted.
type AcceptedConfig struct {
	ID         string
	Location   string
	RetryAfter time.Duration
}

// WriteAccepted writes a 202 Accepted response with Location and Retry-After headers when configured.
func WriteAccepted(w http.ResponseWriter, config AcceptedConfig) {
	if strings.TrimSpace(config.Location) != "" {
		w.Header().Set("Location", strings.TrimSpace(config.Location))
	}
	if config.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSeconds(config.RetryAfter), 10))
	}
	httpx.WriteJSON(w, http.StatusAccepted, Accepted{ID: config.ID, State: StatePending, Location: strings.TrimSpace(config.Location)})
}

// WriteOperation writes an operation resource as JSON.
func WriteOperation[T any](w http.ResponseWriter, status int, operation Operation[T]) {
	if operation.State == "" {
		operation.State = StatePending
	}
	httpx.WriteJSON(w, status, operation)
}

// PollConfig configures PollHandler.
type PollConfig[T any] struct {
	Store       Store[T]
	OperationID func(*http.Request) string
	ErrorWriter func(http.ResponseWriter, int, httpx.Problem)
}

// PollHandler returns an HTTP handler for polling operation state.
func PollHandler[T any](config PollConfig[T]) http.Handler {
	writeProblem := config.ErrorWriter
	if writeProblem == nil {
		writeProblem = httpx.WriteProblem
	}
	operationID := config.OperationID
	if operationID == nil {
		operationID = func(r *http.Request) string { return r.URL.Query().Get("id") }
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if config.Store == nil {
			writeProblem(w, http.StatusInternalServerError, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeInternal), Title: http.StatusText(http.StatusInternalServerError), Detail: "operation store is not configured"})
			return
		}
		id := strings.TrimSpace(operationID(r))
		if id == "" {
			writeProblem(w, http.StatusBadRequest, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeBadRequest), Title: http.StatusText(http.StatusBadRequest), Detail: "operation id is required"})
			return
		}
		operation, ok, err := config.Store.GetOperation(r.Context(), id)
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeInternal), Title: http.StatusText(http.StatusInternalServerError), Detail: "operation lookup failed"})
			return
		}
		if !ok {
			writeProblem(w, http.StatusNotFound, httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeNotFound), Title: http.StatusText(http.StatusNotFound), Detail: "operation not found"})
			return
		}
		if operation.ID == "" {
			operation.ID = id
		}
		WriteOperation(w, http.StatusOK, operation)
	})
}

func retryAfterSeconds(duration time.Duration) int64 {
	return int64((duration + time.Second - time.Nanosecond) / time.Second)
}
