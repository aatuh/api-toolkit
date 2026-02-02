package querylimits

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

func TestQueryLimitsUsesValidationType(t *testing.T) {
	mw, err := New(Options{MaxParams: 1})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/?a=1&b=2", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 status, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	gotType, _ := body["type"].(string)
	wantType := httpx.DefaultTypeURI(httpx.TypeValidation)
	if gotType != wantType {
		t.Fatalf("expected type %q, got %q", wantType, gotType)
	}
}

func TestQueryLimitsRejectsNegativeValues(t *testing.T) {
	if _, err := New(Options{MaxParams: -1}); err == nil {
		t.Fatal("expected error for negative max params")
	}
}
