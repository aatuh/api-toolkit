package httpx

import (
	"net/http"

	"github.com/aatuh/api-toolkit/fielderrors"
)

// WithFieldErrors attaches field-level errors to a problem payload.
func WithFieldErrors(p Problem, errs fielderrors.FieldErrors) Problem {
	if len(errs) == 0 {
		return p
	}
	p.With("errors", errs)
	if fieldMap := errs.ToMap(); fieldMap != nil {
		p.With("field_errors", fieldMap)
	}
	return p
}

// WriteProblemWithFieldErrors writes a problem+json with field errors attached.
func WriteProblemWithFieldErrors(w http.ResponseWriter, status int, p Problem, errs fielderrors.FieldErrors) {
	WriteProblem(w, status, WithFieldErrors(p, errs))
}
