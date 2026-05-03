package idempotent

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequireKeyAndRequestHash(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/widgets?a=1", nil)
	request.Header.Set("Idempotency-Key", " abc ")
	key, err := RequireKey(request, "")
	if err != nil || key != "abc" {
		t.Fatalf("RequireKey() = %q, %v", key, err)
	}
	if RequestHash(request, []byte(`{"name":"a"}`)) != RequestHash(request, []byte(`{"name":"a"}`)) {
		t.Fatalf("request hash is not deterministic")
	}
}

func TestWriteConflictAndAcceptedReplay(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteConflict(recorder, "conflict")
	if recorder.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	WriteAcceptedReplay(recorder, AsyncConfig{ID: "op_1", Location: "/operations/op_1", RetryAfter: 2 * time.Second})
	if recorder.Code != http.StatusAccepted || recorder.Header().Get("Idempotency-Replayed") != "true" || recorder.Header().Get("Retry-After") != "2" {
		t.Fatalf("accepted replay code=%d headers=%#v", recorder.Code, recorder.Header())
	}
}

func TestOperationExtensions(t *testing.T) {
	extensions := OperationExtensions(true)
	contract := extensions["x-idempotency-key"].(map[string]any)
	if contract["required"] != true || contract["header"] != "Idempotency-Key" {
		t.Fatalf("extensions = %#v", extensions)
	}
}
