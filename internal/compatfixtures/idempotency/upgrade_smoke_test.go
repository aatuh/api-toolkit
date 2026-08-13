//go:build downstreamcompat

// Package upgradesmoke is copied into a temporary downstream module by the
// compatibility corpus. It exercises a root middleware with the released
// in-memory idempotency adapter.
package upgradesmoke

import (
	"net/http"
	"net/http/httptest"
	"testing"

	idempotencyadapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/idempotency"
	"github.com/aatuh/api-toolkit/v4/middleware/idempotency"
)

func TestIdempotencyConsumerReplaysCompletedResponse(t *testing.T) {
	middleware, err := idempotency.New(idempotency.Options{
		Store:      idempotencyadapter.NewMemoryStore(),
		RequireKey: true,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer middleware.Close()

	calls := 0
	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))

	for requestNumber := 0; requestNumber < 2; requestNumber++ {
		request := httptest.NewRequest(http.MethodPost, "/widgets", nil)
		request.Header.Set("Idempotency-Key", "downstream-widget-create")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusCreated {
			t.Fatalf("request %d status = %d, want %d", requestNumber+1, response.Code, http.StatusCreated)
		}
		if response.Body.String() != "created" {
			t.Fatalf("request %d body = %q, want %q", requestNumber+1, response.Body.String(), "created")
		}
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d, want replay to avoid a second call", calls)
	}
}
