package apiclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

type signerFunc func(context.Context, []byte) (string, error)

func (f signerFunc) SignWebhook(ctx context.Context, body []byte) (string, error) {
	return f(ctx, body)
}

func TestDecodeProblemAndProblemError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(strings.NewReader(`{"type":"https://example.com/problems/rate-limited","title":"Too Many Requests","status":429,"detail":"slow down","instance":"/requests/1","retryable":true}`)),
	}
	problem, err := DecodeProblem(resp)
	if err != nil {
		t.Fatalf("DecodeProblem() error = %v", err)
	}
	if problem.StatusCode != http.StatusTooManyRequests || problem.Problem.Detail != "slow down" {
		t.Fatalf("problem = %#v", problem)
	}
	if problem.Error() != "slow down" {
		t.Fatalf("Error() = %q", problem.Error())
	}
	if problem.Problem.Ext["retryable"] != true {
		t.Fatalf("extensions = %#v", problem.Problem.Ext)
	}
	if got := (&ProblemError{StatusCode: http.StatusConflict, Problem: httpx.Problem{Title: "Conflict"}}).Error(); got != "Conflict" {
		t.Fatalf("title fallback = %q", got)
	}
	if got := (&ProblemError{StatusCode: http.StatusTeapot}).Error(); got != http.StatusText(http.StatusTeapot) {
		t.Fatalf("status fallback = %q", got)
	}
	var nilProblem *ProblemError
	if got := nilProblem.Error(); got != "api problem" {
		t.Fatalf("nil fallback = %q", got)
	}
}

func TestDecodeProblemRejectsNilAndMalformedResponses(t *testing.T) {
	if _, err := DecodeProblem(nil); err == nil {
		t.Fatal("expected nil response error")
	}
	resp := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader("not-json"))}
	if _, err := DecodeProblem(resp); err == nil {
		t.Fatal("expected malformed problem error")
	}
}

func TestRetryAfterAndPreconditionHeaders(t *testing.T) {
	when := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if duration, ok := RetryAfter(http.Header{"Retry-After": []string{when.Format(http.TimeFormat)}}); !ok || duration <= 0 {
		t.Fatalf("RetryAfter(http-date) = %v, %v", duration, ok)
	}
	if duration, ok := RetryAfter(http.Header{"Retry-After": []string{time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)}}); !ok || duration != 0 {
		t.Fatalf("RetryAfter(past) = %v, %v", duration, ok)
	}
	if _, ok := RetryAfter(http.Header{"Retry-After": []string{"tomorrow"}}); ok {
		t.Fatal("expected invalid Retry-After to fail")
	}
	lastModified := time.Date(2026, 5, 3, 10, 11, 12, 0, time.FixedZone("EET", 2*60*60))
	headers := PreconditionHeaders("  \"abc\"  ", lastModified)
	if headers.Get("If-Match") != "\"abc\"" {
		t.Fatalf("If-Match = %q", headers.Get("If-Match"))
	}
	if headers.Get("If-Unmodified-Since") != lastModified.UTC().Format(http.TimeFormat) {
		t.Fatalf("If-Unmodified-Since = %q", headers.Get("If-Unmodified-Since"))
	}
	if got := PreconditionHeaders(" ", time.Time{}); len(got) != 0 {
		t.Fatalf("empty preconditions = %#v", got)
	}
}

func TestCursorIteratorTerminalCases(t *testing.T) {
	var nilIterator *CursorIterator[string]
	if nilIterator.Next(context.Background()) || nilIterator.Items() != nil || nilIterator.Err() != nil {
		t.Fatal("nil iterator should be inert")
	}
	iterator := CursorIterator[string]{}
	if iterator.Next(context.Background()) {
		t.Fatal("iterator without fetch should not advance")
	}
	wantErr := errors.New("fetch failed")
	iterator = CursorIterator[string]{Fetch: func(context.Context, string) (CursorPage[string], error) {
		return CursorPage[string]{}, wantErr
	}}
	if iterator.Next(context.Background()) {
		t.Fatal("iterator with fetch error should not advance")
	}
	if !errors.Is(iterator.Err(), wantErr) {
		t.Fatalf("Err() = %v", iterator.Err())
	}
}

