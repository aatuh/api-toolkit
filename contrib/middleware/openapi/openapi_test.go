package openapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

func TestProblemForOpenAPIRequestError(t *testing.T) {
	err := &openapi3filter.RequestError{
		Parameter: &openapi3.Parameter{
			Name: "limit",
			In:   "query",
		},
		Reason: openapi3filter.ErrInvalidRequired.Error(),
		Err:    openapi3filter.ErrInvalidRequired,
	}
	p := problemForOpenAPIError(http.StatusBadRequest, err)
	if p.Type != httpx.DefaultTypeURI(httpx.TypeValidation) {
		t.Fatalf("expected validation type, got %q", p.Type)
	}
	val, ok := p.Ext[httpx.ValidationErrorsKey].(httpx.ValidationErrors)
	if !ok {
		t.Fatalf("expected validation extension")
	}
	if len(val.Fields) != 1 {
		t.Fatalf("expected 1 field error, got %d", len(val.Fields))
	}
	if val.Fields[0].Field != "query.limit" {
		t.Fatalf("expected field query.limit, got %q", val.Fields[0].Field)
	}
	if val.Fields[0].Code != "required" {
		t.Fatalf("expected code required, got %q", val.Fields[0].Code)
	}
}

func TestProblemForOpenAPISecurityError(t *testing.T) {
	err := &openapi3filter.SecurityRequirementsError{}
	p := problemForOpenAPIError(http.StatusUnauthorized, err)
	if p.Type != httpx.DefaultTypeURI(httpx.TypeUnauthorized) {
		t.Fatalf("expected unauthorized type, got %q", p.Type)
	}
}

func TestResponseValidation(t *testing.T) {
	spec := buildPingSpec()
	mw, err := New(spec, WithResponseValidation(ResponseValidationOptions{
		Enabled:      true,
		MaxBodyBytes: 1 << 20,
	}))
	if err != nil {
		t.Fatalf("middleware error: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"bad": true})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["type"] != httpx.DefaultTypeURI(httpx.TypeInternal) {
		t.Fatalf("expected internal type, got %v", body["type"])
	}
}

func TestResponseValidationBuffersResponsesWithoutOptionalInterfaces(t *testing.T) {
	spec := buildPingSpec()
	mw, err := New(spec, WithResponseValidation(ResponseValidationOptions{
		Enabled:      true,
		MaxBodyBytes: 1 << 20,
	}))
	if err != nil {
		t.Fatalf("middleware error: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setOptionalInterfaceHeaders(w)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	assertOptionalInterfaceHeadersFalse(t, rec.Header())
}

func buildPingSpec() *openapi3.T {
	schema := openapi3.NewObjectSchema()
	schema.Required = []string{"ok"}
	schema.Properties = map[string]*openapi3.SchemaRef{
		"ok": {Value: openapi3.NewBoolSchema()},
	}
	resp := openapi3.NewResponse().WithDescription("ok").WithJSONSchema(schema)
	responses := openapi3.NewResponses(
		openapi3.WithStatus(http.StatusOK, &openapi3.ResponseRef{Value: resp}),
	)
	return &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Ping",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(
			openapi3.WithPath("/ping", &openapi3.PathItem{
				Get: &openapi3.Operation{
					OperationID: "Ping",
					Responses:   responses,
				},
			}),
		),
	}
}

func setOptionalInterfaceHeaders(w http.ResponseWriter) {
	_, flusher := w.(http.Flusher)
	_, hijacker := w.(http.Hijacker)
	_, pusher := w.(http.Pusher)
	_, readerFrom := w.(io.ReaderFrom)
	w.Header().Set("X-Has-Flusher", strconv.FormatBool(flusher))
	w.Header().Set("X-Has-Hijacker", strconv.FormatBool(hijacker))
	w.Header().Set("X-Has-Pusher", strconv.FormatBool(pusher))
	w.Header().Set("X-Has-ReaderFrom", strconv.FormatBool(readerFrom))
}

func assertOptionalInterfaceHeadersFalse(t *testing.T, header http.Header) {
	t.Helper()
	if got := header.Get("X-Has-Flusher"); got != "false" {
		t.Fatalf("expected buffered writer without flusher, got %q", got)
	}
	if got := header.Get("X-Has-Hijacker"); got != "false" {
		t.Fatalf("expected buffered writer without hijacker, got %q", got)
	}
	if got := header.Get("X-Has-Pusher"); got != "false" {
		t.Fatalf("expected buffered writer without pusher, got %q", got)
	}
	if got := header.Get("X-Has-ReaderFrom"); got != "false" {
		t.Fatalf("expected buffered writer without readerFrom, got %q", got)
	}
}
