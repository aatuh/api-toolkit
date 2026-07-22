package querylimits

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/aatuh/api-toolkit/v4/httpx"
)

// ErrorWriter allows overriding how validation errors are written.
type ErrorWriter func(http.ResponseWriter, int, httpx.Problem)

// Options configures query parameter limits.
type Options struct {
	MaxParams      int
	MaxKeyLength   int
	MaxValueLength int
	LimitParam     string
	MaxLimit       int
	ErrorWriter    ErrorWriter
}

// Middleware enforces query parameter limits.
type Middleware struct {
	opts        Options
	errorWriter ErrorWriter
}

// New constructs a query limits middleware with safe defaults.
func New(opts Options) (*Middleware, error) {
	if opts.MaxParams < 0 {
		return nil, errors.New("max params must be non-negative")
	}
	if opts.MaxKeyLength < 0 {
		return nil, errors.New("max key length must be non-negative")
	}
	if opts.MaxValueLength < 0 {
		return nil, errors.New("max value length must be non-negative")
	}
	if opts.MaxLimit < 0 {
		return nil, errors.New("max limit must be non-negative")
	}
	if opts.MaxParams == 0 {
		opts.MaxParams = 100
	}
	if opts.MaxKeyLength == 0 {
		opts.MaxKeyLength = 100
	}
	if opts.MaxValueLength == 0 {
		opts.MaxValueLength = 2048
	}
	if opts.LimitParam == "" {
		opts.LimitParam = "limit"
	}
	if opts.MaxLimit == 0 {
		opts.MaxLimit = 100
	}
	writer := opts.ErrorWriter
	if writer == nil {
		writer = defaultErrorWriter
	}
	return &Middleware{opts: opts, errorWriter: writer}, nil
}

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.Handler
}

// Handler wraps the next handler with query guardrails.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		problemType := httpx.DefaultTypeURI(httpx.TypeValidation)
		values := r.URL.Query()
		if exceedsParams(values, m.opts.MaxParams) {
			m.errorWriter(w, http.StatusBadRequest, httpx.Problem{
				Type:   problemType,
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "too many query parameters",
			})
			return
		}
		for key, vals := range values {
			if len(key) > m.opts.MaxKeyLength {
				m.errorWriter(w, http.StatusBadRequest, httpx.Problem{
					Type:   problemType,
					Title:  http.StatusText(http.StatusBadRequest),
					Detail: "query parameter key too long",
				})
				return
			}
			for _, val := range vals {
				if len(val) > m.opts.MaxValueLength {
					m.errorWriter(w, http.StatusBadRequest, httpx.Problem{
						Type:   problemType,
						Title:  http.StatusText(http.StatusBadRequest),
						Detail: "query parameter value too long",
					})
					return
				}
			}
		}
		if m.opts.MaxLimit > 0 && m.opts.LimitParam != "" {
			raw := strings.TrimSpace(values.Get(m.opts.LimitParam))
			if raw != "" {
				limit, err := strconv.Atoi(raw)
				if err != nil || limit <= 0 {
					m.errorWriter(w, http.StatusBadRequest, httpx.Problem{
						Type:   problemType,
						Title:  http.StatusText(http.StatusBadRequest),
						Detail: "invalid pagination limit",
					})
					return
				}
				if limit > m.opts.MaxLimit {
					m.errorWriter(w, http.StatusBadRequest, httpx.Problem{
						Type:   problemType,
						Title:  http.StatusText(http.StatusBadRequest),
						Detail: "pagination limit exceeds maximum",
					})
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func exceedsParams(values map[string][]string, maxParams int) bool {
	if maxParams <= 0 {
		return false
	}
	count := 0
	for _, vals := range values {
		count += len(vals)
	}
	return count > maxParams
}

func defaultErrorWriter(w http.ResponseWriter, status int, p httpx.Problem) {
	httpx.WriteProblemChecked(w, status, p)
}
