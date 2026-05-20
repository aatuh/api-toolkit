package validation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
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
		{Field: "email", Code: "required", Message: "value is required"},
		{Field: "age", Code: "int.min", Message: "minimum value is 18"},
		{Field: "role", Code: "string.oneof", Message: "must be one of: admin, user"},
		{Field: "profile.handle", Code: "required", Message: "value is required"},
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
	if len(got.Errors) != 4 {
		t.Fatalf("validation errors = %d, want 4", len(got.Errors))
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
	if got.Code != "required" {
		t.Fatalf("code = %q, want required", got.Code)
	}
	if got.Message != "value is required" {
		t.Fatalf("message = %q, want %q", got.Message, "value is required")
	}
}

func TestValidateFieldAcceptsJSONFieldName(t *testing.T) {
	v := NewValidateValidator()

	err := v.ValidateField(context.Background(), validationFixture{}, "email")
	var got ValidationError
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if got.Field != "email" {
		t.Fatalf("field = %q, want email", got.Field)
	}
	if got.Code != "required" {
		t.Fatalf("code = %q, want required", got.Code)
	}
	if got.Message != "value is required" {
		t.Fatalf("message = %q, want %q", got.Message, "value is required")
	}
}

func TestValidateFieldAcceptsNestedJSONFieldPath(t *testing.T) {
	v := New()

	err := v.ValidateField(context.Background(), validationFixture{}, "profile.handle")
	var got ValidationError
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if got.Field != "profile.handle" {
		t.Fatalf("field = %q, want profile.handle", got.Field)
	}
	if got.Code != "required" {
		t.Fatalf("code = %q, want required", got.Code)
	}
}

func TestValidateFieldFiltersUnrelatedErrors(t *testing.T) {
	v := New()

	err := v.ValidateField(context.Background(), validationFixture{Age: 21}, "Age")
	if err != nil {
		t.Fatalf("ValidateField valid field returned error: %v", err)
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
	v := New()

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

func TestNewPlaygroundValidatorIsDeprecatedValidateBackedAlias(t *testing.T) {
	v := NewPlaygroundValidator()

	err := v.ValidateStruct(context.Background(), validationFixture{})
	var got ValidationErrors
	if !errors.As(err, &got) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if len(got.Errors) == 0 || got.Errors[0].Code != "required" {
		t.Fatalf("first validation error = %#v, want validate-backed required code", got.Errors)
	}
}

func TestValidationErrorsDoNotExposeRawValues(t *testing.T) {
	const secretEmail = "secret-person@example.invalid"
	v := New()

	err := v.ValidateStruct(context.Background(), validationFixture{
		Email: secretEmail,
		Age:   21,
		Role:  "owner",
		Profile: validationProfile{
			Handle: "aatu",
		},
	})
	if err == nil {
		t.Fatal("expected invalid role")
	}
	if strings.Contains(err.Error(), secretEmail) {
		t.Fatalf("validation error exposed raw email %q in %q", secretEmail, err.Error())
	}
	var got ValidationError
	if !errors.As(err, &got) {
		t.Fatalf("expected single ValidationError, got %T", err)
	}
	if got.Code != "string.oneof" {
		t.Fatalf("code = %q, want string.oneof", got.Code)
	}
	if got.Value != "" {
		t.Fatalf("value = %q, want empty deprecated value field", got.Value)
	}
}

func TestValidationMethodsPropagateCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v := New()

	err := v.ValidateStruct(ctx, validationFixture{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ValidateStruct canceled error = %v, want context.Canceled", err)
	}
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
	Email   string            `json:"email" validate:"string;required;email"`
	Age     int               `json:"age" validate:"int;min=18"`
	Role    string            `json:"role" validate:"string;oneof=admin,user"`
	Profile validationProfile `json:"profile"`
}

type validationProfile struct {
	Handle string `json:"handle" validate:"string;required"`
}
