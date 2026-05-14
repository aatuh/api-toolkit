package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerRecordsNormalStatusAndUnknownRoute(t *testing.T) {
	recorder := &captureRecorder{}
	mw, err := New(Options{Recorder: recorder})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", nil))
	if rec.Code != http.StatusCreated || rec.Body.String() != "created" {
		t.Fatalf("response = %d %q", rec.Code, rec.Body.String())
	}
	if recorder.counter["method"] != http.MethodPost || recorder.counter["route"] != "unknown" || recorder.counter["status"] != "201" {
		t.Fatalf("counter labels = %#v", recorder.counter)
	}
	if recorder.histValue < 0 || recorder.hist["status"] != "201" {
		t.Fatalf("histogram = %v %#v", recorder.histValue, recorder.hist)
	}
}

func TestHandlerRecordsServeMuxRoutePattern(t *testing.T) {
	recorder := &captureRecorder{}
	mw, err := New(Options{Recorder: recorder})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /organizations/{organization_id}/widgets/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/organizations/org_secret/widgets/widget_secret", nil)
	mw.Handler(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if recorder.counter["route"] != "GET /organizations/{organization_id}/widgets/{id}" {
		t.Fatalf("counter route label = %q, want ServeMux pattern; labels=%#v", recorder.counter["route"], recorder.counter)
	}
	for _, value := range recorder.counter {
		if value == "org_secret" || value == "widget_secret" {
			t.Fatalf("counter labels leaked path value: %#v", recorder.counter)
		}
	}
}

func TestNilMiddlewareAdaptersAndNoopRecorder(t *testing.T) {
	var mw *Middleware
	called := false
	handler := mw.HandlerFunc()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("nil handler called=%v status=%d", called, rec.Code)
	}
	NoopMetrics{}.IncCounter("ignored", Labels{"status": "200"})
	NoopMetrics{}.ObserveHistogram("ignored", 1, Labels{"status": "200"})
}

func TestResponseRecorderFallbacks(t *testing.T) {
	var nilRecorder *responseRecorder
	if nilRecorder.Status() != 0 || nilRecorder.BytesWritten() != 0 || nilRecorder.Committed() || nilRecorder.Unwrap() != nil {
		t.Fatal("nil response recorder should expose zero values")
	}
	if _, err := nilRecorder.Write([]byte("x")); err == nil {
		t.Fatal("expected nil write error")
	}
	if _, _, err := nilRecorder.Hijack(); err == nil {
		t.Fatal("expected nil hijack error")
	}
	if err := nilRecorder.Push("/", nil); err == nil {
		t.Fatal("expected nil push error")
	}
	if _, err := nilRecorder.ReadFrom(nil); err == nil {
		t.Fatal("expected nil readfrom error")
	}
}
