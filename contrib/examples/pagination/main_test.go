package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

func TestListItemsRejectsMalformedPagination(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items?limit=abc", nil)

	listItems(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("expected problem response, got %q", got)
	}

	var body struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
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
}

func TestListItemsReturnsPagedResults(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/items?limit=3&offset=2", nil)

	listItems(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body listResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(body.Items))
	}
	if body.Items[0] != "charlie" || body.Items[2] != "echo" {
		t.Fatalf("unexpected page items: %+v", body.Items)
	}
	if body.NextOffset == nil || *body.NextOffset != 5 {
		t.Fatalf("expected next offset 5, got %v", body.NextOffset)
	}
}
