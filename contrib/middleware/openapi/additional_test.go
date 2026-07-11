package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

func TestNewRejectsNilSpecAndNilMiddlewarePassesThrough(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("expected nil spec error")
	}
	var mw *Middleware
	called := false
	handler := mw.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))
	if !called || rec.Code != http.StatusNoContent {
		t.Fatalf("nil middleware called=%v status=%d", called, rec.Code)
	}
}

func TestNewAcceptsSpecOptionFilterOptionsAndFileInput(t *testing.T) {
	mw, err := New(nil, WithSpec(buildPingSpec()), WithFilterOptions(openapi3filter.Options{}))
	if err != nil {
		t.Fatalf("New() with spec option error = %v", err)
	}
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	data, err := json.Marshal(buildPingSpec())
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	path := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if _, err := NewFromFile(path); err != nil {
		t.Fatalf("NewFromFile() error = %v", err)
	}
	if _, err := NewFromFile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected missing spec file error")
	}
}

func TestMiddlewarePassesThroughWhenRouterMissing(t *testing.T) {
	mw := &Middleware{}
	called := false
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	})).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/anything", nil))

	if !called || rec.Code != http.StatusAccepted {
		t.Fatalf("called=%v status=%d, want pass-through 202", called, rec.Code)
	}
}

func TestIgnoreNotFoundAndCustomErrorHandler(t *testing.T) {
	mw, err := New(buildPingSpec(), WithIgnoreNotFound(true))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	called := false
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/missing", nil))
	if !called || rec.Code != http.StatusAccepted {
		t.Fatalf("ignored 404 called=%v status=%d", called, rec.Code)
	}

	var gotStatus int
	var gotErr error
	mw, err = New(buildSearchSpec(), WithErrorHandler(func(w http.ResponseWriter, r *http.Request, status int, err error) {
		gotStatus = status
		gotErr = err
		w.WriteHeader(status)
	}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rec = httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	})).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/search", nil))
	if gotStatus != http.StatusBadRequest || gotErr == nil || rec.Code != http.StatusBadRequest {
		t.Fatalf("custom handler status=%d err=%v rec=%d", gotStatus, gotErr, rec.Code)
	}
}

func TestHandlerMapsRouteFailuresToProblemDetails(t *testing.T) {
	mw, err := New(buildPingSpec())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	}))

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantType   string
		wantDetail string
	}{
		{
			name:       "not found",
			method:     http.MethodGet,
			path:       "/missing",
			wantStatus: http.StatusNotFound,
			wantType:   httpx.DefaultTypeURI(httpx.TypeNotFound),
			wantDetail: "route not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), tt.method, tt.path, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Fatalf("content type = %q, want problem json", got)
			}
			var body httpx.Problem
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if body.Type != tt.wantType || body.Detail != tt.wantDetail {
				t.Fatalf("problem = %#v, want type=%q detail=%q", body, tt.wantType, tt.wantDetail)
			}
		})
	}
}

func TestHandlerMapsMethodNotAllowedRouterError(t *testing.T) {
	mw := &Middleware{
		router:       routeErrorRouter{err: routers.ErrMethodNotAllowed},
		errorHandler: defaultErrorHandler,
	}
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	})).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/ping", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	var body httpx.Problem
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if body.Type != httpx.DefaultTypeURI(httpx.TypeValidation) || body.Detail != "method not allowed" {
		t.Fatalf("problem = %#v, want validation method-not-allowed", body)
	}
}

