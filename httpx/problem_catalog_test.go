package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
)

func TestProblemCatalogMapsCodeToProblem(t *testing.T) {
	catalog := NewProblemCatalog(ProblemDefinition{
		Code:             "widget_conflict",
		Status:           http.StatusConflict,
		Type:             "https://example.test/problems/widget-conflict",
		Title:            "Widget conflict",
		Detail:           "widget is stale",
		Retryable:        true,
		DocumentationURL: "https://docs.example.test/widgets/conflicts",
		LogLevel:         "warn",
	})
	problem, status := catalog.Problem("widget_conflict", "")
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if problem.Type != "https://example.test/problems/widget-conflict" || problem.Title != "Widget conflict" {
		t.Fatalf("problem = %#v", problem)
	}
	if problem.Ext["code"] != "widget_conflict" || problem.Ext["retryable"] != true {
		t.Fatalf("extensions = %#v", problem.Ext)
	}
}

func TestProblemFromErrorWithCatalog(t *testing.T) {
	catalog := NewProblemCatalog(ProblemDefinition{
		Code:   "teapot",
		Status: http.StatusTeapot,
		Title:  "Teapot",
		Detail: "short and stout",
	})
	err := NewProblemError("teapot", "custom detail", errors.New("wrapped"))
	problem, status := ProblemFromErrorWithCatalog(err, catalog, ErrorOptions{})
	if status != http.StatusTeapot || problem.Detail != "custom detail" {
		t.Fatalf("problem = %#v, status = %d", problem, status)
	}

	fieldErr := fielderrors.FieldErrors{{Field: "name", Code: "required", Message: "name is required"}}
	problem, status = ProblemFromErrorWithCatalog(fieldErr, nil, ErrorOptions{})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	if problem.Ext["validation"] == nil {
		t.Fatalf("expected validation extension, got %#v", problem.Ext)
	}
}

func TestWriteProblemCodeAndBackwardCompatibleMapping(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblemCode(rec, ProblemCode(TypeNotFound))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not-found"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}

	problem, status := ProblemFromErrorWithCatalog(ErrForbidden, nil, ErrorOptions{})
	if status != http.StatusForbidden || problem.Type != DefaultTypeURI(TypeForbidden) {
		t.Fatalf("problem = %#v, status = %d", problem, status)
	}
}
