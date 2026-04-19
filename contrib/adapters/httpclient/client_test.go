package httpclient

import (
	"context"
	"errors"
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

func TestRetryStopsWhenContextCanceledDuringBackoff(t *testing.T) {
	t.Parallel()

	firstAttempt := make(chan struct{})
	client := New(Options{
		DisableTracing: true,
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			select {
			case <-firstAttempt:
			default:
				close(firstAttempt)
			}
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("retry later")),
			}, nil
		}),
		Retry: RetryOptions{
			MaxRetries:     2,
			MinBackoff:     time.Second,
			MaxBackoff:     time.Second,
			MaxElapsedTime: 5 * time.Second,
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	errCh := make(chan error, 1)
	start := time.Now()
	go func() {
		resp, err := client.Do(req)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		errCh <- err
	}()

	select {
	case <-firstAttempt:
	case <-time.After(time.Second):
		t.Fatal("first attempt did not start")
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("retry wait did not stop promptly after cancellation")
	}

	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Fatalf("expected cancellation before full backoff elapsed, got %s", elapsed)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
