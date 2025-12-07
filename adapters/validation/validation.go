package validation

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/go-playground/validator/v10"
)

// ValidationError represents a validation error with field-specific details.
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

// ValidationErrors represents multiple validation errors.
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
	if isStruct(value) {
		return convertError(p.validator.StructCtx(ctx, value))
	}
	// Non-struct validations are not supported via generic Validate; consider using ValidateField.
	return nil
}

func (p *playgroundValidator) ValidateStruct(ctx context.Context, obj interface{}) error {
	if obj == nil {
		return ValidationError{Message: "object is required"}
	}
	return convertError(p.validator.StructCtx(ctx, obj))
}

func (p *playgroundValidator) ValidateField(ctx context.Context, obj interface{}, field string) error {
	if obj == nil {
		return ValidationError{Message: "object is required"}
	}
	if strings.TrimSpace(field) == "" {
		return ValidationError{Message: "field name is required"}
	}
	return convertError(p.validator.StructPartialCtx(ctx, obj, field))
}

func isStruct(v interface{}) bool {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return false
	}
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	return rv.IsValid() && rv.Kind() == reflect.Struct
}

func convertError(err error) error {
	if err == nil {
		return nil
	}
	if ve, ok := err.(validator.ValidationErrors); ok {
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
