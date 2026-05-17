package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/webhooks"
)

// ProblemError wraps a Problem Details response as an error.
type ProblemError struct {
	StatusCode int
	Problem    httpx.Problem
}

func (err *ProblemError) Error() string {
	if err == nil {
		return "api problem"
	}
	if err.Problem.Detail != "" {
		return err.Problem.Detail
	}
	if err.Problem.Title != "" {
		return err.Problem.Title
	}
	return http.StatusText(err.StatusCode)
}

// DecodeProblem decodes a Problem Details response.
func DecodeProblem(resp *http.Response) (*ProblemError, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is required")
	}
	defer resp.Body.Close()
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	problem := httpx.Problem{Ext: map[string]any{}}
	if value, ok := raw["type"].(string); ok {
		problem.Type = value
	}
	if value, ok := raw["title"].(string); ok {
		problem.Title = value
	}
	if value, ok := raw["status"].(float64); ok {
		problem.Status = int(value)
	}
	if value, ok := raw["detail"].(string); ok {
		problem.Detail = value
	}
	if value, ok := raw["instance"].(string); ok {
		problem.Instance = value
	}
	for key, value := range raw {
		switch key {
		case "type", "title", "status", "detail", "instance":
		default:
			problem.Ext[key] = value
		}
	}
	return &ProblemError{StatusCode: resp.StatusCode, Problem: problem}, nil
}

// CursorPage is one cursor-paginated client page.
type CursorPage[T any] struct {
	Items      []T
	NextCursor string
}

// CursorPageFunc fetches one cursor-paginated page.
type CursorPageFunc[T any] func(context.Context, string) (CursorPage[T], error)

// CursorIterator iterates cursor-paginated resources.
type CursorIterator[T any] struct {
	Fetch  CursorPageFunc[T]
	Cursor string
	items  []T
	done   bool
	err    error
}

// Next fetches the next page.
func (it *CursorIterator[T]) Next(ctx context.Context) bool {
	if it == nil || it.done || it.Fetch == nil {
		return false
	}
	page, err := it.Fetch(ctx, it.Cursor)
	if err != nil {
		it.err = err
		it.done = true
		return false
	}
	it.items = page.Items
	it.Cursor = page.NextCursor
	if it.Cursor == "" {
		it.done = true
	}
	return true
}

// Items returns the current page items.
func (it *CursorIterator[T]) Items() []T {
	if it == nil {
		return nil
	}
	return it.items
}

// Err returns the terminal iterator error.
func (it *CursorIterator[T]) Err() error {
	if it == nil {
		return nil
	}
	return it.err
}

// RetryAfter parses a Retry-After header value.
func RetryAfter(header http.Header) (time.Duration, bool) {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, true
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	duration := time.Until(when)
	if duration < 0 {
		duration = 0
	}
	return duration, true
}

// PreconditionHeaders builds conditional request headers.
func PreconditionHeaders(etag string, lastModified time.Time) http.Header {
	header := http.Header{}
	if strings.TrimSpace(etag) != "" {
		header.Set("If-Match", strings.TrimSpace(etag))
	}
	if !lastModified.IsZero() {
		header.Set("If-Unmodified-Since", lastModified.UTC().Format(http.TimeFormat))
	}
	return header
}

// APIKeyTransport adds API key credentials to outbound requests.
type APIKeyTransport struct {
	Key        string
	HeaderName string
	Scheme     string
	Base       http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (transport APIKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	clone := req.Clone(req.Context())
	headerName := strings.TrimSpace(transport.HeaderName)
	if headerName == "" {
		headerName = "Authorization"
	}
	if headerName == "Authorization" {
		scheme := strings.TrimSpace(transport.Scheme)
		if scheme == "" {
			scheme = "ApiKey"
		}
		clone.Header.Set("Authorization", scheme+" "+transport.Key)
	} else {
		clone.Header.Set(headerName, transport.Key)
	}
	return baseRoundTripper(transport.Base).RoundTrip(clone)
}

// WebhookSignerTransport signs outbound request bodies.
type WebhookSignerTransport struct {
	Signer     webhooks.Signer
	HeaderName string
	Base       http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (transport WebhookSignerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if transport.Signer == nil {
		return nil, fmt.Errorf("webhook signer is required")
	}
	body, err := readRequestBody(req)
	if err != nil {
		return nil, err
	}
	signature, err := transport.Signer.SignWebhook(req.Context(), body)
	if err != nil {
		return nil, err
	}
	clone := req.Clone(req.Context())
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	headerName := strings.TrimSpace(transport.HeaderName)
	if headerName == "" {
		headerName = "X-Signature"
	}
	clone.Header.Set(headerName, signature)
	return baseRoundTripper(transport.Base).RoundTrip(clone)
}

// DoJSON sends a JSON request and decodes a JSON response.
func DoJSON[T any](ctx context.Context, client *http.Client, method, url string, requestBody any) (T, *http.Response, error) {
	var out T
	var body io.Reader
	if requestBody != nil {
		data, err := json.Marshal(requestBody)
		if err != nil {
			return out, nil, err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return out, nil, err
	}
	request.Header.Set("Accept", "application/json")
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(request)
	if err != nil {
		return out, resp, err
	}
	if resp.StatusCode >= 400 {
		problem, err := DecodeProblem(resp)
		if err != nil {
			return out, resp, err
		}
		return out, resp, problem
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, resp, err
	}
	return out, resp, nil
}

func baseRoundTripper(base http.RoundTripper) http.RoundTripper {
	if base != nil {
		return base
	}
	return http.DefaultTransport
}

func readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
