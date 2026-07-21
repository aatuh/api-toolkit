package binding

import (
	"bytes"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/aatuh/api-toolkit/v4/fielderrors"
	"github.com/aatuh/api-toolkit/v4/httpx"
)

// RequiredMode controls how fields tagged required:"true" are evaluated.
type RequiredMode string

const (
	// RequiredModeNonZero preserves the v4 default: a required field must have
	// a non-zero decoded value.
	RequiredModeNonZero RequiredMode = "nonzero"
	// RequiredModePresent requires an input member to be present, allowing
	// explicit zero values such as false and 0. For JSON, an explicit null is
	// present; use an application-level nullable constraint when null is invalid.
	RequiredModePresent RequiredMode = "present"
)

// JSONConfig configures JSON request body decoding.
type JSONConfig struct {
	MaxBytes           int64
	AllowUnknownFields bool
	RequireObject      bool
	// RequiredMode overrides required-field validation. Its zero value preserves
	// RequiredModeNonZero compatibility.
	RequiredMode RequiredMode
}

// QueryConfig configures query decoding.
type QueryConfig struct {
	// RequiredMode overrides required-field validation. Its zero value preserves
	// RequiredModeNonZero compatibility.
	RequiredMode RequiredMode
}

// PathConfig configures path decoding.
type PathConfig struct {
	Param func(r *http.Request, name string) string
	// ParamPresent reports whether name was present in the route parameter
	// source. It is required to distinguish an empty path parameter from an
	// absent one when using RequiredModePresent.
	ParamPresent func(r *http.Request, name string) bool
	// RequiredMode overrides required-field validation. Its zero value preserves
	// RequiredModeNonZero compatibility.
	RequiredMode RequiredMode
}

// DecodeJSON decodes a JSON body into T.
func DecodeJSON[T any](r *http.Request, cfg JSONConfig) (T, error) {
	var out T
	if r == nil || r.Body == nil {
		return out, fieldError("body", "required", "request body is required")
	}
	reader := io.Reader(r.Body)
	if cfg.MaxBytes > 0 {
		reader = io.LimitReader(reader, cfg.MaxBytes+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return out, fieldError("body", "read_failed", "request body could not be read")
	}
	if cfg.MaxBytes > 0 && int64(len(body)) > cfg.MaxBytes {
		return out, fieldError("body", "too_large", "request body exceeds maximum size")
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return out, fieldError("body", "required", "request body is required")
	}
	if shouldRequireObject[T](cfg) && trimmed[0] != '{' {
		return out, fieldError("body", "invalid", "request body must be a JSON object")
	}
	var present map[string]struct{}
	if requiredMode(cfg.RequiredMode) == RequiredModePresent && shouldRequireObject[T](cfg) {
		var err error
		present, err = jsonMemberPresence(trimmed)
		if err != nil {
			return out, fieldError("body", "invalid_json", "request body contains duplicate JSON members")
		}
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	if !cfg.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(&out); err != nil {
		return out, fieldError("body", "invalid_json", jsonDecodeMessage(err))
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return out, fieldError("body", "invalid_json", "request body must contain a single JSON value")
	}
	if errs := requiredFieldErrors(out, "json", cfg.RequiredMode, present); len(errs) > 0 {
		return out, errs
	}
	return out, nil
}

// DecodeQuery decodes query parameters into T.
func DecodeQuery[T any](r *http.Request, cfg QueryConfig) (T, error) {
	var out T
	if r == nil || r.URL == nil {
		return out, fieldError("query", "required", "query values are required")
	}
	if err := decodeValuesInto(&out, r.URL.Query(), "query", nil, cfg.RequiredMode); err != nil {
		return out, err
	}
	return out, nil
}

// DecodePath decodes route path parameters into T.
func DecodePath[T any](r *http.Request, cfg PathConfig) (T, error) {
	var out T
	if cfg.Param == nil {
		return out, fieldError("path", "missing_param_resolver", "path parameter resolver is required")
	}
	values := url.Values{}
	typ := indirectType(reflect.TypeOf(out))
	if typ == nil || typ.Kind() != reflect.Struct {
		return out, fieldError("path", "invalid_target", "path target must be a struct")
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := externalName(field, "path")
		if name == "-" || name == "" {
			continue
		}
		value := cfg.Param(r, name)
		if cfg.ParamPresent != nil {
			if cfg.ParamPresent(r, name) {
				values.Set(name, value)
			}
		} else if strings.TrimSpace(value) != "" {
			values.Set(name, value)
		}
	}
	if err := decodeValuesInto(&out, values, "path", nil, cfg.RequiredMode); err != nil {
		return out, err
	}
	return out, nil
}

// ValidationProblem maps validation errors to a Problem Details payload.
func ValidationProblem(err error) httpx.Problem {
	p := httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeValidation),
		Title:  http.StatusText(http.StatusBadRequest),
		Detail: "validation failed",
	}
	var fields fielderrors.FieldErrors
	if errors.As(err, &fields) && len(fields) > 0 {
		return httpx.WithFieldErrors(p, fields)
	}
	var publicErr PublicError
	if errors.As(err, &publicErr) {
		if detail := strings.TrimSpace(publicErr.PublicMessage()); detail != "" {
			p.Detail = detail
		}
	}
	return p
}