func TestAPIKeyTransportVariants(t *testing.T) {
	resp, err := (APIKeyTransport{}).RoundTrip(nil)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected nil request error")
	}
	transport := APIKeyTransport{Key: "secret", HeaderName: "X-API-Key", Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("X-API-Key"); got != "secret" {
			t.Fatalf("X-API-Key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Header: http.Header{}}, nil
	})}
	resp, err = transport.RoundTrip(httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	_ = resp.Body.Close()
}

func TestWebhookSignerTransport(t *testing.T) {
	resp, err := (WebhookSignerTransport{}).RoundTrip(nil)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected nil request error")
	}
	resp, err = (WebhookSignerTransport{}).RoundTrip(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil))
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected missing signer error")
	}
	wantErr := errors.New("sign failed")
	transport := WebhookSignerTransport{Signer: signerFunc(func(context.Context, []byte) (string, error) {
		return "", wantErr
	})}
	resp, err = transport.RoundTrip(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("payload")))
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	transport = WebhookSignerTransport{
		HeaderName: "X-Hook-Signature",
		Signer: signerFunc(func(_ context.Context, body []byte) (string, error) {
			if string(body) != "payload" {
				t.Fatalf("signed body = %q", body)
			}
			return "signed", nil
		}),
		Base: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("X-Hook-Signature"); got != "signed" {
				t.Fatalf("signature = %q", got)
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != "payload" || r.ContentLength != int64(len("payload")) {
				t.Fatalf("body = %q contentLength=%d", body, r.ContentLength)
			}
			return &http.Response{StatusCode: http.StatusAccepted, Body: io.NopCloser(strings.NewReader("{}")), Header: http.Header{}}, nil
		}),
	}
	resp, err = transport.RoundTrip(httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader("payload")))
	if err != nil {
		t.Fatalf("RoundTrip() error = %v", err)
	}
	_ = resp.Body.Close()
}

func TestDoJSONSuccessAndFailurePaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Fatalf("Accept = %q", r.Header.Get("Accept"))
		}
		switch r.URL.Path {
		case "/ok":
			if r.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "widget") {
				t.Fatalf("request body = %q", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"widget_1"}`))
		case "/problem":
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"title":"Conflict","status":409,"detail":"already exists"}`))
		case "/bad-json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id"`))
		case "/bad-problem":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`not-json`))
		}
	}))
	defer server.Close()

	type response struct {
		ID string `json:"id"`
	}
	got, resp, err := DoJSON[response](context.Background(), server.Client(), http.MethodPost, server.URL+"/ok", map[string]string{"name": "widget"})
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err != nil || resp.StatusCode != http.StatusOK || got.ID != "widget_1" {
		t.Fatalf("DoJSON success = %#v, %v, %v", got, resp.StatusCode, err)
	}
	_, resp, err = DoJSON[response](context.Background(), server.Client(), http.MethodGet, server.URL+"/problem", nil)
	if resp != nil {
		_ = resp.Body.Close()
	}
	var problem *ProblemError
	if !errors.As(err, &problem) || problem.Problem.Detail != "already exists" {
		t.Fatalf("problem error = %T %v", err, err)
	}
	_, resp, err = DoJSON[response](context.Background(), server.Client(), http.MethodGet, server.URL+"/bad-json", nil)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected bad JSON response error")
	}
	_, resp, err = DoJSON[response](context.Background(), server.Client(), http.MethodGet, server.URL+"/bad-problem", nil)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected bad problem response error")
	}
	_, resp, err = DoJSON[response](context.Background(), server.Client(), http.MethodPost, server.URL+"/ok", make(chan int))
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected request marshal error")
	}
}
