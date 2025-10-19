package endpoints

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ListQuery captures pagination, search, and filter inputs from a request.
type ListQuery struct {
	Limit   int
	Offset  int
	Search  string
	Filters Filters
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

// ListQueryConfig configures parsing behaviour for list endpoints.
type ListQueryConfig struct {
	DefaultLimit   int
	MaxLimit       int
	AllowedFilters []string
	Required       []string
	SearchParam    string
}

// ParseListQuery parses pagination and filters from the HTTP request according to cfg.
func ParseListQuery(r *http.Request, cfg ListQueryConfig) ListQuery {
	values := r.URL.Query()
	limit := clampInt(parseInt(values.Get("limit"), cfg.DefaultLimit), 1, cfg.MaxLimit)
	offset := parseOffset(values.Get("offset"))

	searchKey := cfg.SearchParam
	if strings.TrimSpace(searchKey) == "" {
		searchKey = "search"
	}
	search := strings.TrimSpace(values.Get(searchKey))

	allowed := make(map[string]struct{}, len(cfg.AllowedFilters))
	for _, f := range cfg.AllowedFilters {
		if f == "" {
			continue
		}
		allowed[f] = struct{}{}
	}

	filters := make(Filters)
	for key, vals := range values {
		if key == "limit" || key == "offset" || key == searchKey {
			continue
		}
		if name, ok := parseFilterKey(key); ok {
			if allowFilter(name, allowed) {
				addFilter(filters, name, vals)
			}
			continue
		}
		if allowFilter(key, allowed) {
			addFilter(filters, key, vals)
		}
	}

	missing := requiredMissing(filters, cfg.Required)

	return ListQuery{
		Limit:   limit,
		Offset:  offset,
		Search:  search,
		Filters: filters,
		Raw:     values,
		missing: missing,
	}
}

// ListMeta captures pagination metadata for responses.
type ListMeta struct {
	Total   int                 `json:"total"`
	Count   int                 `json:"count"`
	Limit   int                 `json:"limit"`
	Offset  int                 `json:"offset"`
	Filters map[string][]string `json:"filters,omitempty"`
	Search  string              `json:"search,omitempty"`
}

// ListResponse wraps list results with metadata.
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

func parseInt(val string, def int) int {
	if strings.TrimSpace(val) == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return n
}

func parseOffset(val string) int {
	if strings.TrimSpace(val) == "" {
		return 0
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func clampInt(n, min, max int) int {
	if min > 0 && n < min {
		n = min
	}
	if max > 0 && n > max {
		n = max
	}
	return n
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
