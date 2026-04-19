package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryIdempotentMethod(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(Options{
		Sleep: func(time.Duration) {},
		Retry: RetryOptions{
			MaxRetries: 2,
		},
	})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, _ := client.Do(req)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestRetrySkipsNonIdempotentMethod(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(Options{
		Sleep: func(time.Duration) {},
		Retry: RetryOptions{
			MaxRetries: 2,
		},
	})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, _ := client.Do(req)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 attempt, got %d", got)
	}
}

func TestRetryReturnsOriginalResponseWhenBodyIsNotReplayable(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream failure"))
	}))
	defer server.Close()

	client := New(Options{
		Sleep: func(time.Duration) {},
		Retry: RetryOptions{
			MaxRetries: 2,
		},
	})
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPut,
		server.URL,
		io.NopCloser(strings.NewReader("payload")),
	)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if req.GetBody != nil {
		t.Fatal("expected request body to be non-replayable")
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("expected original upstream response, got error %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 status, got %d", resp.StatusCode)
	}
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	if string(body) != "upstream failure" {
		t.Fatalf("expected upstream response body, got %q", string(body))
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 attempt for non-replayable body, got %d", got)
	}
}
