package queryparams

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
)

// Sort describes parsed sort fields in request order.
type Sort struct {
	Fields []SortField
}

// SortField describes one sortable field and direction.
type SortField struct {
	Name string
	Desc bool
}

// SortConfig configures allowed sort fields. Empty AllowedFields accepts any field.
type SortConfig struct {
	AllowedFields []string
}

// Filter describes one parsed filter expression.
type Filter struct {
	Field    string
	Operator FilterOperator
	Values   []string
}

// FilterField configures one allowed filter field.
type FilterField struct {
	Name      string
	Operators []FilterOperator
}

// FilterOperator describes a supported filter operator.
type FilterOperator string

const (
	// FilterOperatorEqual is the default equality operator.
	FilterOperatorEqual FilterOperator = "eq"
	// FilterOperatorNotEqual selects non-equal values.
	FilterOperatorNotEqual FilterOperator = "ne"
	// FilterOperatorGreaterThan selects values greater than the filter value.
	FilterOperatorGreaterThan FilterOperator = "gt"
	// FilterOperatorGreaterThanOrEqual selects values greater than or equal to the filter value.
	FilterOperatorGreaterThanOrEqual FilterOperator = "gte"
	// FilterOperatorLessThan selects values less than the filter value.
	FilterOperatorLessThan FilterOperator = "lt"
	// FilterOperatorLessThanOrEqual selects values less than or equal to the filter value.
	FilterOperatorLessThanOrEqual FilterOperator = "lte"
	// FilterOperatorIn selects values contained in the provided values.
	FilterOperatorIn FilterOperator = "in"
	// FilterOperatorContains selects values containing the provided value.
	FilterOperatorContains FilterOperator = "contains"
)

// FilterConfig configures allowed filter fields and operators. Empty Fields accepts any known operator.
type FilterConfig struct {
	Fields []FilterField
}

// FieldSet describes sparse fieldset parameters.
type FieldSet struct {
	Fields     []string
	ByResource map[string][]string
}

// IncludeSet describes included relationship names.
type IncludeSet struct {
	Includes []string
}

// ParseSort parses sort=name,-created_at from query values.
func ParseSort(values url.Values, config SortConfig) (Sort, error) {
	parsed, errs := ParseSortChecked(values, config)
	if len(errs) > 0 {
		return parsed, errs
	}
	return parsed, nil
}

// ParseSortChecked parses sort parameters and returns field errors for invalid input.
func ParseSortChecked(values url.Values, config SortConfig) (Sort, fielderrors.FieldErrors) {
	allowed := stringSet(config.AllowedFields)
	var out Sort
	var errs fielderrors.FieldErrors
	for _, token := range splitCSVValues(values["sort"]) {
		field := token
		desc := false
		if strings.HasPrefix(field, "-") {
			desc = true
			field = strings.TrimPrefix(field, "-")
		} else if strings.HasPrefix(field, "+") {
			field = strings.TrimPrefix(field, "+")
		}
		field = strings.TrimSpace(field)
		if field == "" {
			errs = append(errs, fieldError("sort", "invalid", "sort field is required"))
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[field]; !ok {
				errs = append(errs, fieldError("sort", "unknown_field", fmt.Sprintf("sort field %q is not allowed", field)))
				continue
			}
		}
		out.Fields = append(out.Fields, SortField{Name: field, Desc: desc})
	}
	return out, errs
}

// ParseFilters parses filter[field]=value and filter[field][operator]=value query values.
func ParseFilters(values url.Values, config FilterConfig) ([]Filter, error) {
	parsed, errs := ParseFiltersChecked(values, config)
	if len(errs) > 0 {
		return parsed, errs
	}
	return parsed, nil
}

