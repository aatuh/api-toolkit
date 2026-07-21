package binding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/fielderrors"
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

func TestRequiredModePresentAcceptsExplicitZeroValues(t *testing.T) {
	type request struct {
		Enabled bool `json:"enabled" query:"enabled" path:"enabled" required:"true"`
		Limit   int  `json:"limit" query:"limit" path:"limit" required:"true"`
	}

	jsonRequest := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"enabled":false,"limit":0}`))
	if _, err := DecodeJSON[request](jsonRequest, JSONConfig{}); !hasFieldError(err, "enabled", "required") {
		t.Fatalf("default mode must retain non-zero behavior, got %v", err)
	}
	jsonRequest = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"enabled":false,"limit":0}`))
	got, err := DecodeJSON[request](jsonRequest, JSONConfig{RequiredMode: RequiredModePresent})
	if err != nil || got.Enabled || got.Limit != 0 {
		t.Fatalf("DecodeJSON() = %#v, %v", got, err)
	}

	queryRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?enabled=false&limit=0", nil)
	got, err = DecodeQuery[request](queryRequest, QueryConfig{RequiredMode: RequiredModePresent})
	if err != nil || got.Enabled || got.Limit != 0 {
		t.Fatalf("DecodeQuery() = %#v, %v", got, err)
	}

	pathRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets/0", nil)
	got, err = DecodePath[request](pathRequest, PathConfig{
		RequiredMode: RequiredModePresent,
		Param: func(_ *http.Request, name string) string {
			mapValues := map[string]string{"enabled": "false", "limit": "0"}
			return mapValues[name]
		},
		ParamPresent: func(_ *http.Request, name string) bool {
			return name == "enabled" || name == "limit"
		},
	})
	if err != nil || got.Enabled || got.Limit != 0 {
		t.Fatalf("DecodePath() = %#v, %v", got, err)
	}
}

func TestRequiredModePresentRejectsMissingButAcceptsJSONNull(t *testing.T) {
	type request struct {
		Name string `json:"name" required:"true"`
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{}`))
	if _, err := DecodeJSON[request](req, JSONConfig{RequiredMode: RequiredModePresent}); !hasFieldError(err, "name", "required") {
		t.Fatalf("missing JSON field error = %v", err)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"name":null}`))
	got, err := DecodeJSON[request](req, JSONConfig{RequiredMode: RequiredModePresent})
	if err != nil || got.Name != "" {
		t.Fatalf("DecodeJSON(null) = %#v, %v", got, err)
	}
}

func TestRequiredModePresentRejectsDuplicateJSONMembers(t *testing.T) {
	type request struct {
		Name string `json:"name" required:"true"`
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"name":"first","name":"second"}`))
	if _, err := DecodeJSON[request](req, JSONConfig{RequiredMode: RequiredModePresent}); !hasFieldError(err, "body", "invalid_json") {
		t.Fatalf("duplicate JSON member error = %v", err)
	}
}

func TestRequiredModePresentAcceptsAllPresentJSONValues(t *testing.T) {
	type request struct {
		Name       string            `json:"name" required:"true"`
		Tags       []string          `json:"tags" required:"true"`
		Attributes map[string]string `json:"attributes" required:"true"`
		Count      *int              `json:"count" required:"true"`
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{
			name: "empty values and pointer to zero",
			body: `{"name":"","tags":[],"attributes":{},"count":0}`,
		},
		{
			name: "null values",
			body: `{"name":null,"tags":null,"attributes":null,"count":null}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(tc.body))
			if _, err := DecodeJSON[request](req, JSONConfig{RequiredMode: RequiredModePresent}); err != nil {
				t.Fatalf("DecodeJSON() error = %v", err)
			}
		})
	}
}

func TestRequiredModePresentAcceptsEmptyAndRepeatedTransportValues(t *testing.T) {
	type queryRequest struct {
		Name string   `query:"name" required:"true"`
		Tags []string `query:"tag" required:"true"`
	}
	query := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?name=&tag=&tag=second", nil)
	got, err := DecodeQuery[queryRequest](query, QueryConfig{RequiredMode: RequiredModePresent})
	if err != nil {
		t.Fatalf("DecodeQuery() error = %v", err)
	}
	if got.Name != "" || len(got.Tags) != 1 || got.Tags[0] != "second" {
		t.Fatalf("DecodeQuery() = %#v", got)
	}

	type pathRequest struct {
		Name string `path:"name" required:"true"`
	}
	path := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/widgets/", nil)
	gotPath, err := DecodePath[pathRequest](path, PathConfig{
		RequiredMode: RequiredModePresent,
		Param: func(_ *http.Request, _ string) string {
			return ""
		},
		ParamPresent: func(_ *http.Request, name string) bool {
			return name == "name"
		},
	})
	if err != nil || gotPath.Name != "" {
		t.Fatalf("DecodePath() = %#v, %v", gotPath, err)
	}
}

func TestRequiredModePresentRejectsUnknownJSONFields(t *testing.T) {
	type request struct {
		Name string `json:"name" required:"true"`
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"name":"ok","extra":true}`))
	if _, err := DecodeJSON[request](req, JSONConfig{RequiredMode: RequiredModePresent}); !hasFieldError(err, "body", "invalid_json") {
		t.Fatalf("unknown field error = %v", err)
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
