package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

func TestCreatePetValidationErrorUsesProblemDetails(t *testing.T) {
	server := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/pets",
		strings.NewReader(`{"name":"   "}`),
	)

	server.CreatePet(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected problem content type, got %q", got)
	}

	var body struct {
		Type       string `json:"type"`
		Title      string `json:"title"`
		Status     int    `json:"status"`
		Detail     string `json:"detail"`
		Validation struct {
			Fields []struct {
				Field   string `json:"field"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"fields"`
		} `json:"validation"`
		FieldErrors map[string]string `json:"field_errors"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Type != httpx.DefaultTypeURI(httpx.TypeValidation) {
		t.Fatalf("expected validation type, got %q", body.Type)
	}
	if body.Title != http.StatusText(http.StatusBadRequest) {
		t.Fatalf("expected bad request title, got %q", body.Title)
	}
	if body.Detail != "validation failed" {
		t.Fatalf("expected validation detail, got %q", body.Detail)
	}
	if len(body.Validation.Fields) != 1 {
		t.Fatalf("expected 1 validation field, got %d", len(body.Validation.Fields))
	}
	if got := body.Validation.Fields[0]; got.Field != "name" || got.Code != "required" || got.Message != "name is required" {
		t.Fatalf("unexpected validation field: %+v", got)
	}
	if got := body.FieldErrors["name"]; got != "name is required" {
		t.Fatalf("expected compatibility field_errors entry, got %q", got)
	}
}

func TestOpenAPISpecUsesProblemDetailsForErrors(t *testing.T) {
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile("openapi.json")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := spec.Validate(loader.Context); err != nil {
		t.Fatalf("validate spec: %v", err)
	}

	post := spec.Paths.Map()["/pets"].Post
	get := spec.Paths.Map()["/pets"].Get

	assertProblemResponse(t, get.Responses.Map()["400"])
	assertProblemResponse(t, post.Responses.Map()["400"])
	assertProblemResponse(t, post.Responses.Map()["409"])

	problemSchema := spec.Components.Schemas["Problem"]
	if problemSchema == nil || problemSchema.Value == nil {
		t.Fatal("expected Problem schema")
	}
	if _, ok := problemSchema.Value.Properties["validation"]; !ok {
		t.Fatal("expected validation extension on Problem schema")
	}
	if _, ok := problemSchema.Value.Properties["errors"]; !ok {
		t.Fatal("expected errors extension on Problem schema")
	}
	fieldErrors := problemSchema.Value.Properties["field_errors"]
	if fieldErrors == nil || fieldErrors.Ref != "#/components/schemas/FieldErrorMap" {
		t.Fatalf("expected field_errors ref, got %#v", fieldErrors)
	}
}

func assertProblemResponse(t *testing.T, resp *openapi3.ResponseRef) {
	t.Helper()
	if resp == nil || resp.Value == nil {
		t.Fatal("expected response")
	}
	if _, ok := resp.Value.Content["application/json"]; ok {
		t.Fatal("did not expect application/json error response")
	}
	problem := resp.Value.Content["application/problem+json"]
	if problem == nil || problem.Schema == nil {
		t.Fatal("expected application/problem+json schema")
	}
	if problem.Schema.Ref != "#/components/schemas/Problem" {
		t.Fatalf("expected Problem schema ref, got %q", problem.Schema.Ref)
	}
}
