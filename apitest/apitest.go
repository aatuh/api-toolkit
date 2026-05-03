// Package apitest provides deterministic HTTP response assertions for API tests.
package apitest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/contracttest"
	"github.com/aatuh/api-toolkit/v2/httpx"
)

// AssertProblem asserts a Problem Details response and returns the decoded body.
func AssertProblem(t testing.TB, recorder *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d", recorder.Code, status)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/problem+json") {
		t.Fatalf("Content-Type = %q, want application/problem+json", contentType)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	return body
}

// AssertProblemCode asserts a typed problem code extension.
func AssertProblemCode(t testing.TB, recorder *httptest.ResponseRecorder, code httpx.ProblemCode) {
	t.Helper()
	body := AssertProblem(t, recorder, recorder.Code)
	if got := fmt.Sprint(body["code"]); got != string(code) {
		t.Fatalf("problem code = %q, want %q", got, code)
	}
}

// AssertValidationFields asserts validation field names in a Problem Details response.
func AssertValidationFields(t testing.TB, recorder *httptest.ResponseRecorder, fields ...string) {
	t.Helper()
	body := AssertProblem(t, recorder, recorder.Code)
	present := map[string]bool{}
	collectValidationFields(body["validation"], present)
	collectValidationFields(body["errors"], present)
	for _, field := range fields {
		if !present[field] {
			t.Fatalf("validation field %q missing in %#v", field, body)
		}
	}
}

// AssertJSON asserts a JSON response body.
func AssertJSON(t testing.TB, recorder *httptest.ResponseRecorder, want any) {
	t.Helper()
	var got any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	wantBytes, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal expected JSON: %v", err)
	}
	var normalizedWant any
	if err := json.Unmarshal(wantBytes, &normalizedWant); err != nil {
		t.Fatalf("normalize expected JSON: %v", err)
	}
	if !reflect.DeepEqual(got, normalizedWant) {
		t.Fatalf("JSON = %#v, want %#v", got, normalizedWant)
	}
}

// AssertHeader asserts a response header value.
func AssertHeader(t testing.TB, recorder *httptest.ResponseRecorder, name, want string) {
	t.Helper()
	if got := recorder.Header().Get(name); got != want {
		t.Fatalf("header %s = %q, want %q", name, got, want)
	}
}

// AssertRateLimitHeaders asserts standard quota headers exist.
func AssertRateLimitHeaders(t testing.TB, recorder *httptest.ResponseRecorder) {
	t.Helper()
	for _, name := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset"} {
		if recorder.Header().Get(name) == "" {
			t.Fatalf("header %s is missing", name)
		}
	}
}

// AssertETag asserts the ETag response header.
func AssertETag(t testing.TB, recorder *httptest.ResponseRecorder, want string) {
	AssertHeader(t, recorder, "ETag", want)
}

// AssertDeprecationHeaders asserts runtime deprecation headers.
func AssertDeprecationHeaders(t testing.TB, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if recorder.Header().Get("Deprecation") == "" && recorder.Header().Get("Sunset") == "" {
		t.Fatalf("deprecation headers are missing")
	}
}

// AssertPagination asserts common list metadata values.
func AssertPagination(t testing.TB, recorder *httptest.ResponseRecorder, count, limit int) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode paginated response: %v", err)
	}
	meta, _ := body["meta"].(map[string]any)
	if int(number(meta["count"])) != count || int(number(meta["limit"])) != limit {
		t.Fatalf("pagination meta = %#v, want count=%d limit=%d", meta, count, limit)
	}
}

// AssertOperationAccepted asserts a 202 operation response.
func AssertOperationAccepted(t testing.TB, recorder *httptest.ResponseRecorder, location string) {
	t.Helper()
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if location != "" && recorder.Header().Get("Location") != location {
		t.Fatalf("Location = %q, want %q", recorder.Header().Get("Location"), location)
	}
}

// AssertWebhookSignature asserts that a signature header is present.
func AssertWebhookSignature(t testing.TB, header http.Header, name string) {
	t.Helper()
	if strings.TrimSpace(name) == "" {
		name = "X-Signature"
	}
	if header.Get(name) == "" {
		t.Fatalf("webhook signature header %s is missing", name)
	}
}

// AssertOpenAPIGolden asserts normalized OpenAPI JSON against a golden document.
func AssertOpenAPIGolden(t testing.TB, got, golden []byte) {
	t.Helper()
	normalizedGot, err := contracttest.NormalizeOpenAPI(got)
	if err != nil {
		t.Fatalf("normalize OpenAPI: %v", err)
	}
	normalizedGolden, err := contracttest.NormalizeOpenAPI(golden)
	if err != nil {
		t.Fatalf("normalize golden OpenAPI: %v", err)
	}
	if !bytes.Equal(normalizedGot, normalizedGolden) {
		t.Fatalf("OpenAPI golden mismatch\n got: %s\nwant: %s", normalizedGot, normalizedGolden)
	}
}

func collectValidationFields(value any, out map[string]bool) {
	switch typed := value.(type) {
	case map[string]any:
		collectValidationFields(typed["fields"], out)
	case []any:
		for _, item := range typed {
			if entry, ok := item.(map[string]any); ok {
				if field, ok := entry["field"].(string); ok {
					out[field] = true
				}
			}
		}
	}
}

func number(value any) float64 {
	if n, ok := value.(float64); ok {
		return n
	}
	return 0
}
