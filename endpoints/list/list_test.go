package list

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseListQuerySort(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?sort=name,-created_at,+name", nil)

	cfg := ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
		AllowedSorts: []string{"name", "created_at"},
	}

	q := ParseListQuery(req, cfg)

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
	req := httptest.NewRequest("GET", "/items", nil)

	cfg := ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
		DefaultSort: []SortField{
			{Field: "created_at", Desc: true},
		},
	}

	q := ParseListQuery(req, cfg)

	if len(q.Sort) != 1 {
		t.Fatalf("expected default sort to be applied, got %d entries", len(q.Sort))
	}
	if q.Sort[0].Field != "created_at" || !q.Sort[0].Desc {
		t.Fatalf("unexpected default sort: %+v", q.Sort[0])
	}
}

func TestParseListQueryCustomFilterParser(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?token=abc123", nil)

	cfg := ListQueryConfig{
		DefaultLimit: 10,
		MaxLimit:     50,
		FilterParser: func(values url.Values, _ ListQueryConfig) Filters {
			return Filters{
				"token_hash": {values.Get("token") + "_hashed"},
			}
		},
	}

	q := ParseListQuery(req, cfg)

	if got := q.First("token_hash"); got != "abc123_hashed" {
		t.Fatalf("expected transformed filter value, got %q", got)
	}
}

func TestParseListQueryCustomSortParser(t *testing.T) {
	req := httptest.NewRequest("GET", "/items?order=created_at:desc", nil)

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

	q := ParseListQuery(req, cfg)

	if len(q.Sort) != 1 || q.Sort[0].Field != "created_at" || !q.Sort[0].Desc {
		t.Fatalf("expected custom sort parser to apply, got %+v", q.Sort)
	}
}
