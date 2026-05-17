package openapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/routers"

	"github.com/aatuh/api-toolkit/v3/httpx"
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
