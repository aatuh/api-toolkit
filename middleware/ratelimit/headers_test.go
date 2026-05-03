package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type headerClock struct{ now time.Time }

func (c headerClock) Now() time.Time { return c.now }

func TestSetRateLimitHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	reset := time.Unix(100, 0).UTC()
	SetRateLimitHeaders(recorder, Quota{Limit: 10, Remaining: 3, Reset: reset, RetryAfter: 2 * time.Second}, DefaultHeaderConfig())
	if recorder.Header().Get("RateLimit-Limit") != "10" || recorder.Header().Get("RateLimit-Remaining") != "3" || recorder.Header().Get("RateLimit-Reset") != "100" || recorder.Header().Get("Retry-After") != "2" {
		t.Fatalf("headers = %#v", recorder.Header())
	}
}

func TestMiddlewareEmitsQuotaHeadersWhenEnabled(t *testing.T) {
	mw, err := New(Options{Capacity: 1, RefillRate: 1, Clock: headerClock{now: time.Unix(100, 0).UTC()}, HeaderConfig: DefaultHeaderConfig()})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Header().Get("RateLimit-Limit") != "1" || recorder.Header().Get("RateLimit-Remaining") != "0" {
		t.Fatalf("allowed headers = %#v", recorder.Header())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
	if recorder.Header().Get("RateLimit-Remaining") != "0" || recorder.Header().Get("Retry-After") == "" {
		t.Fatalf("denied headers = %#v", recorder.Header())
	}
}
