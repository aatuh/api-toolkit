package httpx

import (
	"net/http"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
)

// ValidationErrorsKey is the canonical extension key for field errors.
const ValidationErrorsKey = "validation"

// ValidationErrors wraps field-level validation errors for Problem extensions.
type ValidationErrors struct {
	Fields fielderrors.FieldErrors `json:"fields,omitempty"`
}

// WithValidationErrors attaches canonical validation errors to a problem payload.
func WithValidationErrors(p Problem, errs fielderrors.FieldErrors) Problem {
	if len(errs) == 0 {
		return p
	}
	if p.Type == "" {
		p.Type = DefaultTypeURI(TypeValidation)
	}
	p.With(ValidationErrorsKey, ValidationErrors{Fields: errs})
	return p
}

// WithFieldErrors attaches field-level errors to a problem payload.
func WithFieldErrors(p Problem, errs fielderrors.FieldErrors) Problem {
	if len(errs) == 0 {
		return p
	}
	p = WithValidationErrors(p, errs)
	// Legacy fields retained for compatibility.
	p.With("errors", errs)
	if fieldMap := errs.ToMap(); fieldMap != nil {
		p.With("field_errors", fieldMap)
	}
	return p
}

// WriteProblemWithFieldErrors writes problem details with field errors attached.
func WriteProblemWithFieldErrors(w http.ResponseWriter, status int, p Problem, errs fielderrors.FieldErrors) {
	WriteProblem(w, status, WithFieldErrors(p, errs))
}
