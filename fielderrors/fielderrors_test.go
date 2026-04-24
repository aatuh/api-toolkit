package fielderrors

import (
	"reflect"
	"testing"
)

func TestFieldErrorError(t *testing.T) {
	tests := []struct {
		name string
		err  FieldError
		want string
	}{
		{
			name: "message wins",
			err:  FieldError{Field: "email", Message: "email is invalid"},
			want: "email is invalid",
		},
		{
			name: "field fallback",
			err:  FieldError{Field: "email"},
			want: "email is invalid",
		},
		{
			name: "generic fallback",
			err:  FieldError{},
			want: "validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFieldErrorsProviderAndMap(t *testing.T) {
	errs := FieldErrors{
		{Field: "email", Message: "email is invalid"},
		{Field: "name", Message: "name is required"},
		{Field: "ignored"},
		{Message: "also ignored"},
	}

	if got := errs.Error(); got != "email is invalid" {
		t.Fatalf("Error() = %q, want email is invalid", got)
	}
	var provider Provider = errs
	if !reflect.DeepEqual(provider.FieldErrors(), errs) {
		t.Fatalf("FieldErrors() = %#v, want %#v", provider.FieldErrors(), errs)
	}
	wantMap := map[string]string{
		"email": "email is invalid",
		"name":  "name is required",
	}
	if got := errs.ToMap(); !reflect.DeepEqual(got, wantMap) {
		t.Fatalf("ToMap() = %#v, want %#v", got, wantMap)
	}
}

func TestFieldErrorsEmptyCollection(t *testing.T) {
	var errs FieldErrors
	if got := errs.Error(); got != "validation failed" {
		t.Fatalf("Error() = %q, want validation failed", got)
	}
	if got := errs.ToMap(); got != nil {
		t.Fatalf("ToMap() = %#v, want nil", got)
	}
}
