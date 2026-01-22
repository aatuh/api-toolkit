package fielderrors

import "strings"

// FieldError describes a single field-level validation failure.
type FieldError struct {
	Field   string `json:"field,omitempty" example:"postal_code"`
	Code    string `json:"code,omitempty" example:"required"`
	Message string `json:"message,omitempty" example:"postal_code is required"`
}

func (e FieldError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if strings.TrimSpace(e.Field) != "" {
		return e.Field + " is invalid"
	}
	return "validation failed"
}

// FieldErrors is a collection of field-level validation errors.
type FieldErrors []FieldError

func (e FieldErrors) Error() string {
	if len(e) == 0 {
		return "validation failed"
	}
	return e[0].Error()
}

// Provider exposes field errors for downstream mappers.
type Provider interface {
	FieldErrors() FieldErrors
}

// FieldErrors returns itself to satisfy the Provider interface.
func (e FieldErrors) FieldErrors() FieldErrors {
	return e
}

// ToMap converts field errors into a field->message map.
func (e FieldErrors) ToMap() map[string]string {
	out := make(map[string]string, len(e))
	for _, entry := range e {
		if entry.Field == "" || entry.Message == "" {
			continue
		}
		out[entry.Field] = entry.Message
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
