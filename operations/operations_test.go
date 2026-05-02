package operations

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

type operationResult struct {
	Message string `json:"message"`
}

func TestWriteAcceptedSetsHeadersAndBody(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteAccepted(rec, AcceptedConfig{ID: "op_1", Location: "/operations/op_1", RetryAfter: 1500 * time.Millisecond})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if rec.Header().Get("Location") != "/operations/op_1" {
		t.Fatalf("location = %q", rec.Header().Get("Location"))
	}
	if rec.Header().Get("Retry-After") != "2" {
		t.Fatalf("retry-after = %q", rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), `"state":"pending"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestPollHandlerReturnsOperationStates(t *testing.T) {
	result := operationResult{Message: "done"}
	handler := PollHandler(PollConfig[operationResult]{
		Store: StoreFunc[operationResult](func(ctx context.Context, id string) (Operation[operationResult], bool, error) {
			return Operation[operationResult]{ID: id, State: StateSucceeded, Result: &result}, true, nil
		}),
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/operations?id=op_1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"state":"succeeded"`) || !strings.Contains(rec.Body.String(), `"message":"done"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestPollHandlerFailureAndCanceledOperationShapes(t *testing.T) {
	problem := httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeConflict), Title: "Conflict", Detail: "already running"}
	operation := Operation[operationResult]{ID: "op_1", State: StateFailed, Problem: &problem}
	rec := httptest.NewRecorder()
	WriteOperation(rec, http.StatusOK, operation)
	if !strings.Contains(rec.Body.String(), `"state":"failed"`) || !strings.Contains(rec.Body.String(), `"already running"`) {
		t.Fatalf("failed body = %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	WriteOperation(rec, http.StatusOK, Operation[operationResult]{ID: "op_2", State: StateCanceled})
	if !strings.Contains(rec.Body.String(), `"state":"canceled"`) {
		t.Fatalf("canceled body = %s", rec.Body.String())
	}
}

func TestPollHandlerWritesProblemDetailsForMissingNotFoundAndStoreErrors(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		store      Store[operationResult]
		wantStatus int
	}{
		{name: "missing id", url: "/operations", store: StoreFunc[operationResult](func(ctx context.Context, id string) (Operation[operationResult], bool, error) {
			return Operation[operationResult]{}, false, nil
		}), wantStatus: http.StatusBadRequest},
		{name: "not found", url: "/operations?id=missing", store: StoreFunc[operationResult](func(ctx context.Context, id string) (Operation[operationResult], bool, error) {
			return Operation[operationResult]{}, false, nil
		}), wantStatus: http.StatusNotFound},
		{name: "store error", url: "/operations?id=op_1", store: StoreFunc[operationResult](func(ctx context.Context, id string) (Operation[operationResult], bool, error) {
			return Operation[operationResult]{}, false, errors.New("down")
		}), wantStatus: http.StatusInternalServerError},
		{name: "nil store", url: "/operations?id=op_1", wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := PollHandler(PollConfig[operationResult]{Store: tt.store})
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.url, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if rec.Header().Get("Content-Type") != "application/problem+json" {
				t.Fatalf("content type = %q", rec.Header().Get("Content-Type"))
			}
		})
	}
}
