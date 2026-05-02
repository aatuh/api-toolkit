package binding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v2/fielderrors"
)

type createWidget struct {
	Name     string `json:"name" query:"name" required:"true"`
	Quantity int    `json:"quantity" query:"quantity" required:"true"`
	Active   bool   `json:"active" query:"active"`
}

func TestDecodeJSONValidatesShapeUnknownFieldsAndRequiredFields(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"name":"starter","quantity":2}`))
	got, err := DecodeJSON[createWidget](req, JSONConfig{})
	if err != nil {
		t.Fatalf("DecodeJSON() error = %v", err)
	}
	if got.Name != "starter" || got.Quantity != 2 {
		t.Fatalf("decoded = %#v", got)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`["not-object"]`))
	if _, err := DecodeJSON[createWidget](req, JSONConfig{}); !hasFieldError(err, "body", "invalid") {
		t.Fatalf("expected object error, got %v", err)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"extra":true}`))
	if _, err := DecodeJSON[createWidget](req, JSONConfig{}); !hasFieldError(err, "body", "invalid_json") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"name":"starter"}`))
	if _, err := DecodeJSON[createWidget](req, JSONConfig{AllowUnknownFields: true}); !hasFieldError(err, "quantity", "required") {
		t.Fatalf("expected required quantity error, got %v", err)
	}
}

func TestDecodeQueryConvertsScalarsAndRepeatedValues(t *testing.T) {
	type query struct {
		Limit int      `query:"limit" required:"true"`
		Tags  []string `query:"tag"`
		Admin bool     `query:"admin"`
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?limit=10&tag=a&tag=b&admin=true", nil)
	got, err := DecodeQuery[query](req, QueryConfig{})
	if err != nil {
		t.Fatalf("DecodeQuery() error = %v", err)
	}
	if got.Limit != 10 || got.Admin != true || len(got.Tags) != 2 || got.Tags[1] != "b" {
		t.Fatalf("decoded = %#v", got)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?limit=nope", nil)
	if _, err := DecodeQuery[query](req, QueryConfig{}); !hasFieldError(err, "limit", "invalid") {
		t.Fatalf("expected invalid limit error, got %v", err)
	}
}

func TestDecodePathUsesConfiguredResolver(t *testing.T) {
	type path struct {
		ID int `path:"id" required:"true"`
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets/42", nil)
	got, err := DecodePath[path](req, PathConfig{Param: func(r *http.Request, name string) string {
		if name == "id" {
			return "42"
		}
		return ""
	}})
	if err != nil {
		t.Fatalf("DecodePath() error = %v", err)
	}
	if got.ID != 42 {
		t.Fatalf("ID = %d, want 42", got.ID)
	}

	if _, err := DecodePath[path](req, PathConfig{}); !hasFieldError(err, "path", "missing_param_resolver") {
		t.Fatalf("expected missing resolver error, got %v", err)
	}
}

func TestWriteValidationProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteValidationProblem(rec, fielderrors.FieldErrors{{Field: "name", Code: "required", Message: "name is required"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"validation"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func hasFieldError(err error, field, code string) bool {
	provider, ok := err.(fielderrors.Provider)
	if !ok {
		return false
	}
	for _, entry := range provider.FieldErrors() {
		if entry.Field == field && entry.Code == code {
			return true
		}
	}
	return false
}
