package apitest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/contracttest"
	"github.com/aatuh/api-toolkit/v4/httpx"
)

// AssertProblem asserts a Problem Details response and returns the decoded body.
func AssertProblem(t testing.TB, recorder *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	body, err := checkProblem(recorder, status)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// AssertProblemCode asserts a typed problem code extension.
func AssertProblemCode(t testing.TB, recorder *httptest.ResponseRecorder, code httpx.ProblemCode) {
	t.Helper()
	if err := checkProblemCode(recorder, code); err != nil {
		t.Fatal(err)
	}
}

// AssertValidationFields asserts validation field names in a Problem Details response.
func AssertValidationFields(t testing.TB, recorder *httptest.ResponseRecorder, fields ...string) {
	t.Helper()
	if err := checkValidationFields(recorder, fields...); err != nil {
		t.Fatal(err)
	}
}

// AssertJSON asserts a JSON response body.
func AssertJSON(t testing.TB, recorder *httptest.ResponseRecorder, want any) {
	t.Helper()
	if err := checkJSON(recorder, want); err != nil {
		t.Fatal(err)
	}
}

// AssertHeader asserts a response header value.
func AssertHeader(t testing.TB, recorder *httptest.ResponseRecorder, name, want string) {
	t.Helper()
	if err := checkHeader(recorder, name, want); err != nil {
		t.Fatal(err)
	}
}

// AssertRateLimitHeaders asserts standard quota headers exist.
func AssertRateLimitHeaders(t testing.TB, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if err := checkRateLimitHeaders(recorder); err != nil {
		t.Fatal(err)
	}
}

// AssertETag asserts the ETag response header.
func AssertETag(t testing.TB, recorder *httptest.ResponseRecorder, want string) {
	AssertHeader(t, recorder, "ETag", want)
}

// AssertDeprecationHeaders asserts runtime deprecation headers.
func AssertDeprecationHeaders(t testing.TB, recorder *httptest.ResponseRecorder) {
	t.Helper()
	if err := checkDeprecationHeaders(recorder); err != nil {
		t.Fatal(err)
	}
}

// AssertPagination asserts common list metadata values.
func AssertPagination(t testing.TB, recorder *httptest.ResponseRecorder, count, limit int) {
	t.Helper()
	if err := checkPagination(recorder, count, limit); err != nil {
		t.Fatal(err)
	}
}

// AssertOperationAccepted asserts a 202 operation response.
func AssertOperationAccepted(t testing.TB, recorder *httptest.ResponseRecorder, location string) {
	t.Helper()
	if err := checkOperationAccepted(recorder, location); err != nil {
		t.Fatal(err)
	}
}

// AssertWebhookSignature asserts that a signature header is present.
func AssertWebhookSignature(t testing.TB, header http.Header, name string) {
	t.Helper()
	if err := checkWebhookSignature(header, name); err != nil {
		t.Fatal(err)
	}
}

// AssertOpenAPIGolden asserts normalized OpenAPI JSON against a golden document.
func AssertOpenAPIGolden(t testing.TB, got, golden []byte) {
	t.Helper()
	if err := checkOpenAPIGolden(got, golden); err != nil {
		t.Fatal(err)
	}
}

func checkProblem(recorder *httptest.ResponseRecorder, status int) (map[string]any, error) {
	if recorder == nil {
		return nil, errors.New("recorder is required")
	}
	if recorder.Code != status {
		return nil, fmt.Errorf("status = %d, want %d", recorder.Code, status)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "application/problem+json") {
		return nil, fmt.Errorf("Content-Type = %q, want application/problem+json", contentType)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		return nil, fmt.Errorf("decode problem: %w", err)
	}
	return body, nil
}

func checkProblemCode(recorder *httptest.ResponseRecorder, code httpx.ProblemCode) error {
	body, err := checkProblem(recorder, recorder.Code)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(body["code"]); got != string(code) {
		return fmt.Errorf("problem code = %q, want %q", got, code)
	}
	return nil
}

func checkValidationFields(recorder *httptest.ResponseRecorder, fields ...string) error {
	body, err := checkProblem(recorder, recorder.Code)
	if err != nil {
		return err
	}
	present := map[string]bool{}
	collectValidationFields(body["validation"], present)
	collectValidationFields(body["errors"], present)
	for _, field := range fields {
		if !present[field] {
			return fmt.Errorf("validation field %q missing in %#v", field, body)
		}
	}
	return nil
}

func checkJSON(recorder *httptest.ResponseRecorder, want any) error {
	var got any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	wantBytes, err := json.Marshal(want)
	if err != nil {
		return fmt.Errorf("marshal expected JSON: %w", err)
	}
	var normalizedWant any
	if err := json.Unmarshal(wantBytes, &normalizedWant); err != nil {
		return fmt.Errorf("normalize expected JSON: %w", err)
	}
	if !reflect.DeepEqual(got, normalizedWant) {
		return fmt.Errorf("JSON = %#v, want %#v", got, normalizedWant)
	}
	return nil
}

func checkHeader(recorder *httptest.ResponseRecorder, name, want string) error {
	if got := recorder.Header().Get(name); got != want {
		return fmt.Errorf("header %s = %q, want %q", name, got, want)
	}
	return nil
}

func checkRateLimitHeaders(recorder *httptest.ResponseRecorder) error {
	for _, name := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset"} {
		if recorder.Header().Get(name) == "" {
			return fmt.Errorf("header %s is missing", name)
		}
	}
	return nil
}

func checkDeprecationHeaders(recorder *httptest.ResponseRecorder) error {
	if recorder.Header().Get("Deprecation") == "" && recorder.Header().Get("Sunset") == "" {
		return errors.New("deprecation headers are missing")
	}
	return nil
}

func checkPagination(recorder *httptest.ResponseRecorder, count, limit int) error {
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		return fmt.Errorf("decode paginated response: %w", err)
	}
	meta, _ := body["meta"].(map[string]any)
	if int(number(meta["count"])) != count || int(number(meta["limit"])) != limit {
		return fmt.Errorf("pagination meta = %#v, want count=%d limit=%d", meta, count, limit)
	}
	return nil
}

func checkOperationAccepted(recorder *httptest.ResponseRecorder, location string) error {
	if recorder.Code != http.StatusAccepted {
		return fmt.Errorf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
	if location != "" && recorder.Header().Get("Location") != location {
		return fmt.Errorf("location = %q, want %q", recorder.Header().Get("Location"), location)
	}
	return nil
}

func checkWebhookSignature(header http.Header, name string) error {
	if strings.TrimSpace(name) == "" {
		name = "X-Signature"
	}
	if header.Get(name) == "" {
		return fmt.Errorf("webhook signature header %s is missing", name)
	}
	return nil
}

func checkOpenAPIGolden(got, golden []byte) error {
	normalizedGot, err := contracttest.NormalizeOpenAPI(got)
	if err != nil {
		return fmt.Errorf("normalize OpenAPI: %w", err)
	}
	normalizedGolden, err := contracttest.NormalizeOpenAPI(golden)
	if err != nil {
		return fmt.Errorf("normalize golden OpenAPI: %w", err)
	}
	if !bytes.Equal(normalizedGot, normalizedGolden) {
		return fmt.Errorf("OpenAPI golden mismatch\n got: %s\nwant: %s", normalizedGot, normalizedGolden)
	}
	return nil
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
