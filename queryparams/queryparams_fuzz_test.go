package queryparams

import (
	"net/url"
	"strings"
	"testing"
)

func FuzzParseCollectionQuery(f *testing.F) {
	for _, seed := range []string{
		"sort=name,-created_at&include=owner,items&fields=id,name",
		"filter[status]=active&filter[created_at][gte]=2026-01-01",
		"filter=&fields[]=bad&include=,owner",
		"sort=+id,-",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, rawQuery string) {
		rawQuery = limitQueryFuzzString(rawQuery, 4096)
		values, err := url.ParseQuery(rawQuery)
		if err != nil {
			return
		}

		sort, _ := ParseSortChecked(values, SortConfig{})
		for _, field := range sort.Fields {
			if strings.TrimSpace(field.Name) == "" {
				t.Fatalf("ParseSort returned an empty field from %q: %#v", rawQuery, sort.Fields)
			}
		}

		filters, _ := ParseFiltersChecked(values, FilterConfig{})
		for _, filter := range filters {
			if strings.TrimSpace(filter.Field) == "" {
				t.Fatalf("ParseFilters returned an empty field from %q: %#v", rawQuery, filters)
			}
			if !knownOperator(filter.Operator) {
				t.Fatalf("ParseFilters returned unknown operator from %q: %#v", rawQuery, filter)
			}
		}

		fields, _ := ParseFieldsChecked(values)
		assertNoEmptyOrDuplicateFuzzValues(t, rawQuery, "fields", fields.Fields)
		for resource, names := range fields.ByResource {
			if strings.TrimSpace(resource) == "" {
				t.Fatalf("ParseFields returned an empty resource from %q: %#v", rawQuery, fields.ByResource)
			}
			assertNoEmptyOrDuplicateFuzzValues(t, rawQuery, "fields["+resource+"]", names)
		}

		includes, _ := ParseIncludesChecked(values)
		assertNoEmptyOrDuplicateFuzzValues(t, rawQuery, "include", includes.Includes)
	})
}

func assertNoEmptyOrDuplicateFuzzValues(t *testing.T, rawQuery, field string, values []string) {
	t.Helper()

	seen := map[string]struct{}{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("%s returned an empty value from %q: %#v", field, rawQuery, values)
		}
		if _, ok := seen[value]; ok {
			t.Fatalf("%s returned duplicate value from %q: %#v", field, rawQuery, values)
		}
		seen[value] = struct{}{}
	}
}

func limitQueryFuzzString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
