package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func FuzzRequestAndResponseValidation(f *testing.F) {
	f.Add(`{"name":"widget"}`, `{"id":"widget-1","name":"widget"}`)
	f.Add(`{"name":123}`, `{"id":false}`)
	f.Add(`{`, `{"id":"widget-1","name":"widget"}`)

	middleware, err := New(buildCreateWidgetSpec(), WithResponseValidation(ResponseValidationOptions{
		Enabled:      true,
		MaxBodyBytes: 4096,
	}))
	if err != nil {
		f.Fatalf("new OpenAPI middleware: %v", err)
	}

	f.Fuzz(func(t *testing.T, requestBody, responseBody string) {
		requestBody = limitOpenAPIFuzzString(requestBody, 4096)
		responseBody = limitOpenAPIFuzzString(responseBody, 4096)
		handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(responseBody))
		}))

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code < http.StatusContinue || rec.Code > 599 {
			t.Fatalf("OpenAPI validation wrote invalid status %d", rec.Code)
		}
		if rec.Code >= http.StatusInternalServerError && !strings.Contains(rec.Header().Get("Content-Type"), "application/problem+json") {
			t.Fatalf("OpenAPI validation failure must use Problem Details, got %q", rec.Header().Get("Content-Type"))
		}
	})
}

func limitOpenAPIFuzzString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
