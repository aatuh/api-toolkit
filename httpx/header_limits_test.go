package httpx

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestHeaderCountAndBytes(t *testing.T) {
	h := http.Header{}
	h.Add("X-One", "a")
	h.Add("X-Two", "bb")
	h.Add("X-Two", "cc")

	if got := HeaderCount(h); got != 3 {
		t.Fatalf("expected count 3, got %d", got)
	}
	wantBytes := len("X-One") + len("a") + 2 +
		len("X-Two") + len("bb") + 2 +
		len("X-Two") + len("cc") + 2
	if got := HeaderBytes(h); got != wantBytes {
		t.Fatalf("expected bytes %d, got %d", wantBytes, got)
	}
}

func TestHeaderLimitsCheck(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-One", "a")
	req.Header.Set("X-Two", "bb")

	limits := HeaderLimits{MaxCount: 1}
	if err := limits.Check(req); !errors.Is(err, ErrHeaderCountExceeded) {
		t.Fatalf("expected count error, got %v", err)
	}

	limits = HeaderLimits{MaxBytes: 1}
	if err := limits.Check(req); !errors.Is(err, ErrHeaderBytesExceeded) {
		t.Fatalf("expected bytes error, got %v", err)
	}
}
