package httpx

import (
	"errors"
	"net/http"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
	"github.com/aatuh/api-toolkit/v3/ports"
)

// Sentinel errors for common HTTP categories.
var (
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrTooManyRequests = errors.New("rate limit exceeded")
)

// HTTPError carries an explicit HTTP status and response detail.
type HTTPError struct {
	Status int
	Title  string
	Detail string
	Type   string
	Err    error
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail != "" {
		return e.Detail
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return http.StatusText(e.Status)
}

func (e *HTTPError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewHTTPError constructs an HTTPError with status and detail.
func NewHTTPError(status int, detail string) *HTTPError {
	return &HTTPError{
		Status: status,
		Title:  http.StatusText(status),
		Detail: detail,
	}
}

// ErrorMode controls how problems are produced from errors.
type ErrorMode int

const (
	// ErrorModeDefault preserves details for non-internal errors.
	ErrorModeDefault ErrorMode = iota
	// ErrorModeStrict redacts details for 5xx responses.
	ErrorModeStrict
)

// ErrorOptions customizes how errors are mapped to problems.
type ErrorOptions struct {
	Mode         ErrorMode
	TypeRegistry *TypeRegistry
}

// ProblemFromError maps errors to a Problem and HTTP status code.
func ProblemFromError(err error) (Problem, int) {
	return ProblemFromErrorWithOptions(err, ErrorOptions{})
}

// ProblemFromErrorStrict redacts 5xx details to avoid internal leakage.
func ProblemFromErrorStrict(err error) (Problem, int) {
	return ProblemFromErrorWithOptions(err, ErrorOptions{Mode: ErrorModeStrict})
}

// ProblemFromErrorWithOptions maps errors to a Problem using custom options.
func ProblemFromErrorWithOptions(err error, opts ErrorOptions) (Problem, int) {
	if err == nil {
		return Problem{}, 0
	}
	registry := registryOrDefault(opts.TypeRegistry)

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		status := httpErr.Status
		if status <= 0 {
			status = http.StatusInternalServerError
		}
		p := Problem{
			Title:  httpErr.Title,
			Detail: httpErr.Detail,
			Type:   registry.URI(httpErr.Type),
		}
		if p.Title == "" {
			p.Title = http.StatusText(status)
		}
		if p.Detail == "" {
			if httpErr.Err != nil {
				p.Detail = httpErr.Err.Error()
			} else {
				p.Detail = http.StatusText(status)
			}
		}
		if p.Type == "" {
			p.Type = typeForStatus(registry, status)
		}
		return maybeRedact(p, status, opts.Mode), status
	}
	if errors.Is(err, ErrUnauthorized) {
		return maybeRedact(Problem{
			Type:   registry.URI(TypeUnauthorized),
			Title:  http.StatusText(http.StatusUnauthorized),
			Detail: "unauthorized",
		}, http.StatusUnauthorized, opts.Mode), http.StatusUnauthorized
	}
	if errors.Is(err, ErrForbidden) {
		return maybeRedact(Problem{
			Type:   registry.URI(TypeForbidden),
			Title:  http.StatusText(http.StatusForbidden),
			Detail: "forbidden",
		}, http.StatusForbidden, opts.Mode), http.StatusForbidden
	}
	if errors.Is(err, ErrTooManyRequests) {
		return maybeRedact(Problem{
			Type:   registry.URI(TypeRateLimited),
			Title:  http.StatusText(http.StatusTooManyRequests),
			Detail: "rate limit exceeded",
		}, http.StatusTooManyRequests, opts.Mode), http.StatusTooManyRequests
	}
	if errors.Is(err, ports.ErrResourceMissing) {
		return maybeRedact(Problem{
			Type:   registry.URI(TypeNotFound),
			Title:  http.StatusText(http.StatusNotFound),
			Detail: "resource not found",
		}, http.StatusNotFound, opts.Mode), http.StatusNotFound
	}
	var provider fielderrors.Provider
	if errors.As(err, &provider) {
		fieldErrs := provider.FieldErrors()
		if len(fieldErrs) > 0 {
			p := Problem{
				Type:   registry.URI(TypeValidation),
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "validation failed",
			}
			return maybeRedact(WithFieldErrors(p, fieldErrs), http.StatusBadRequest, opts.Mode), http.StatusBadRequest
		}
	}
	return maybeRedact(Problem{
		Type:   registry.URI(TypeInternal),
		Title:  http.StatusText(http.StatusInternalServerError),
		Detail: "internal server error",
	}, http.StatusInternalServerError, opts.Mode), http.StatusInternalServerError
}

// WriteError maps an error to a Problem Details response.
func WriteError(w http.ResponseWriter, err error) {
	p, status := ProblemFromError(err)
	if status <= 0 {
		return
	}
	WriteProblem(w, status, p)
}

// WriteErrorStrict maps an error to a Problem Details response with redaction.
func WriteErrorStrict(w http.ResponseWriter, err error) {
	p, status := ProblemFromErrorStrict(err)
	if status <= 0 {
		return
	}
	WriteProblem(w, status, p)
}

// WriteErrorWithOptions maps an error to Problem Details using custom options.
func WriteErrorWithOptions(w http.ResponseWriter, err error, opts ErrorOptions) {
	p, status := ProblemFromErrorWithOptions(err, opts)
	if status <= 0 {
		return
	}
	WriteProblem(w, status, p)
}

func registryOrDefault(reg *TypeRegistry) *TypeRegistry {
	if reg != nil {
		return reg
	}
	return defaultTypeRegistry
}

func typeForStatus(reg *TypeRegistry, status int) string {
	switch status {
	case http.StatusBadRequest:
		return reg.URI(TypeBadRequest)
	case http.StatusNotAcceptable:
		return reg.URI(TypeNotAcceptable)
	case http.StatusUnsupportedMediaType:
		return reg.URI(TypeUnsupportedMedia)
	case http.StatusUnauthorized:
		return reg.URI(TypeUnauthorized)
	case http.StatusForbidden:
		return reg.URI(TypeForbidden)
	case http.StatusNotFound:
		return reg.URI(TypeNotFound)
	case http.StatusConflict:
		return reg.URI(TypeConflict)
	case http.StatusRequestEntityTooLarge:
		return reg.URI(TypePayloadTooLarge)
	case http.StatusTooManyRequests:
		return reg.URI(TypeRateLimited)
	case http.StatusServiceUnavailable:
		return reg.URI(TypeServiceUnavailable)
	}
	if status >= http.StatusInternalServerError {
		return reg.URI(TypeInternal)
	}
	return ""
}

// DefaultTypeForStatus returns the standard type URI for a status code.
func DefaultTypeForStatus(status int) string {
	return typeForStatus(defaultTypeRegistry, status)
}

func maybeRedact(p Problem, status int, mode ErrorMode) Problem {
	if mode != ErrorModeStrict || status < http.StatusInternalServerError {
		return p
	}
	title := http.StatusText(status)
	if title == "" {
		title = http.StatusText(http.StatusInternalServerError)
	}
	p.Title = title
	p.Detail = title
	return p
}
