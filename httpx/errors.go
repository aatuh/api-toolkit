package httpx

import (
	"errors"
	"net/http"

	"github.com/aatuh/api-toolkit/fielderrors"
	"github.com/aatuh/api-toolkit/ports"
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

// ProblemFromError maps errors to a Problem and HTTP status code.
func ProblemFromError(err error) (Problem, int) {
	if err == nil {
		return Problem{}, 0
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		status := httpErr.Status
		if status <= 0 {
			status = http.StatusInternalServerError
		}
		p := Problem{
			Title:  httpErr.Title,
			Detail: httpErr.Detail,
		}
		if p.Title == "" {
			p.Title = http.StatusText(status)
		}
		if p.Detail == "" {
			p.Detail = httpErr.Error()
		}
		return p, status
	}
	if errors.Is(err, ErrUnauthorized) {
		return Problem{
			Title:  http.StatusText(http.StatusUnauthorized),
			Detail: "unauthorized",
		}, http.StatusUnauthorized
	}
	if errors.Is(err, ErrForbidden) {
		return Problem{
			Title:  http.StatusText(http.StatusForbidden),
			Detail: "forbidden",
		}, http.StatusForbidden
	}
	if errors.Is(err, ErrTooManyRequests) {
		return Problem{
			Title:  http.StatusText(http.StatusTooManyRequests),
			Detail: "rate limit exceeded",
		}, http.StatusTooManyRequests
	}
	if errors.Is(err, ports.ErrResourceMissing) {
		return Problem{
			Title:  http.StatusText(http.StatusNotFound),
			Detail: "resource not found",
		}, http.StatusNotFound
	}
	var provider fielderrors.Provider
	if errors.As(err, &provider) {
		fieldErrs := provider.FieldErrors()
		if len(fieldErrs) > 0 {
			p := Problem{
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "validation failed",
			}
			return WithFieldErrors(p, fieldErrs), http.StatusBadRequest
		}
	}
	return Problem{
		Title:  http.StatusText(http.StatusInternalServerError),
		Detail: "internal server error",
	}, http.StatusInternalServerError
}

// WriteError maps an error to Problem+JSON response.
func WriteError(w http.ResponseWriter, err error) {
	p, status := ProblemFromError(err)
	if status <= 0 {
		return
	}
	WriteProblem(w, status, p)
}
