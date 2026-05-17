package httpx

import (
	"errors"
	"net/http"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
)

// ProblemCode is a stable machine-readable problem code.
type ProblemCode string

// ProblemDefinition describes one problem code.
type ProblemDefinition struct {
	Code             ProblemCode
	Status           int
	Type             string
	Title            string
	Detail           string
	Retryable        bool
	DocumentationURL string
	LogLevel         string
}

// ProblemCatalog maps stable codes to Problem Details definitions.
type ProblemCatalog struct {
	definitions map[ProblemCode]ProblemDefinition
}

// NewProblemCatalog constructs a catalog from definitions.
func NewProblemCatalog(definitions ...ProblemDefinition) *ProblemCatalog {
	catalog := &ProblemCatalog{definitions: map[ProblemCode]ProblemDefinition{}}
	for _, definition := range definitions {
		catalog.Register(definition)
	}
	return catalog
}

// DefaultProblemCatalog returns the toolkit default problem catalog.
func DefaultProblemCatalog() *ProblemCatalog {
	return NewProblemCatalog(
		ProblemDefinition{Code: ProblemCode(TypeBadRequest), Status: http.StatusBadRequest, Type: DefaultTypeURI(TypeBadRequest), Title: http.StatusText(http.StatusBadRequest), Detail: "bad request"},
		ProblemDefinition{Code: ProblemCode(TypeNotAcceptable), Status: http.StatusNotAcceptable, Type: DefaultTypeURI(TypeNotAcceptable), Title: http.StatusText(http.StatusNotAcceptable), Detail: "not acceptable"},
		ProblemDefinition{Code: ProblemCode(TypeValidation), Status: http.StatusBadRequest, Type: DefaultTypeURI(TypeValidation), Title: http.StatusText(http.StatusBadRequest), Detail: "validation failed"},
		ProblemDefinition{Code: ProblemCode(TypeUnsupportedMedia), Status: http.StatusUnsupportedMediaType, Type: DefaultTypeURI(TypeUnsupportedMedia), Title: http.StatusText(http.StatusUnsupportedMediaType), Detail: "unsupported media type"},
		ProblemDefinition{Code: ProblemCode(TypeUnauthorized), Status: http.StatusUnauthorized, Type: DefaultTypeURI(TypeUnauthorized), Title: http.StatusText(http.StatusUnauthorized), Detail: "unauthorized"},
		ProblemDefinition{Code: ProblemCode(TypeForbidden), Status: http.StatusForbidden, Type: DefaultTypeURI(TypeForbidden), Title: http.StatusText(http.StatusForbidden), Detail: "forbidden"},
		ProblemDefinition{Code: ProblemCode(TypeNotFound), Status: http.StatusNotFound, Type: DefaultTypeURI(TypeNotFound), Title: http.StatusText(http.StatusNotFound), Detail: "resource not found"},
		ProblemDefinition{Code: ProblemCode(TypeConflict), Status: http.StatusConflict, Type: DefaultTypeURI(TypeConflict), Title: http.StatusText(http.StatusConflict), Detail: "conflict"},
		ProblemDefinition{Code: ProblemCode(TypePayloadTooLarge), Status: http.StatusRequestEntityTooLarge, Type: DefaultTypeURI(TypePayloadTooLarge), Title: http.StatusText(http.StatusRequestEntityTooLarge), Detail: "payload too large"},
		ProblemDefinition{Code: ProblemCode(TypeRateLimited), Status: http.StatusTooManyRequests, Type: DefaultTypeURI(TypeRateLimited), Title: http.StatusText(http.StatusTooManyRequests), Detail: "rate limit exceeded", Retryable: true},
		ProblemDefinition{Code: ProblemCode(TypeServiceUnavailable), Status: http.StatusServiceUnavailable, Type: DefaultTypeURI(TypeServiceUnavailable), Title: http.StatusText(http.StatusServiceUnavailable), Detail: "service unavailable", Retryable: true},
		ProblemDefinition{Code: ProblemCode(TypeInternal), Status: http.StatusInternalServerError, Type: DefaultTypeURI(TypeInternal), Title: http.StatusText(http.StatusInternalServerError), Detail: "internal server error"},
	)
}

