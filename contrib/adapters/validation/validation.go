package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/aatuh/api-toolkit/v2/fielderrors"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// ValidationError represents a validation error with field-specific details.
//
//revive:disable-next-line:exported
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// FieldErrors converts the validation error to standard field errors.
func (e ValidationError) FieldErrors() fielderrors.FieldErrors {
	if strings.TrimSpace(e.Field) == "" && strings.TrimSpace(e.Message) == "" {
		return nil
	}
	return fielderrors.FieldErrors{
		{
			Field:   e.Field,
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
			Message: err.Message,
		})
	}
	return out
}

// playgroundValidator adapts go-playground/validator to the toolkit interface.
type playgroundValidator struct {
	validator *validator.Validate
}

// NewBasicValidator retains the old constructor name but now returns the
// go-playground-backed validator.
func NewBasicValidator() ports.Validator {
	return NewPlaygroundValidator()
}

// New returns the default validator implementation.
func New() ports.Validator {
	return NewPlaygroundValidator()
}

// NewPlaygroundValidator constructs a validator backed by github.com/go-playground/validator/v10.
func NewPlaygroundValidator() ports.Validator {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		tag := fld.Tag.Get("json")
		if tag == "" {
			return fld.Name
		}
		name := strings.Split(tag, ",")[0]
		if name == "-" || name == "" {
			return fld.Name
		}
		return name
	})
	return &playgroundValidator{validator: v}
}

func (p *playgroundValidator) Validate(ctx context.Context, value interface{}) error {
	if value == nil {
		return ValidationError{Message: "value is required"}
	}
	if !isStruct(value) {
		return ValidationError{Message: "value must be a struct or pointer to struct"}
	}
	return convertError(p.validator.StructCtx(normalizeContext(ctx), value))
}

func (p *playgroundValidator) ValidateStruct(ctx context.Context, obj interface{}) error {
	if obj == nil {
		return ValidationError{Message: "object is required"}
	}
	if !isStruct(obj) {
		return ValidationError{Message: "object must be a struct or pointer to struct"}
	}
	return convertError(p.validator.StructCtx(normalizeContext(ctx), obj))
}

func (p *playgroundValidator) ValidateField(ctx context.Context, obj interface{}, field string) error {
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
	resolvedField, err := resolveFieldPath(structType, field)
	if err != nil {
		return err
	}
	return convertError(p.validator.StructPartialCtx(normalizeContext(ctx), obj, resolvedField))
}

func isStruct(v interface{}) bool {
	_, ok := structTypeOf(v)
	return ok
}

func convertError(err error) error {
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return ValidationError{Message: invalidValidationMessage(invalid)}
	}
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		errs := ValidationErrors{}
		for _, fe := range ve {
			msg := buildMessage(fe)
			errs.Errors = append(errs.Errors, ValidationError{
				Field:   fe.Field(),
				Message: msg,
				Value:   fmt.Sprintf("%v", fe.Value()),
			})
		}
		if len(errs.Errors) == 1 {
			return errs.Errors[0]
		}
		return errs
	}
	return err
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

		resolved = append(resolved, field.Name)
		currentType = indirectType(field.Type)
	}

	return strings.Join(resolved, "."), nil
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

func invalidValidationMessage(err *validator.InvalidValidationError) string {
	if err == nil || err.Type == nil {
		return "invalid validation target"
	}
	return fmt.Sprintf("invalid validation target: %s", err.Type.String())
}

func buildMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "len":
		return fmt.Sprintf("must be %s in length", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of [%s]", fe.Param())
	default:
		if fe.Param() != "" {
			return fmt.Sprintf("failed '%s'=%s validation", fe.Tag(), fe.Param())
		}
		return fmt.Sprintf("failed '%s' validation", fe.Tag())
	}
}