func TestProblemMappingVariantsAndPointerFields(t *testing.T) {
	if got := problemForOpenAPIError(http.StatusForbidden, errors.New("forbidden")); got.Type != httpx.DefaultTypeURI(httpx.TypeForbidden) {
		t.Fatalf("forbidden problem = %#v", got)
	}
	if got := problemForOpenAPIError(http.StatusNotFound, &routers.RouteError{}); got.Detail != "route not found" {
		t.Fatalf("not found problem = %#v", got)
	}
	if field := pointerToField("body", "/items/0/name~1value~0raw"); field != "body.items.0.name/value~raw" {
		t.Fatalf("pointer field = %q", field)
	}
	if detail := validationDetail(errors.New("plain")); detail != "" {
		t.Fatalf("plain validation detail = %q", detail)
	}
	if status := statusFromOpenAPIError(nil); status != http.StatusBadRequest {
		t.Fatalf("nil status = %d, want 400", status)
	}
	respErr := &openapi3filter.ResponseError{Reason: "bad response"}
	if status := statusFromOpenAPIError(respErr); status != http.StatusInternalServerError {
		t.Fatalf("response status = %d, want 500", status)
	}
	validationErr := &openapi3filter.ValidationError{
		Status: http.StatusConflict,
		Title:  "conflict",
		Source: &openapi3filter.ValidationErrorSource{Parameter: "query.cursor"},
	}
	if status := statusFromOpenAPIError(validationErr); status != http.StatusConflict {
		t.Fatalf("validation status = %d, want 409", status)
	}
	fields := fieldErrorsFromOpenAPI(validationErr)
	if len(fields) != 1 || fields[0].Field != "query.cursor" || fields[0].Message != "conflict" {
		t.Fatalf("validation fields = %#v", fields)
	}
	parseErr := &openapi3filter.ParseError{Kind: openapi3filter.KindUnsupportedFormat, Reason: "bad style"}
	if code := codeFromOpenAPIError(parseErr); code != "unsupported_format" {
		t.Fatalf("parse code = %q, want unsupported_format", code)
	}
	if msg := messageFromOpenAPIError(parseErr); msg != "bad style" {
		t.Fatalf("parse message = %q, want bad style", msg)
	}
	if status := statusFromOpenAPIError(&openapi3filter.SecurityRequirementsError{}); status != http.StatusUnauthorized {
		t.Fatalf("security status = %d, want 401", status)
	}
	schemaErr := &openapi3.SchemaError{SchemaField: "Min Length", Reason: "too short"}
	if pointer := schemaPointerFromError(schemaErr); pointer != "" {
		t.Fatalf("schema pointer = %q, want empty without path", pointer)
	}
	if code := codeFromOpenAPIError(schemaErr); code != "min_length" {
		t.Fatalf("schema code = %q, want min_length", code)
	}
	if msg := messageFromOpenAPIError(schemaErr); msg != "too short" {
		t.Fatalf("schema message = %q, want too short", msg)
	}
	requestErr := &openapi3filter.RequestError{
		RequestBody: &openapi3.RequestBody{},
		Reason:      "body is invalid",
		Err:         schemaErr,
	}
	if field := fieldFromRequestError(requestErr); field != "body" {
		t.Fatalf("request body field = %q, want body", field)
	}
	if msg := messageFromOpenAPIError(requestErr); msg != "too short" {
		t.Fatalf("request message = %q, want schema reason", msg)
	}
	requestErrReason := &openapi3filter.RequestError{Reason: "body is invalid"}
	if msg := messageFromOpenAPIError(requestErrReason); msg != "body is invalid" {
		t.Fatalf("request reason message = %q, want body is invalid", msg)
	}
	if code := codeFromOpenAPIError(&openapi3filter.ParseError{Kind: openapi3filter.KindInvalidFormat}); code != "invalid_format" {
		t.Fatalf("invalid format code = %q", code)
	}
	if code := codeFromOpenAPIError(&openapi3filter.ParseError{Kind: openapi3filter.KindOther}); code != "invalid" {
		t.Fatalf("other parse code = %q", code)
	}
	if code := codeFromOpenAPIError(nil); code != "invalid" {
		t.Fatalf("nil code = %q", code)
	}
	if msg := messageFromOpenAPIError(nil); msg != "invalid" {
		t.Fatalf("nil message = %q", msg)
	}
	if field := fieldFromParameter(&openapi3.Parameter{Name: " ", In: "query"}); field != "" {
		t.Fatalf("blank parameter field = %q", field)
	}
	if field := fieldFromParameter(&openapi3.Parameter{Name: "cursor"}); field != "cursor" {
		t.Fatalf("parameter field = %q, want cursor", field)
	}
	if code := normalizeCode("123"); code != "invalid" {
		t.Fatalf("numeric code = %q, want invalid", code)
	}
	if code := normalizeCode("Bad-Thing"); code != "bad_thing" {
		t.Fatalf("normalized code = %q, want bad_thing", code)
	}
}

func TestResponseValidationPassesValidResponseAndCopiesHeaders(t *testing.T) {
	mw, err := New(buildPingSpec(), WithResponseValidation(ResponseValidationOptions{Enabled: true}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "copied")
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("X-Test") != "copied" || !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Fatalf("response = %d headers=%v body=%s", rec.Code, rec.Header(), rec.Body.String())
	}
}

func TestResponseValidationUsesCustomErrorHandler(t *testing.T) {
	var gotStatus int
	var gotErr error
	mw, err := New(buildPingSpec(), WithResponseValidation(ResponseValidationOptions{
		Enabled: true,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, status int, err error) {
			gotStatus = status
			gotErr = err
			httpx.WriteProblem(w, status, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeInternal),
				Title:  "custom response validation failed",
				Detail: "response validation failed",
			})
		},
	}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rec := httptest.NewRecorder()
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"bad": true})
	})).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil))

	if gotStatus != http.StatusInternalServerError || gotErr == nil {
		t.Fatalf("custom response handler status=%d err=%v", gotStatus, gotErr)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "custom response validation failed") {
		t.Fatalf("body = %s, want custom problem", rec.Body.String())
	}
}

func TestResponseValidationCanBypassLargeStreamingResponses(t *testing.T) {
	mw, err := New(buildPingSpec(), WithResponseValidation(ResponseValidationOptions{
		Enabled:      true,
		MaxBodyBytes: 4,
		ShouldValidate: func(r *http.Request) bool {
			return r.Header.Get("X-API-Toolkit-Streaming") != "true"
		},
	}))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	req.Header.Set("X-API-Toolkit-Streaming", "true")
	mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setOptionalInterfaceHeaders(w)
		_, _ = w.Write([]byte(strings.Repeat("x", 128)))
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 128 {
		t.Fatalf("body length = %d, want 128", rec.Body.Len())
	}
	if got := rec.Header().Get("X-Has-Flusher"); got != "true" {
		t.Fatalf("flusher header = %q, want true", got)
	}
}

type routeErrorRouter struct {
	err error
}

func (r routeErrorRouter) FindRoute(*http.Request) (*routers.Route, map[string]string, error) {
	return nil, nil, r.err
}
