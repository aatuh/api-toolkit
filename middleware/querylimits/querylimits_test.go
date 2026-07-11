package querylimits

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

func TestQueryLimitsUsesValidationType(t *testing.T) {
	mw, err := New(Options{MaxParams: 1})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?a=1&b=2", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 status, got %d", rec.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	gotType, _ := body["type"].(string)
	wantType := httpx.DefaultTypeURI(httpx.TypeValidation)
	if gotType != wantType {
		t.Fatalf("expected type %q, got %q", wantType, gotType)
	}
}

func TestQueryLimitsRejectsNegativeValues(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "max params", opts: Options{MaxParams: -1}},
		{name: "max key length", opts: Options{MaxKeyLength: -1}},
		{name: "max value length", opts: Options{MaxValueLength: -1}},
		{name: "max limit", opts: Options{MaxLimit: -1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := New(tc.opts); err == nil {
				t.Fatal("expected error for negative option")
			}
		})
	}
}

func TestQueryLimitsRejectsLongKeyValueAndInvalidLimits(t *testing.T) {
	tests := []struct {
		name       string
		opts       Options
		target     string
		wantDetail string
	}{
		{
			name:       "long key",
			opts:       Options{MaxKeyLength: 3},
			target:     "/?toolong=1",
			wantDetail: "query parameter key too long",
		},
		{
			name:       "long value",
			opts:       Options{MaxValueLength: 3},
			target:     "/?a=toolong",
			wantDetail: "query parameter value too long",
		},
		{
			name:       "invalid limit",
			opts:       Options{},
			target:     "/?limit=abc",
			wantDetail: "invalid pagination limit",
		},
		{
			name:       "zero limit",
			opts:       Options{},
			target:     "/?limit=0",
			wantDetail: "invalid pagination limit",
		},
		{
			name:       "limit exceeds maximum",
			opts:       Options{MaxLimit: 10},
			target:     "/?limit=11",
			wantDetail: "pagination limit exceeds maximum",
		},
		{
			name:       "custom limit param",
			opts:       Options{LimitParam: "page_size", MaxLimit: 10},
			target:     "/?page_size=11",
			wantDetail: "pagination limit exceeds maximum",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mw, err := New(tc.opts)
			if err != nil {
				t.Fatalf("new middleware: %v", err)
			}
			handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.target, nil)
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.wantDetail) {
				t.Fatalf("expected detail %q in body %q", tc.wantDetail, rec.Body.String())
			}
		})
	}
}

func TestQueryLimitsCustomErrorWriterAndNilMiddleware(t *testing.T) {
	var capturedStatus int
	var capturedProblem httpx.Problem
	mw, err := New(Options{
		MaxParams: 1,
		ErrorWriter: func(w http.ResponseWriter, status int, p httpx.Problem) {
			capturedStatus = status
			capturedProblem = p
			w.WriteHeader(status)
		},
	})
	if err != nil {
		t.Fatalf("new middleware: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?a=1&b=2", nil)
	mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, req)

	if capturedStatus != http.StatusBadRequest {
		t.Fatalf("captured status = %d, want 400", capturedStatus)
	}
	if capturedProblem.Detail != "too many query parameters" {
		t.Fatalf("captured problem detail = %q", capturedProblem.Detail)
	}

	var nilMW *Middleware
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?a=1&b=2", nil)
	nilMW.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})).ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("nil middleware code = %d, want 202", rec.Code)
	}
}
