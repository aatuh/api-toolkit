package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/aatuh/api-toolkit/v2/fielderrors"
)

func TestValidateNilValueReturnsValidationError(t *testing.T) {
	v := New()

	err := v.Validate(context.Background(), nil)
	var got ValidationError
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if got.Message != "value is required" {
		t.Fatalf("message = %q, want %q", got.Message, "value is required")
	}
}

func TestValidateStructUsesJSONFieldNames(t *testing.T) {
	v := New()

	err := v.ValidateStruct(context.Background(), validationFixture{})
	var got ValidationErrors
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	want := fielderrors.FieldErrors{
		{Field: "email", Message: "is required"},
		{Field: "age", Message: "must be at least 18"},
		{Field: "role", Message: "must be one of [admin user]"},
	}
	if diff := got.FieldErrors(); len(diff) != len(want) {
		t.Fatalf("FieldErrors() len = %d, want %d", len(diff), len(want))
	}
	for i, fieldErr := range want {
		if gotField := got.FieldErrors()[i]; gotField != fieldErr {
			t.Fatalf("FieldErrors()[%d] = %#v, want %#v", i, gotField, fieldErr)
		}
	}
}

func TestValidateFieldReturnsSingleFieldError(t *testing.T) {
	v := NewBasicValidator()

	err := v.ValidateField(context.Background(), validationFixture{}, "Email")
	var got ValidationError
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if got.Field != "email" {
		t.Fatalf("field = %q, want email", got.Field)
	}
	if got.Message != "is required" {
		t.Fatalf("message = %q, want %q", got.Message, "is required")
	}
}

func TestValidateFieldRequiresInputAndFieldName(t *testing.T) {
	v := NewPlaygroundValidator()

	err := v.ValidateField(context.Background(), nil, "Email")
	var got ValidationError
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationError for nil object, got %T", err)
	}
	if got.Message != "object is required" {
		t.Fatalf("message = %q, want %q", got.Message, "object is required")
	}

	err = v.ValidateField(context.Background(), validationFixture{}, "")
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationError for empty field name, got %T", err)
	}
	if got.Message != "field name is required" {
		t.Fatalf("message = %q, want %q", got.Message, "field name is required")
	}
}

type validationFixture struct {
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"min=18"`
	Role  string `json:"role" validate:"oneof=admin user"`
}
