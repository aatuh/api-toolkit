package apiclient

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAPIKeyTransportAndRetryAfter(t *testing.T) {
	transport := APIKeyTransport{Key: "secret", Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "ApiKey secret" {
			t.Fatalf("Authorization = %q", got)
		}
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})}
	resp, err := transport.RoundTrip(&http.Request{Header: http.Header{}})
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	header := http.Header{"Retry-After": []string{"2"}}
	if duration, ok := RetryAfter(header); !ok || duration != 2*time.Second {
		t.Fatalf("RetryAfter() = %v, %v", duration, ok)
	}
}

func TestCursorIteratorAndDoJSON(t *testing.T) {
	pages := []CursorPage[string]{{Items: []string{"a"}, NextCursor: "next"}, {Items: []string{"b"}}}
	iterator := CursorIterator[string]{Fetch: func(context.Context, string) (CursorPage[string], error) {
		page := pages[0]
		pages = pages[1:]
		return page, nil
	}}
	if !iterator.Next(context.Background()) || iterator.Items()[0] != "a" {
		t.Fatalf("first page = %#v", iterator.Items())
	}
	if !iterator.Next(context.Background()) || iterator.Items()[0] != "b" {
		t.Fatalf("second page = %#v", iterator.Items())
	}
}
