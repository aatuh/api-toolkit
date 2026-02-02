package httpclient

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAndResets(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	current := now
	cb := NewCircuitBreaker(CircuitBreakerOptions{
		FailureThreshold:    1,
		SuccessThreshold:    1,
		OpenTimeout:         time.Second,
		HalfOpenMaxInFlight: 1,
		Now: func() time.Time {
			return current
		},
	})

	fail := func() (*http.Response, error) {
		return nil, errors.New("fail")
	}
	resp, err := cb.Execute(fail, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error from failed call")
	}
	resp, err = cb.Execute(fail, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, ErrBreakerOpen) {
		t.Fatalf("expected breaker open, got %v", err)
	}

	current = current.Add(time.Second)
	ok := func() (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	}
	resp, err = cb.Execute(ok, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("expected breaker to allow, got %v", err)
	}
}