// Register adds or replaces a problem definition.
func (c *ProblemCatalog) Register(definition ProblemDefinition) {
	if c == nil {
		return
	}
	if definition.Code == "" {
		return
	}
	if definition.Status <= 0 {
		definition.Status = http.StatusInternalServerError
	}
	if definition.Title == "" {
		definition.Title = http.StatusText(definition.Status)
	}
	if definition.Type == "" {
		definition.Type = DefaultTypeURI(string(definition.Code))
	}
	if definition.Detail == "" {
		definition.Detail = definition.Title
	}
	if c.definitions == nil {
		c.definitions = map[ProblemCode]ProblemDefinition{}
	}
	c.definitions[definition.Code] = definition
}

// Definition returns a problem definition by code.
func (c *ProblemCatalog) Definition(code ProblemCode) (ProblemDefinition, bool) {
	if c == nil || c.definitions == nil {
		return ProblemDefinition{}, false
	}
	definition, ok := c.definitions[code]
	return definition, ok
}

// ProblemFromCode maps a code to Problem Details with the default catalog.
func ProblemFromCode(code ProblemCode) (Problem, int) {
	return DefaultProblemCatalog().Problem(code, "")
}

// Problem maps a code to Problem Details.
func (c *ProblemCatalog) Problem(code ProblemCode, detail string) (Problem, int) {
	definition, ok := c.Definition(code)
	if !ok {
		definition = ProblemDefinition{
			Code:   code,
			Status: http.StatusInternalServerError,
			Type:   DefaultTypeURI(TypeInternal),
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: "internal server error",
		}
	}
	if detail == "" {
		detail = definition.Detail
	}
	problem := Problem{
		Type:   definition.Type,
		Title:  definition.Title,
		Detail: detail,
	}
	problem.With("code", string(definition.Code))
	if definition.Retryable {
		problem.With("retryable", true)
	}
	if definition.DocumentationURL != "" {
		problem.With("documentation_url", definition.DocumentationURL)
	}
	if definition.LogLevel != "" {
		problem.With("log_level", definition.LogLevel)
	}
	return problem, definition.Status
}

// ProblemError carries a problem code and optional underlying error.
type ProblemError struct {
	Code   ProblemCode
	Detail string
	Err    error
}

func (e *ProblemError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail != "" {
		return e.Detail
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Code)
}

func (e *ProblemError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewProblemError constructs a coded problem error.
func NewProblemError(code ProblemCode, detail string, err error) *ProblemError {
	return &ProblemError{Code: code, Detail: detail, Err: err}
}

// WriteProblemCode writes a Problem Details response for a code.
func WriteProblemCode(w http.ResponseWriter, code ProblemCode) {
	problem, status := ProblemFromCode(code)
	WriteProblem(w, status, problem)
}

// ProblemFromErrorWithCatalog maps errors to Problem Details using a catalog.
func ProblemFromErrorWithCatalog(err error, catalog *ProblemCatalog, opts ErrorOptions) (Problem, int) {
	if err == nil {
		return Problem{}, 0
	}
	if catalog == nil {
		catalog = DefaultProblemCatalog()
	}
	var coded *ProblemError
	if errors.As(err, &coded) {
		problem, status := catalog.Problem(coded.Code, coded.Detail)
		return maybeRedact(problem, status, opts.Mode), status
	}
	var provider fielderrors.Provider
	if errors.As(err, &provider) {
		fieldErrs := provider.FieldErrors()
		if len(fieldErrs) > 0 {
			problem, status := catalog.Problem(ProblemCode(TypeValidation), "validation failed")
			return maybeRedact(WithFieldErrors(problem, fieldErrs), status, opts.Mode), status
		}
	}
	return ProblemFromErrorWithOptions(err, opts)
}
