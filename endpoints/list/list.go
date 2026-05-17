package list

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
)

// ListQuery captures pagination, search, and filter inputs from a request.
//
//revive:disable-next-line:exported
type ListQuery struct {
	Limit   int
	Offset  int
	Search  string
	Filters Filters
	Sort    []SortField
	Raw     url.Values
	missing []string
}

// MissingRequired returns filter keys that were required but absent.
func (q ListQuery) MissingRequired() []string {
	out := make([]string, len(q.missing))
	copy(out, q.missing)
	return out
}

// First returns the first filter value for the key if present.
func (q ListQuery) First(key string) string {
	vals := q.Filters[key]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// Has reports whether the filter key exists.
func (q ListQuery) Has(key string) bool {
	_, ok := q.Filters[key]
	return ok
}

// Filters describes allowed filter values.
type Filters map[string][]string

// SortField describes a field and direction used for ordering results.
type SortField struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc"`
}

// ListQueryConfig configures parsing behaviour for list endpoints.
//
//revive:disable-next-line:exported
type ListQueryConfig struct {
	DefaultLimit   int
	MaxLimit       int
	AllowedFilters []string
	Required       []string
	SearchParam    string
	SortParam      string
	AllowedSorts   []string
	DefaultSort    []SortField
	// FilterParser overrides the default filter parser when set.
	FilterParser func(values url.Values, cfg ListQueryConfig) (Filters, fielderrors.FieldErrors)
	// SortParser overrides the default sort parser when set.
	SortParser func(values url.Values, cfg ListQueryConfig) ([]SortField, fielderrors.FieldErrors)
}

// ParseListQueryChecked parses pagination and filters from the HTTP request and
// returns field-level validation errors.
func ParseListQueryChecked(r *http.Request, cfg ListQueryConfig) (ListQuery, error) {
	values := r.URL.Query()
	limit, limitErrs := parseLimit(values.Get("limit"), cfg.DefaultLimit, cfg.MaxLimit)
	offset, offsetErrs := parseOffset(values.Get("offset"))

	searchKey := effectiveSearchKey(cfg)
	search := strings.TrimSpace(values.Get(searchKey))

	filters, filterErrs := filtersFromConfig(values, cfg)
	missing := requiredMissing(filters, cfg.Required)
	sortFields, sortErrs := sortFromConfig(values, cfg)

	var errs fielderrors.FieldErrors
	errs = append(errs, limitErrs...)
	errs = append(errs, offsetErrs...)
	errs = append(errs, filterErrs...)
	errs = append(errs, sortErrs...)
	for _, key := range missing {
		errs = append(errs, fielderrors.FieldError{
			Field:   "filter." + key,
			Code:    "required",
			Message: key + " is required",
		})
	}

	query := ListQuery{
		Limit:   limit,
		Offset:  offset,
		Search:  search,
		Filters: filters,
		Sort:    sortFields,
		Raw:     values,
		missing: missing,
	}
	if len(errs) > 0 {
		return query, errs
	}
	return query, nil
}

// ListMeta captures pagination metadata for responses.
//
//revive:disable-next-line:exported
type ListMeta struct {
	Total   int                 `json:"total"`
	Count   int                 `json:"count"`
	Limit   int                 `json:"limit"`
	Offset  int                 `json:"offset"`
	Filters map[string][]string `json:"filters,omitempty"`
	Search  string              `json:"search,omitempty"`
	Sort    []SortField         `json:"sort,omitempty"`
}

// ListResponse wraps list results with metadata.
//
//revive:disable-next-line:exported
type ListResponse[T any] struct {
	Data []T      `json:"data"`
	Meta ListMeta `json:"meta"`
}

// NewListResponse constructs a ListResponse with paging metadata.
func NewListResponse[T any](items []T, total int, query ListQuery) ListResponse[T] {
	meta := ListMeta{
		Total:  total,
		Count:  len(items),
		Limit:  query.Limit,
		Offset: query.Offset,
		Search: query.Search,
	}
	if len(query.Filters) > 0 {
		meta.Filters = cloneFilters(query.Filters)
	}
	if len(query.Sort) > 0 {
		meta.Sort = cloneSort(query.Sort)
	}
	return ListResponse[T]{
		Data: items,
		Meta: meta,
	}
}

func cloneFilters(in Filters) map[string][]string {
	out := make(map[string][]string, len(in))
	for k, vals := range in {
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[k] = cp
	}
	return out
}

func parseLimit(val string, def, max int) (int, fielderrors.FieldErrors) {
	if strings.TrimSpace(val) == "" {
		return clampLimit(def, max), nil
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return clampLimit(def, max), fielderrors.FieldErrors{{
			Field:   "limit",
			Code:    "invalid",
			Message: "limit must be a positive integer",
		}}
	}
	if max > 0 && n > max {
		return clampLimit(def, max), fielderrors.FieldErrors{{
			Field:   "limit",
			Code:    "max",
			Message: "limit exceeds maximum",
		}}
	}
	return clampLimit(n, max), nil
}

func parseOffset(val string) (int, fielderrors.FieldErrors) {
	if strings.TrimSpace(val) == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return 0, fielderrors.FieldErrors{{
			Field:   "offset",
			Code:    "invalid",
			Message: "offset must be a non-negative integer",
		}}
	}
	return n, nil
}

func clampLimit(n, maxVal int) int {
	if n < 1 {
		n = 1
	}
	if maxVal > 0 && n > maxVal {
		n = maxVal
	}
	return n
}

