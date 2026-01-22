package idempotency

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	store "github.com/aatuh/api-toolkit/adapters/idempotency"
	"github.com/aatuh/api-toolkit/httpx"
)

func TestIdempotencyReplay(t *testing.T) {
	mem := store.NewMemoryStore()
	mw := New(Options{
		Store:        mem,
		MaxBodyBytes: 1024,
	})
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"body": string(body)})
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/charge", strings.NewReader("alpha"))
	req1.Header.Set("Idempotency-Key", "key-1")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/charge", strings.NewReader("alpha"))
	req2.Header.Set("Idempotency-Key", "key-1")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected 201 on replay, got %d", rec2.Code)
	}
	if rec2.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("expected replay header to be set")
	}
	if rec2.Body.String() != rec1.Body.String() {
		t.Fatalf("expected replayed body to match original")
	}

	req3 := httptest.NewRequest(http.MethodPost, "/charge", strings.NewReader("beta"))
	req3.Header.Set("Idempotency-Key", "key-1")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusConflict {
		t.Fatalf("expected conflict on key reuse, got %d", rec3.Code)
	}
}