// ParseFiltersChecked parses filters and returns field errors for invalid input.
func ParseFiltersChecked(values url.Values, config FilterConfig) ([]Filter, fielderrors.FieldErrors) {
	fieldConfig := configuredFilterFields(config.Fields)
	keys := make([]string, 0, len(values))
	for key := range values {
		if key == "filter" || strings.HasPrefix(key, "filter[") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	var out []Filter
	var errs fielderrors.FieldErrors
	for _, key := range keys {
		field, operator, ok := parseFilterKey(key)
		if !ok {
			errs = append(errs, fieldError(key, "invalid", "filter must use filter[field] or filter[field][operator] syntax"))
			continue
		}
		if !knownOperator(operator) {
			errs = append(errs, fieldError("filter."+field, "unknown_operator", fmt.Sprintf("filter operator %q is not supported", operator)))
			continue
		}
		if len(fieldConfig) > 0 {
			operators, ok := fieldConfig[field]
			if !ok {
				errs = append(errs, fieldError("filter."+field, "unknown_field", fmt.Sprintf("filter field %q is not allowed", field)))
				continue
			}
			if len(operators) == 0 {
				operators = map[FilterOperator]struct{}{FilterOperatorEqual: {}}
			}
			if _, ok := operators[operator]; !ok {
				errs = append(errs, fieldError("filter."+field, "unknown_operator", fmt.Sprintf("filter operator %q is not allowed for %q", operator, field)))
				continue
			}
		}
		out = append(out, Filter{Field: field, Operator: operator, Values: append([]string(nil), values[key]...)})
	}
	return out, errs
}

// ParseFields parses fields=id,name and fields[resource]=id,name query values.
func ParseFields(values url.Values) (FieldSet, error) {
	parsed, errs := ParseFieldsChecked(values)
	if len(errs) > 0 {
		return parsed, errs
	}
	return parsed, nil
}

// ParseFieldsChecked parses sparse fieldset parameters and returns field errors for invalid input.
func ParseFieldsChecked(values url.Values) (FieldSet, fielderrors.FieldErrors) {
	out := FieldSet{ByResource: map[string][]string{}}
	var errs fielderrors.FieldErrors
	for _, token := range splitCSVValues(values["fields"]) {
		if token == "" {
			errs = append(errs, fieldError("fields", "invalid", "field name is required"))
			continue
		}
		out.Fields = appendUnique(out.Fields, token)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.HasPrefix(key, "fields[") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		resource, ok := bracketName(key, "fields")
		if !ok || resource == "" {
			errs = append(errs, fieldError(key, "invalid", "fields must use fields[resource] syntax"))
			continue
		}
		for _, token := range splitCSVValues(values[key]) {
			if token == "" {
				errs = append(errs, fieldError(key, "invalid", "field name is required"))
				continue
			}
			out.ByResource[resource] = appendUnique(out.ByResource[resource], token)
		}
	}
	if len(out.ByResource) == 0 {
		out.ByResource = nil
	}
	return out, errs
}

// ParseIncludes parses include=owner,items query values.
func ParseIncludes(values url.Values) (IncludeSet, error) {
	parsed, errs := ParseIncludesChecked(values)
	if len(errs) > 0 {
		return parsed, errs
	}
	return parsed, nil
}

// ParseIncludesChecked parses include parameters and returns field errors for invalid input.
func ParseIncludesChecked(values url.Values) (IncludeSet, fielderrors.FieldErrors) {
	var out IncludeSet
	var errs fielderrors.FieldErrors
	for _, token := range splitCSVValues(values["include"]) {
		if token == "" {
			errs = append(errs, fieldError("include", "invalid", "include name is required"))
			continue
		}
		out.Includes = appendUnique(out.Includes, token)
	}
	return out, errs
}

func parseFilterKey(key string) (string, FilterOperator, bool) {
	if !strings.HasPrefix(key, "filter[") || !strings.HasSuffix(key, "]") {
		return "", "", false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(key, "filter["), "]")
	parts := strings.Split(inner, "][")
	if len(parts) == 1 && strings.TrimSpace(parts[0]) != "" {
		return strings.TrimSpace(parts[0]), FilterOperatorEqual, true
	}
	if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[0]), FilterOperator(strings.TrimSpace(parts[1])), true
	}
	return "", "", false
}

func configuredFilterFields(fields []FilterField) map[string]map[FilterOperator]struct{} {
	out := map[string]map[FilterOperator]struct{}{}
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			continue
		}
		operators := map[FilterOperator]struct{}{}
		for _, operator := range field.Operators {
			operators[operator] = struct{}{}
		}
		out[name] = operators
	}
	return out
}

func knownOperator(operator FilterOperator) bool {
	switch operator {
	case FilterOperatorEqual, FilterOperatorNotEqual, FilterOperatorGreaterThan,
		FilterOperatorGreaterThanOrEqual, FilterOperatorLessThan,
		FilterOperatorLessThanOrEqual, FilterOperatorIn, FilterOperatorContains:
		return true
	default:
		return false
	}
}

func bracketName(key, prefix string) (string, bool) {
	open := prefix + "["
	if !strings.HasPrefix(key, open) || !strings.HasSuffix(key, "]") {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(key, open), "]")
	if strings.Contains(name, "][") {
		return "", false
	}
	return strings.TrimSpace(name), true
}

func splitCSVValues(values []string) []string {
	var out []string
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			out = append(out, strings.TrimSpace(token))
		}
	}
	return out
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func stringSet(values []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func fieldError(field, code, message string) fielderrors.FieldError {
	return fielderrors.FieldError{Field: field, Code: code, Message: message}
}
