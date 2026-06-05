package queryparams

import (
	"net/url"
	"testing"
)

func BenchmarkQueryParamsParseRequestShape(b *testing.B) {
	values := url.Values{
		"sort":                    []string{"name,-created_at,+id"},
		"filter[status]":          []string{"active", "paused"},
		"filter[created_at][gte]": []string{"2026-01-01"},
		"fields":                  []string{"id,name,created_at"},
		"fields[widgets]":         []string{"id,name,owner"},
		"include":                 []string{"owner,items"},
	}
	sortConfig := SortConfig{AllowedFields: []string{"name", "created_at", "id"}}
	filterConfig := FilterConfig{Fields: []FilterField{
		{Name: "status"},
		{Name: "created_at", Operators: []FilterOperator{FilterOperatorGreaterThanOrEqual}},
	}}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseSort(values, sortConfig); err != nil {
			b.Fatalf("ParseSort() error = %v", err)
		}
		if _, err := ParseFilters(values, filterConfig); err != nil {
			b.Fatalf("ParseFilters() error = %v", err)
		}
		if _, err := ParseFields(values); err != nil {
			b.Fatalf("ParseFields() error = %v", err)
		}
		if _, err := ParseIncludes(values); err != nil {
			b.Fatalf("ParseIncludes() error = %v", err)
		}
	}
}
