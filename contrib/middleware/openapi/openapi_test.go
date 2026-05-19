package openapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"

	"github.com/aatuh/api-toolkit/v3/httpx"
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

func TestRequestValidationRejectsMissingRequiredQueryParameter(t *testing.T) {
	spec := buildSearchSpec()
	mw, err := New(spec)
	if err != nil {
		t.Fatalf("middleware error: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run for invalid request")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/search", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["type"] != httpx.DefaultTypeURI(httpx.TypeValidation) {
		t.Fatalf("problem type = %q, want validation", body["type"])
	}
	validation, ok := body[httpx.ValidationErrorsKey].(map[string]any)
	if !ok {
		t.Fatalf("validation extension = %#v, want 1 field error", body[httpx.ValidationErrorsKey])
	}
	fields, ok := validation["fields"].([]any)
	if !ok || len(fields) != 1 {
		t.Fatalf("validation fields = %#v, want 1 field error", validation["fields"])
	}
}

func TestRequestValidationRejectsInvalidJSONRequestBody(t *testing.T) {
	spec := buildCreateWidgetSpec()
	mw, err := New(spec)
	if err != nil {
		t.Fatalf("middleware error: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run for invalid body")
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/widgets", strings.NewReader(`{"name":123}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body httpx.Problem
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Type != httpx.DefaultTypeURI(httpx.TypeValidation) {
		t.Fatalf("problem type = %q, want validation", body.Type)
	}
	if body.Status != http.StatusUnprocessableEntity {
		t.Fatalf("problem status = %d, want 422", body.Status)
	}
}

func TestResponseValidationDisabledByDefaultPassesInvalidResponses(t *testing.T) {
	spec := buildPingSpec()
	mw, err := New(spec)
	if err != nil {
		t.Fatalf("middleware error: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"bad": true})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected response validation to be disabled by default, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["bad"] != true {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestMethodNotAllowedMapsToValidationProblem(t *testing.T) {
	status := statusFromError(routers.ErrMethodNotAllowed)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", status)
	}
	problem := problemForOpenAPIError(status, routers.ErrMethodNotAllowed)
	if problem.Type != httpx.DefaultTypeURI(httpx.TypeValidation) {
		t.Fatalf("problem type = %q, want validation", problem.Type)
	}
	if problem.Detail != "method not allowed" {
		t.Fatalf("detail = %q, want method not allowed", problem.Detail)
	}
}

func TestStatusFromErrorMapsNotFoundTo404(t *testing.T) {
	status := statusFromError(routers.ErrPathNotFound)
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
	problem := problemForOpenAPIError(status, routers.ErrPathNotFound)
	if problem.Type != httpx.DefaultTypeURI(httpx.TypeNotFound) {
		t.Fatalf("problem type = %q, want not-found", problem.Type)
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

func TestResponseValidationRejectsOversizedCapture(t *testing.T) {
	spec := buildPingSpec()
	mw, err := New(spec, WithResponseValidation(ResponseValidationOptions{
		Enabled:      true,
		MaxBodyBytes: 4,
	}))
	if err != nil {
		t.Fatalf("middleware error: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	var body httpx.Problem
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Detail != "response body exceeds validation limit" {
		t.Fatalf("detail = %q, want response body exceeds validation limit", body.Detail)
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

func TestResponseValidationCanSkipStreamingRoutes(t *testing.T) {
	spec := buildPingSpec()
	mw, err := New(spec, WithResponseValidation(ResponseValidationOptions{
		Enabled: true,
		ShouldValidate: func(r *http.Request) bool {
			return r.URL.Path != "/ping"
		},
	}))
	if err != nil {
		t.Fatalf("middleware error: %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setOptionalInterfaceHeaders(w)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"bad": true})
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected skipped response validation to preserve handler status, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-Has-Flusher"); got != "true" {
		t.Fatalf("expected skipped response validation to preserve flusher, got %q", got)
	}
}

func buildSearchSpec() *openapi3.T {
	responses := openapi3.NewResponses(
		openapi3.WithStatus(http.StatusOK, &openapi3.ResponseRef{
			Value: openapi3.NewResponse().WithDescription("ok"),
		}),
	)
	return &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Search",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(
			openapi3.WithPath("/search", &openapi3.PathItem{
				Get: &openapi3.Operation{
					OperationID: "Search",
					Parameters: openapi3.Parameters{
						&openapi3.ParameterRef{
							Value: openapi3.NewQueryParameter("limit").
								WithRequired(true).
								WithSchema(openapi3.NewIntegerSchema()),
						},
					},
					Responses: responses,
				},
			}),
		),
	}
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

func buildCreateWidgetSpec() *openapi3.T {
	requestSchema := openapi3.NewObjectSchema()
	requestSchema.Required = []string{"name"}
	requestSchema.Properties = map[string]*openapi3.SchemaRef{
		"name": {Value: openapi3.NewStringSchema()},
	}
	responseSchema := openapi3.NewObjectSchema()
	responseSchema.Required = []string{"id", "name"}
	responseSchema.Properties = map[string]*openapi3.SchemaRef{
		"id":   {Value: openapi3.NewStringSchema()},
		"name": {Value: openapi3.NewStringSchema()},
	}
	return &openapi3.T{
		OpenAPI: "3.0.0",
		Info: &openapi3.Info{
			Title:   "Widgets",
			Version: "1.0.0",
		},
		Paths: openapi3.NewPaths(
			openapi3.WithPath("/widgets", &openapi3.PathItem{
				Post: &openapi3.Operation{
					OperationID: "CreateWidget",
					RequestBody: &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().
						WithRequired(true).
						WithJSONSchema(requestSchema),
					},
					Responses: openapi3.NewResponses(
						openapi3.WithStatus(http.StatusCreated, &openapi3.ResponseRef{
							Value: openapi3.NewResponse().WithDescription("created").WithJSONSchema(responseSchema),
						}),
					),
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
