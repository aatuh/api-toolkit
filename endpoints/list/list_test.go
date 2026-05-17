package list

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
)

func TestParseListQuerySort(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items?sort=name,-created_at,+name", nil)

	cfg := ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
		AllowedSorts: []string{"name", "created_at"},
	}

	q, err := ParseListQueryChecked(req, cfg)
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

	q, err := ParseListQueryChecked(req, cfg)
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
		FilterParser: func(values url.Values, _ ListQueryConfig) (Filters, fielderrors.FieldErrors) {
			return Filters{
				"token_hash": {values.Get("token") + "_hashed"},
			}, nil
		},
	}

	q, err := ParseListQueryChecked(req, cfg)
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
		SortParser: func(values url.Values, _ ListQueryConfig) ([]SortField, fielderrors.FieldErrors) {
			order := values.Get("order")
			if order == "" {
				return nil, nil
			}
			parts := strings.Split(order, ":")
			if len(parts) != 2 {
				return nil, nil
			}
			return []SortField{{
				Field: parts[0],
				Desc:  strings.EqualFold(parts[1], "desc"),
			}}, nil
		},
	}

	q, err := ParseListQueryChecked(req, cfg)
	if err != nil {
		t.Fatalf("parse list query: %v", err)
	}

	if len(q.Sort) != 1 || q.Sort[0].Field != "created_at" || !q.Sort[0].Desc {
		t.Fatalf("expected custom sort parser to apply, got %+v", q.Sort)
	}
}

func TestParseListQueryReturnsFieldErrorsForInvalidPagination(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items?limit=abc&offset=-1", nil)

	_, err := ParseListQueryChecked(req, ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestParseListQueryReturnsFieldErrorsForUnsupportedFilterAndSort(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items?filter[status]=active&sort=name", nil)

	_, err := ParseListQueryChecked(req, ListQueryConfig{
		DefaultLimit:   10,
		MaxLimit:       50,
		AllowedFilters: []string{"category"},
		AllowedSorts:   []string{"created_at"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestParseListQueryCheckedReturnsPartialQueryWithErrors(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/items?limit=abc&filter[status]=active&sort=-created_at", nil)

	q, err := ParseListQueryChecked(req, ListQueryConfig{
		DefaultLimit:   10,
		MaxLimit:       50,
		AllowedFilters: []string{"status"},
		AllowedSorts:   []string{"created_at"},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	if q.Limit != 10 {
		t.Fatalf("expected default limit through checked parser, got %d", q.Limit)
	}
	if got := q.First("status"); got != "active" {
		t.Fatalf("expected filter through checked parser, got %q", got)
	}
	if len(q.Sort) != 1 || q.Sort[0].Field != "created_at" || !q.Sort[0].Desc {
		t.Fatalf("expected sort through checked parser, got %+v", q.Sort)
	}
}

func TestDefaultCheckedParsersReturnValidatedValues(t *testing.T) {
	values := url.Values{
		"filter[status]": []string{"active"},
		"sort":           []string{"-created_at"},
	}
	cfg := ListQueryConfig{
		AllowedFilters: []string{"status"},
		AllowedSorts:   []string{"created_at"},
	}

	filters, filterErrs := DefaultFilterParserChecked(values, cfg)
	if len(filterErrs) > 0 {
		t.Fatalf("expected no filter errors, got %v", filterErrs)
	}
	if got := filters["status"]; len(got) != 1 || got[0] != "active" {
		t.Fatalf("expected checked filter parser output, got %#v", filters)
	}
	sortFields, sortErrs := DefaultSortParserChecked(values, cfg)
	if len(sortErrs) > 0 {
		t.Fatalf("expected no sort errors, got %v", sortErrs)
	}
	if len(sortFields) != 1 || sortFields[0].Field != "created_at" || !sortFields[0].Desc {
		t.Fatalf("expected checked sort parser output, got %+v", sortFields)
	}
}
