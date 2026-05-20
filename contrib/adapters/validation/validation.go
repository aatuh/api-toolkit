package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	validatev3 "github.com/aatuh/validate/v3"
	validateerrors "github.com/aatuh/validate/v3/errors"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
	"github.com/aatuh/api-toolkit/v3/ports"
)

// ValidationError represents a validation error with field-specific details.
//
//revive:disable-next-line:exported
type ValidationError struct {
	Field   string `json:"field"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	Param   any    `json:"param,omitempty"`
	// Deprecated: Value is retained for source compatibility and is no longer
	// populated by this adapter to avoid exposing raw submitted values.
	Value string `json:"value,omitempty"`
}

func (e ValidationError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = strings.TrimSpace(e.Code)
	}
	if message == "" {
		message = "validation failed"
	}
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, message)
	}
	return message
}

// FieldErrors converts the validation error to standard field errors.
func (e ValidationError) FieldErrors() fielderrors.FieldErrors {
	if strings.TrimSpace(e.Field) == "" && strings.TrimSpace(e.Message) == "" && strings.TrimSpace(e.Code) == "" {
		return nil
	}
	return fielderrors.FieldErrors{
		{
			Field:   e.Field,
			Code:    e.Code,
			Message: e.Message,
		},
	}
}

// ValidationErrors represents multiple validation errors.
//
//revive:disable-next-line:exported
type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (e ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	msgs := make([]string, 0, len(e.Errors))
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// FieldErrors converts the validation errors to standard field errors.
func (e ValidationErrors) FieldErrors() fielderrors.FieldErrors {
	if len(e.Errors) == 0 {
		return nil
	}
	out := make(fielderrors.FieldErrors, 0, len(e.Errors))
	for _, err := range e.Errors {
		out = append(out, fielderrors.FieldError{
			Field:   err.Field,
			Code:    err.Code,
			Message: err.Message,
		})
	}
	return out
}

// validateValidator adapts github.com/aatuh/validate/v3 to the toolkit interface.
type validateValidator struct {
	validator *validatev3.Validate
}

// NewBasicValidator retains the old constructor name and returns the default
// validate-backed validator.
func NewBasicValidator() ports.Validator {
	return NewValidateValidator()
}

// New returns the default validator implementation.
func New() ports.Validator {
	return NewValidateValidator()
}

// NewValidateValidator constructs a validator backed by github.com/aatuh/validate/v3.
func NewValidateValidator() ports.Validator {
	return &validateValidator{validator: validatev3.New()}
}

// NewPlaygroundValidator constructs the default validate-backed validator.
//
// Deprecated: use NewValidateValidator. This constructor no longer uses
// github.com/go-playground/validator/v10.
func NewPlaygroundValidator() ports.Validator {
	return NewValidateValidator()
}

func (v *validateValidator) Validate(ctx context.Context, value interface{}) error {
	if value == nil {
		return ValidationError{Message: "value is required"}
	}
	if !isStruct(value) {
		return ValidationError{Message: "value must be a struct or pointer to struct"}
	}
	return v.validateStruct(ctx, value)
}

func (v *validateValidator) ValidateStruct(ctx context.Context, obj interface{}) error {
	if obj == nil {
		return ValidationError{Message: "object is required"}
	}
	if !isStruct(obj) {
		return ValidationError{Message: "object must be a struct or pointer to struct"}
	}
	return v.validateStruct(ctx, obj)
}

func (v *validateValidator) ValidateField(ctx context.Context, obj interface{}, field string) error {
	if obj == nil {
		return ValidationError{Message: "object is required"}
	}
	field = strings.TrimSpace(field)
	if field == "" {
		return ValidationError{Message: "field name is required"}
	}
	structType, ok := structTypeOf(obj)
	if !ok {
		return ValidationError{Message: "object must be a struct or pointer to struct"}
	}
	resolvedPath, err := resolveFieldPath(structType, field)
	if err != nil {
		return err
	}

	err = validatev3.ValidateStructContextWithOpts(
		normalizeContext(ctx),
		v.validator,
		obj,
		validationOptions(),
	)
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var validateErrors validatev3.Errors
	if errors.As(err, &validateErrors) {
		filtered := filterValidateErrors(validateErrors, resolvedPath)
		return convertValidateErrors(filtered)
	}
	return ValidationError{Field: resolvedPath, Message: "field validation failed"}
}

func (v *validateValidator) validateStruct(ctx context.Context, obj interface{}) error {
	return convertError(validatev3.ValidateStructContextWithOpts(
		normalizeContext(ctx),
		v.validator,
		obj,
		validationOptions(),
	))
}

func validationOptions() validatev3.ValidateOpts {
	return validatev3.ValidateOpts{FieldNameFunc: validatev3.JSONFieldName}
}

func isStruct(v interface{}) bool {
	_, ok := structTypeOf(v)
	return ok
}

func convertError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var ve validatev3.Errors
	if errors.As(err, &ve) {
		return convertValidateErrors(ve)
	}
	return ValidationError{Message: "validation failed"}
}

func convertValidateErrors(ve validatev3.Errors) error {
	if len(ve) == 0 {
		return nil
	}
	errs := ValidationErrors{Errors: make([]ValidationError, 0, len(ve))}
	for _, fe := range ve {
		errs.Errors = append(errs.Errors, convertValidateError(fe))
	}
	if len(errs.Errors) == 1 {
		return errs.Errors[0]
	}
	return errs
}

func convertValidateError(fe validateerrors.FieldError) ValidationError {
	message := strings.TrimSpace(fe.Msg)
	if message == "" {
		message = strings.TrimSpace(fe.Code)
	}
	if message == "" {
		message = "validation failed"
	}
	return ValidationError{
		Field:   fe.Path,
		Code:    fe.Code,
		Message: message,
		Param:   fe.Param,
	}
}

var validatorTimeType = reflect.TypeOf(time.Time{})

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func structTypeOf(v interface{}) (reflect.Type, bool) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil, false
	}
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, false
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct || rv.Type().ConvertibleTo(validatorTimeType) {
		return nil, false
	}
	return rv.Type(), true
}

func resolveFieldPath(rootType reflect.Type, fieldPath string) (string, error) {
	parts := strings.Split(fieldPath, ".")
	resolved := make([]string, 0, len(parts))
	currentType := rootType

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", ValidationError{
				Field:   strings.TrimSpace(fieldPath),
				Message: "field path is invalid",
			}
		}

		field, ok := resolveField(currentType, part)
		if !ok {
			return "", ValidationError{
				Field:   strings.TrimSpace(fieldPath),
				Message: "field is not valid for validation",
			}
		}

		resolved = append(resolved, externalFieldName(field))
		currentType = indirectType(field.Type)
	}

	return strings.Join(resolved, "."), nil
}

func filterValidateErrors(errs validatev3.Errors, fieldPath string) validatev3.Errors {
	if len(errs) == 0 {
		return nil
	}
	filtered := make(validatev3.Errors, 0, len(errs))
	for _, err := range errs {
		if matchesFieldPath(err.Path, fieldPath) {
			filtered = append(filtered, err)
		}
	}
	return filtered
}

func matchesFieldPath(got, want string) bool {
	return got == want || strings.HasPrefix(got, want+".") || strings.HasPrefix(got, want+"[")
}

func resolveField(structType reflect.Type, name string) (reflect.StructField, bool) {
	structType = indirectType(structType)
	if structType.Kind() != reflect.Struct {
		return reflect.StructField{}, false
	}
	if field, ok := structType.FieldByName(name); ok && field.PkgPath == "" {
		return field, true
	}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if jsonName := jsonFieldName(field); jsonName == name {
			return field, true
		}
	}
	return reflect.StructField{}, false
}

func indirectType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func jsonFieldName(field reflect.StructField) string {
	tag := strings.TrimSpace(field.Tag.Get("json"))
	if tag == "" {
		return ""
	}
	name := strings.Split(tag, ",")[0]
	if name == "" || name == "-" {
		return ""
	}
	return name
}

func externalFieldName(field reflect.StructField) string {
	if name := jsonFieldName(field); name != "" {
		return name
	}
	return field.Name
}
