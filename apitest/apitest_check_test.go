package apitest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/fielderrors"
	"github.com/aatuh/api-toolkit/v4/httpx"
)

func TestInternalCheckFunctionsCoverSuccessAndFailurePaths(t *testing.T) {
	problem := httptest.NewRecorder()
	problem.Header().Set("Content-Type", "application/problem+json")
	problem.WriteHeader(http.StatusBadRequest)
	_, _ = problem.WriteString(`{"code":"invalid_input","validation":{"fields":[{"field":"name"}]}}`)
	if _, err := checkProblem(problem, http.StatusBadRequest); err != nil {
		t.Fatalf("checkProblem() error = %v", err)
	}
	if err := checkProblemCode(problem, httpx.ProblemCode("invalid_input")); err != nil {
		t.Fatalf("checkProblemCode() error = %v", err)
	}
	if err := checkValidationFields(problem, "name"); err != nil {
		t.Fatalf("checkValidationFields() error = %v", err)
	}
	if _, err := checkProblem(problem, http.StatusUnauthorized); err == nil || !strings.Contains(err.Error(), "status") {
		t.Fatalf("expected status failure, got %v", err)
	}
	missingType := httptest.NewRecorder()
	missingType.WriteHeader(http.StatusBadRequest)
	_, _ = missingType.WriteString(`{}`)
	if _, err := checkProblem(missingType, http.StatusBadRequest); err == nil || !strings.Contains(err.Error(), "Content-Type") {
		t.Fatalf("expected content type failure, got %v", err)
	}
	badJSON := httptest.NewRecorder()
	badJSON.Header().Set("Content-Type", "application/problem+json")
	badJSON.WriteHeader(http.StatusBadRequest)
	_, _ = badJSON.WriteString(`{bad`)
	if _, err := checkProblem(badJSON, http.StatusBadRequest); err == nil {
		t.Fatal("expected decode failure")
	}
	if err := checkProblemCode(problem, httpx.ProblemCode("other")); err == nil {
		t.Fatal("expected problem code failure")
	}
	if err := checkValidationFields(problem, "missing"); err == nil {
		t.Fatal("expected validation field failure")
	}
}

func TestInternalJSONHeaderAndMetadataChecks(t *testing.T) {
	jsonRec := httptest.NewRecorder()
	_, _ = jsonRec.WriteString(`{"ok":true}`)
	if err := checkJSON(jsonRec, map[string]bool{"ok": true}); err != nil {
		t.Fatalf("checkJSON() error = %v", err)
	}
	if err := checkJSON(jsonRec, map[string]bool{"ok": false}); err == nil {
		t.Fatal("expected JSON mismatch")
	}
	badJSON := httptest.NewRecorder()
	_, _ = badJSON.WriteString(`{bad`)
	if err := checkJSON(badJSON, map[string]bool{}); err == nil {
		t.Fatal("expected JSON decode failure")
	}
	jsonRec.Header().Set("X-Test", "value")
	if err := checkHeader(jsonRec, "X-Test", "value"); err != nil {
		t.Fatalf("checkHeader() error = %v", err)
	}
	if err := checkHeader(jsonRec, "X-Test", "other"); err == nil {
		t.Fatal("expected header mismatch")
	}

	rate := httptest.NewRecorder()
	rate.Header().Set("RateLimit-Limit", "10")
	rate.Header().Set("RateLimit-Remaining", "9")
	rate.Header().Set("RateLimit-Reset", "60")
	if err := checkRateLimitHeaders(rate); err != nil {
		t.Fatalf("checkRateLimitHeaders() error = %v", err)
	}
	if err := checkRateLimitHeaders(httptest.NewRecorder()); err == nil {
		t.Fatal("expected missing rate limit header failure")
	}
	dep := httptest.NewRecorder()
	dep.Header().Set("Sunset", "Sun, 03 May 2026 12:00:00 GMT")
	if err := checkDeprecationHeaders(dep); err != nil {
		t.Fatalf("checkDeprecationHeaders() error = %v", err)
	}
	if err := checkDeprecationHeaders(httptest.NewRecorder()); err == nil {
		t.Fatal("expected deprecation header failure")
	}
}

func TestInternalPaginationOperationWebhookAndOpenAPIChecks(t *testing.T) {
	pagination := httptest.NewRecorder()
	_, _ = pagination.WriteString(`{"meta":{"count":2,"limit":10}}`)
	if err := checkPagination(pagination, 2, 10); err != nil {
		t.Fatalf("checkPagination() error = %v", err)
	}
	if err := checkPagination(pagination, 1, 10); err == nil {
		t.Fatal("expected pagination mismatch")
	}
	badPagination := httptest.NewRecorder()
	_, _ = badPagination.WriteString(`{bad`)
	if err := checkPagination(badPagination, 1, 10); err == nil {
		t.Fatal("expected pagination decode failure")
	}
	accepted := httptest.NewRecorder()
	accepted.Header().Set("Location", "/operations/1")
	accepted.WriteHeader(http.StatusAccepted)
	if err := checkOperationAccepted(accepted, "/operations/1"); err != nil {
		t.Fatalf("checkOperationAccepted() error = %v", err)
	}
	if err := checkOperationAccepted(accepted, "/operations/2"); err == nil {
		t.Fatal("expected location mismatch")
	}
	wrongStatus := httptest.NewRecorder()
	wrongStatus.WriteHeader(http.StatusOK)
	if err := checkOperationAccepted(wrongStatus, ""); err == nil {
		t.Fatal("expected accepted status mismatch")
	}
	if err := checkWebhookSignature(http.Header{"X-Signature": []string{"abc"}}, ""); err != nil {
		t.Fatalf("checkWebhookSignature() error = %v", err)
	}
	if err := checkWebhookSignature(http.Header{}, "X-Hook"); err == nil {
		t.Fatal("expected missing webhook signature failure")
	}
	if err := checkOpenAPIGolden([]byte(`{"b":2,"a":1}`), []byte(`{"a":1,"b":2}`)); err != nil {
		t.Fatalf("checkOpenAPIGolden() error = %v", err)
	}
	if err := checkOpenAPIGolden([]byte(`{bad`), []byte(`{}`)); err == nil {
		t.Fatal("expected invalid OpenAPI JSON failure")
	}
	if err := checkOpenAPIGolden([]byte(`{"a":1}`), []byte(`{"a":2}`)); err == nil {
		t.Fatal("expected OpenAPI golden mismatch")
	}
	if _, err := checkProblem(nil, http.StatusBadRequest); err == nil {
		t.Fatal("expected nil recorder failure")
	}
	if number("not-a-number") != 0 {
		t.Fatal("non-float JSON number helper should return zero")
	}
}

func TestPublicAssertionsStillDelegateToChecks(t *testing.T) {
	recorder := httptest.NewRecorder()
	problem := httpx.WithFieldErrors(httpx.Problem{Title: "Bad Request", Ext: map[string]any{"code": "invalid_input"}}, fielderrors.FieldErrors{{Field: "name", Code: "required", Message: "required"}})
	httpx.WriteProblem(recorder, http.StatusBadRequest, problem)
	AssertProblem(t, recorder, http.StatusBadRequest)
	AssertProblemCode(t, recorder, httpx.ProblemCode("invalid_input"))
	AssertValidationFields(t, recorder, "name")
}
