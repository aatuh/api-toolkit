package list

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseListQuerySort(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items?sort=name,-created_at,+name", nil)

	cfg := ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
		AllowedSorts: []string{"name", "created_at"},
	}

	q, err := ParseListQuery(req, cfg)
	if err != nil {
		t.Fatalf("parse list query: %v", err)
	}

	if len(q.Sort) != 2 {
		t.Fatalf("expected 2 sort fields, got %d", len(q.Sort))
	}
	if q.Sort[0].Field != "name" || q.Sort[0].Desc {
		t.Fatalf("expected first sort to be asc name, got %+v", q.Sort[0])
	}
	if q.Sort[1].Field != "created_at" || !q.Sort[1].Desc {
		t.Fatalf("expected second sort to be desc created_at, got %+v", q.Sort[1])
	}
}

func TestParseListQuerySortDefaults(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items", nil)

	cfg := ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
		DefaultSort: []SortField{
			{Field: "created_at", Desc: true},
		},
	}

	q, err := ParseListQuery(req, cfg)
	if err != nil {
		t.Fatalf("parse list query: %v", err)
	}

	if len(q.Sort) != 1 {
		t.Fatalf("expected default sort to be applied, got %d entries", len(q.Sort))
	}
	if q.Sort[0].Field != "created_at" || !q.Sort[0].Desc {
		t.Fatalf("unexpected default sort: %+v", q.Sort[0])
	}
}

func TestParseListQueryCustomFilterParser(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items?token=abc123", nil)

	cfg := ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
		FilterParser: func(values url.Values, _ ListQueryConfig) Filters {
			return Filters{
				"token_hash": {values.Get("token") + "_hashed"},
			}
		},
	}

	q, err := ParseListQuery(req, cfg)
	if err != nil {
		t.Fatalf("parse list query: %v", err)
	}

	if got := q.First("token_hash"); got != "abc123_hashed" {
		t.Fatalf("expected transformed filter value, got %q", got)
	}
}

func TestParseListQueryCustomSortParser(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items?order=created_at:desc", nil)

	cfg := ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
		SortParser: func(values url.Values, _ ListQueryConfig) []SortField {
			order := values.Get("order")
			if order == "" {
				return nil
			}
			parts := strings.Split(order, ":")
			if len(parts) != 2 {
				return nil
			}
			return []SortField{{
				Field: parts[0],
				Desc:  strings.EqualFold(parts[1], "desc"),
			}}
		},
	}

	q, err := ParseListQuery(req, cfg)
	if err != nil {
		t.Fatalf("parse list query: %v", err)
	}

	if len(q.Sort) != 1 || q.Sort[0].Field != "created_at" || !q.Sort[0].Desc {
		t.Fatalf("expected custom sort parser to apply, got %+v", q.Sort)
	}
}

func TestParseListQueryReturnsFieldErrorsForInvalidPagination(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items?limit=abc&offset=-1", nil)

	_, err := ParseListQuery(req, ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestParseListQueryReturnsFieldErrorsForUnsupportedFilterAndSort(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items?filter[status]=active&sort=name", nil)

	_, err := ParseListQuery(req, ListQueryConfig{
		DefaultLimit:   10,
		MaxLimit:       50,
		AllowedFilters: []string{"category"},
		AllowedSorts:   []string{"created_at"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
