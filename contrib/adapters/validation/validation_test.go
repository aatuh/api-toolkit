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

func TestValidateRejectsUnsupportedTargets(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{name: "scalar", value: 42},
		{name: "slice", value: []string{"admin"}},
		{name: "map", value: map[string]string{"role": "admin"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := New()

			err := v.Validate(context.Background(), tt.value)
			var got ValidationError
			if !errors.As(err, &got) {
				t.Fatalf("expected ValidationError, got %T", err)
			}
			if got.Message != "value must be a struct or pointer to struct" {
				t.Fatalf("message = %q, want %q", got.Message, "value must be a struct or pointer to struct")
			}
		})
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

func TestValidateStructAcceptsPointerToStruct(t *testing.T) {
	v := New()

	err := v.ValidateStruct(context.Background(), &validationFixture{})
	var got ValidationErrors
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if len(got.Errors) != 3 {
		t.Fatalf("validation errors = %d, want 3", len(got.Errors))
	}
}

func TestValidateStructRejectsUnsupportedTarget(t *testing.T) {
	v := New()

	err := v.ValidateStruct(context.Background(), []string{"admin"})
	var got ValidationError
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if got.Message != "object must be a struct or pointer to struct" {
		t.Fatalf("message = %q, want %q", got.Message, "object must be a struct or pointer to struct")
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

func TestValidateFieldAcceptsJSONFieldName(t *testing.T) {
	v := NewPlaygroundValidator()

	err := v.ValidateField(context.Background(), validationFixture{}, "email")
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

func TestValidateFieldRejectsUnsupportedTarget(t *testing.T) {
	v := NewPlaygroundValidator()

	err := v.ValidateField(context.Background(), []string{"admin"}, "role")
	var got ValidationError
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if got.Message != "object must be a struct or pointer to struct" {
		t.Fatalf("message = %q, want %q", got.Message, "object must be a struct or pointer to struct")
	}
}

func TestValidateFieldRejectsUnknownField(t *testing.T) {
	v := NewPlaygroundValidator()

	err := v.ValidateField(context.Background(), validationFixture{}, "missing")
	var got ValidationError
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if got.Field != "missing" {
		t.Fatalf("field = %q, want missing", got.Field)
	}
	if got.Message != "field is not valid for validation" {
		t.Fatalf("message = %q, want %q", got.Message, "field is not valid for validation")
	}
}

func TestValidationMethodsAcceptNilContext(t *testing.T) {
	v := NewPlaygroundValidator()

	assertNoPanic(t, "Validate nil context", func() {
		err := v.Validate(nilContext(), validationFixture{})
		var got ValidationErrors
		if !errors.As(err, &got) {
			t.Fatalf("expected ValidationErrors, got %T", err)
		}
	})

	assertNoPanic(t, "ValidateStruct nil context", func() {
		err := v.ValidateStruct(nilContext(), validationFixture{})
		var got ValidationErrors
		if !errors.As(err, &got) {
			t.Fatalf("expected ValidationErrors, got %T", err)
		}
	})

	assertNoPanic(t, "ValidateField nil context", func() {
		err := v.ValidateField(nilContext(), validationFixture{}, "email")
		var got ValidationError
		if !errors.As(err, &got) {
			t.Fatalf("expected ValidationError, got %T", err)
		}
	})
}

func assertNoPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("%s panicked: %v", name, recovered)
		}
	}()
	fn()
}

func nilContext() context.Context {
	return nil
}

type validationFixture struct {
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"min=18"`
	Role  string `json:"role" validate:"oneof=admin user"`
}
