package fielderrors

import "strings"

// FieldError describes a single field-level validation failure.
type FieldError struct {
	Field   string `json:"field,omitempty" example:"postal_code"`
	Code    string `json:"code,omitempty" example:"required"`
	Message string `json:"message,omitempty" example:"postal_code is required"`
	// Public marks Message as explicitly safe to include in an API response.
	// It is deliberately not serialized: classification is an application
	// boundary decision, not a client-visible field.
	Public bool `json:"-"`
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

// Provider exposes field errors for callers that explicitly choose to consume
// them. Automatic HTTP error mapping exposes a Provider's messages only when
// every returned field error is explicitly marked Public.
type Provider interface {
	FieldErrors() FieldErrors
}

// FieldErrors returns itself to satisfy the Provider interface.
func (e FieldErrors) FieldErrors() FieldErrors {
	return e
}

// AllPublic reports whether every field error has an explicitly classified,
// non-empty public message suitable for an API response.
func (e FieldErrors) AllPublic() bool {
	if len(e) == 0 {
		return false
	}
	for _, entry := range e {
		if !entry.Public || strings.TrimSpace(entry.Message) == "" {
			return false
		}
	}
	return true
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