func effectiveSearchKey(cfg ListQueryConfig) string {
	if strings.TrimSpace(cfg.SearchParam) == "" {
		return "search"
	}
	return cfg.SearchParam
}

func effectiveSortKey(cfg ListQueryConfig) string {
	if strings.TrimSpace(cfg.SortParam) == "" {
		return "sort"
	}
	return cfg.SortParam
}

func filtersFromConfig(values url.Values, cfg ListQueryConfig) (Filters, fielderrors.FieldErrors) {
	if cfg.FilterParser != nil {
		filters, errs := cfg.FilterParser(values, cfg)
		if filters == nil {
			filters = make(Filters)
		}
		return filters, errs
	}
	return DefaultFilterParserChecked(values, cfg)
}

// DefaultFilterParserChecked implements the toolkit's standard query syntax and
// returns unsupported-filter validation errors.
func DefaultFilterParserChecked(values url.Values, cfg ListQueryConfig) (Filters, fielderrors.FieldErrors) {
	searchKey := effectiveSearchKey(cfg)
	sortKey := effectiveSortKey(cfg)
	allowed := buildAllowedSet(cfg.AllowedFilters)

	filters := make(Filters)
	var errs fielderrors.FieldErrors
	for key, vals := range values {
		if key == "limit" || key == "offset" || key == searchKey || key == sortKey {
			continue
		}
		if name, ok := parseFilterKey(key); ok {
			if !allowFilter(name, allowed) {
				errs = append(errs, fielderrors.FieldError{
					Field:   "filter." + name,
					Code:    "unsupported",
					Message: name + " is not a supported filter",
				})
				continue
			}
			addFilter(filters, name, vals)
			continue
		}
		if !allowFilter(key, allowed) {
			errs = append(errs, fielderrors.FieldError{
				Field:   "filter." + key,
				Code:    "unsupported",
				Message: key + " is not a supported filter",
			})
			continue
		}
		addFilter(filters, key, vals)
	}
	return filters, errs
}

func sortFromConfig(values url.Values, cfg ListQueryConfig) ([]SortField, fielderrors.FieldErrors) {
	if cfg.SortParser != nil {
		sortFields, errs := cfg.SortParser(values, cfg)
		return cloneSort(sortFields), errs
	}
	return DefaultSortParserChecked(values, cfg)
}

// DefaultSortParserChecked implements the toolkit's comma-delimited sort syntax
// and returns unsupported-sort validation errors.
func DefaultSortParserChecked(values url.Values, cfg ListQueryConfig) ([]SortField, fielderrors.FieldErrors) {
	sortKey := effectiveSortKey(cfg)
	sortAllowed := buildAllowedSet(cfg.AllowedSorts)
	return parseSort(values[sortKey], sortAllowed, cfg.DefaultSort)
}

func parseFilterKey(key string) (string, bool) {
	if strings.HasPrefix(key, "filter[") && strings.HasSuffix(key, "]") {
		name := key[len("filter[") : len(key)-1]
		return strings.TrimSpace(name), name != ""
	}
	if strings.HasPrefix(key, "filter.") {
		name := key[len("filter."):]
		return strings.TrimSpace(name), name != ""
	}
	return "", false
}

func allowFilter(name string, allowed map[string]struct{}) bool {
	if len(allowed) == 0 {
		return true
	}
	_, ok := allowed[name]
	return ok
}

func parseSort(values []string, allowed map[string]struct{}, defaults []SortField) ([]SortField, fielderrors.FieldErrors) {
	var out []SortField
	seen := make(map[string]struct{})
	var errs fielderrors.FieldErrors
	for _, raw := range values {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		parts := strings.Split(raw, ",")
		for _, part := range parts {
			field, desc, ok := normalizeSort(part)
			if !ok {
				errs = append(errs, fielderrors.FieldError{
					Field:   "sort",
					Code:    "invalid",
					Message: "sort contains an invalid value",
				})
				continue
			}
			if !allowFilter(field, allowed) {
				errs = append(errs, fielderrors.FieldError{
					Field:   "sort",
					Code:    "unsupported",
					Message: field + " is not a supported sort field",
				})
				continue
			}
			key := field
			if desc {
				key = "-" + key
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, SortField{Field: field, Desc: desc})
		}
	}
	if len(out) == 0 {
		return cloneSort(defaults), errs
	}
	return out, errs
}

func normalizeSort(in string) (field string, desc bool, ok bool) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", false, false
	}
	switch {
	case strings.HasPrefix(in, "-"):
		return strings.TrimSpace(in[1:]), true, strings.TrimSpace(in[1:]) != ""
	case strings.HasPrefix(in, "+"):
		return strings.TrimSpace(in[1:]), false, strings.TrimSpace(in[1:]) != ""
	default:
		return in, false, true
	}
}

func cloneSort(in []SortField) []SortField {
	if len(in) == 0 {
		return nil
	}
	out := make([]SortField, len(in))
	copy(out, in)
	return out
}

func buildAllowedSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	return out
}

func addFilter(filters Filters, key string, vals []string) {
	if len(vals) == 0 {
		return
	}
	trimmed := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		trimmed = append(trimmed, v)
	}
	if len(trimmed) == 0 {
		return
	}
	filters[key] = append(filters[key], trimmed...)
}

func requiredMissing(filters Filters, required []string) []string {
	if len(required) == 0 {
		return nil
	}
	var out []string
	for _, key := range required {
		if key == "" {
			continue
		}
		if _, ok := filters[key]; !ok {
			out = append(out, key)
		}
	}
	return out
}
