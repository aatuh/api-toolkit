package queryparams

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/aatuh/api-toolkit/v2/fielderrors"
)

func TestParseSortHandlesDirectionRepeatedFieldsAndUnknownFields(t *testing.T) {
	values := url.Values{"sort": []string{"name,-created_at", "+id"}}
	got, err := ParseSort(values, SortConfig{AllowedFields: []string{"name", "created_at", "id"}})
	if err != nil {
		t.Fatalf("ParseSort() error = %v", err)
	}
	want := Sort{Fields: []SortField{{Name: "name"}, {Name: "created_at", Desc: true}, {Name: "id"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sort = %#v, want %#v", got, want)
	}
	_, err = ParseSort(url.Values{"sort": []string{"secret"}}, SortConfig{AllowedFields: []string{"name"}})
	if !hasFieldError(err, "sort", "unknown_field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestParseFiltersHandlesBracketSyntaxRepeatedValuesAndOperators(t *testing.T) {
	values := url.Values{
		"filter[status]":          []string{"active", "paused"},
		"filter[created_at][gte]": []string{"2026-01-01"},
	}
	got, err := ParseFilters(values, FilterConfig{Fields: []FilterField{
		{Name: "status"},
		{Name: "created_at", Operators: []FilterOperator{FilterOperatorGreaterThanOrEqual}},
	}})
	if err != nil {
		t.Fatalf("ParseFilters() error = %v", err)
	}
	want := []Filter{
		{Field: "created_at", Operator: FilterOperatorGreaterThanOrEqual, Values: []string{"2026-01-01"}},
		{Field: "status", Operator: FilterOperatorEqual, Values: []string{"active", "paused"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filters = %#v, want %#v", got, want)
	}
}

func TestParseFiltersRejectsUnknownFieldsOperatorsAndBadSyntax(t *testing.T) {
	_, err := ParseFilters(url.Values{"filter[secret]": []string{"1"}}, FilterConfig{Fields: []FilterField{{Name: "status"}}})
	if !hasFieldError(err, "filter.secret", "unknown_field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
	_, err = ParseFilters(url.Values{"filter[status][regex]": []string{"a"}}, FilterConfig{})
	if !hasFieldError(err, "filter.status", "unknown_operator") {
		t.Fatalf("expected unknown operator error, got %v", err)
	}
	_, err = ParseFilters(url.Values{"filter": []string{"bad"}}, FilterConfig{})
	if !hasFieldError(err, "filter", "invalid") {
		t.Fatalf("expected invalid syntax error, got %v", err)
	}
}

func TestParseFieldsHandlesSparseFieldsets(t *testing.T) {
	got, err := ParseFields(url.Values{
		"fields":          []string{"id,name", "created_at"},
		"fields[widgets]": []string{"id,name,name"},
	})
	if err != nil {
		t.Fatalf("ParseFields() error = %v", err)
	}
	if !reflect.DeepEqual(got.Fields, []string{"id", "name", "created_at"}) {
		t.Fatalf("fields = %#v", got.Fields)
	}
	if !reflect.DeepEqual(got.ByResource["widgets"], []string{"id", "name"}) {
		t.Fatalf("resource fields = %#v", got.ByResource)
	}
}

func TestParseIncludesHandlesIncludesAndEmptyInput(t *testing.T) {
	got, err := ParseIncludes(url.Values{"include": []string{"owner,items", "owner"}})
	if err != nil {
		t.Fatalf("ParseIncludes() error = %v", err)
	}
	if !reflect.DeepEqual(got.Includes, []string{"owner", "items"}) {
		t.Fatalf("includes = %#v", got.Includes)
	}
	empty, err := ParseIncludes(url.Values{})
	if err != nil {
		t.Fatalf("ParseIncludes(empty) error = %v", err)
	}
	if len(empty.Includes) != 0 {
		t.Fatalf("empty includes = %#v", empty)
	}
}

func hasFieldError(err error, field, code string) bool {
	provider, ok := err.(fielderrors.Provider)
	if !ok {
		return false
	}
	for _, entry := range provider.FieldErrors() {
		if entry.Field == field && entry.Code == code {
			return true
		}
	}
	return false
}