// WriteValidationProblem writes validation errors as RFC 9457 Problem Details.
func WriteValidationProblem(w http.ResponseWriter, err error) {
	httpx.WriteProblem(w, http.StatusBadRequest, ValidationProblem(err))
}

func decodeValuesInto(dst any, values url.Values, tag string, defaults map[string]string, mode RequiredMode) error {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return fieldError(tag, "invalid_target", tag+" target must be a non-nil pointer")
	}
	elem := indirectValue(v)
	if !elem.IsValid() || elem.Kind() != reflect.Struct {
		return fieldError(tag, "invalid_target", tag+" target must be a struct")
	}
	typ := elem.Type()
	var errs fielderrors.FieldErrors
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := externalName(field, tag)
		if name == "-" || name == "" {
			continue
		}
		raw := values[name]
		if len(raw) == 0 && defaults != nil {
			if value := strings.TrimSpace(defaults[name]); value != "" {
				raw = []string{value}
			}
		}
		present := len(raw) > 0
		if !present || allEmpty(raw) {
			if required(field) && (!present || requiredMode(mode) != RequiredModePresent) {
				errs = append(errs, fieldError(name, "required", name+" is required")...)
			}
			if !present || requiredMode(mode) != RequiredModePresent || !required(field) {
				continue
			}
		}
		if err := setField(elem.Field(i), raw); err != nil {
			errs = append(errs, fieldError(name, "invalid", err.Error())...)
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func setField(field reflect.Value, raw []string) error {
	if field.Kind() == reflect.Pointer {
		value := reflect.New(field.Type().Elem())
		if err := setField(value.Elem(), raw); err != nil {
			return err
		}
		field.Set(value)
		return nil
	}
	if field.CanAddr() {
		if unmarshaler, ok := field.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return unmarshaler.UnmarshalText([]byte(raw[0]))
		}
	}
	if field.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(field.Type(), 0, len(raw))
		for _, value := range raw {
			if strings.TrimSpace(value) == "" {
				continue
			}
			elem := reflect.New(field.Type().Elem()).Elem()
			if err := setScalar(elem, value); err != nil {
				return err
			}
			slice = reflect.Append(slice, elem)
		}
		field.Set(slice)
		return nil
	}
	return setScalar(field, raw[0])
}

func setScalar(field reflect.Value, raw string) error {
	raw = strings.TrimSpace(raw)
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("must be a boolean")
		}
		field.SetBool(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("must be an integer")
		}
		field.SetInt(value)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value, err := strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("must be an unsigned integer")
		}
		field.SetUint(value)
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		field.SetFloat(value)
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128,
		reflect.Array, reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Pointer, reflect.Slice, reflect.Struct,
		reflect.UnsafePointer:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}

func shouldRequireObject[T any](cfg JSONConfig) bool {
	if cfg.RequireObject {
		return true
	}
	var zero T
	typ := indirectType(reflect.TypeOf(zero))
	return typ != nil && typ.Kind() == reflect.Struct
}

func requiredFieldErrors(value any, tag string, mode RequiredMode, present map[string]struct{}) fielderrors.FieldErrors {
	v := indirectValue(reflect.ValueOf(value))
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return nil
	}
	typ := v.Type()
	var errs fielderrors.FieldErrors
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" || !required(field) {
			continue
		}
		name := externalName(field, tag)
		if name == "-" || name == "" {
			continue
		}
		if requiredMode(mode) == RequiredModePresent {
			if _, ok := present[name]; !ok {
				errs = append(errs, fieldError(name, "required", name+" is required")...)
			}
			continue
		}
		if isZero(v.Field(i)) {
			errs = append(errs, fieldError(name, "required", name+" is required")...)
		}
	}
	return errs
}

func requiredMode(mode RequiredMode) RequiredMode {
	if mode == RequiredModePresent {
		return RequiredModePresent
	}
	return RequiredModeNonZero
}

func jsonMemberPresence(body []byte) (map[string]struct{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("expected JSON object")
	}
	present := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("expected JSON member name")
		}
		if _, duplicate := present[name]; duplicate {
			return nil, fmt.Errorf("duplicate JSON member")
		}
		present[name] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return present, nil
}

func externalName(field reflect.StructField, tag string) string {
	value := field.Tag.Get(tag)
	if value == "" && tag == "json" {
		value = field.Tag.Get("json")
	}
	if value == "" {
		return lowerFirst(field.Name)
	}
	name := strings.Split(value, ",")[0]
	if name == "" {
		return lowerFirst(field.Name)
	}
	return name
}

func required(field reflect.StructField) bool {
	return strings.EqualFold(field.Tag.Get("required"), "true") ||
		strings.Contains(field.Tag.Get("binding"), "required")
}

func fieldError(field, code, message string) fielderrors.FieldErrors {
	return fielderrors.FieldErrors{{Field: field, Code: code, Message: message}}
}

func allEmpty(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func indirectType(typ reflect.Type) reflect.Type {
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func indirectValue(value reflect.Value) reflect.Value {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func isZero(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	return value.IsZero()
}

func lowerFirst(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToLower(value[:1]) + value[1:]
}

func jsonDecodeMessage(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		return typeErr.Field + " has the wrong JSON type"
	}
	if strings.Contains(err.Error(), "unknown field") {
		return err.Error()
	}
	return "request body must be valid JSON"
}
