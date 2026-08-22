package recover

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/ports"
)

type config struct {
	log      ports.Logger
	logStack bool
}

// Option customizes panic recovery logging.
type Option func(*config)

// WithLogger routes panic logs to the provided logger.
func WithLogger(log ports.Logger) Option {
	return func(cfg *config) {
		cfg.log = log
	}
}

// WithStackLogging controls whether panic stacks are added to logs.
func WithStackLogging(enabled bool) Option {
	return func(cfg *config) {
		cfg.logStack = enabled
	}
}

type stderrLogger struct{}

func (stderrLogger) Debug(string, ...any) {}
func (stderrLogger) Info(string, ...any)  {}
func (stderrLogger) Warn(string, ...any)  {}

func (stderrLogger) Error(msg string, kv ...any) {
	_, _ = fmt.Fprintln(os.Stderr, msg)
	for i := 0; i+1 < len(kv); i += 2 {
		_, _ = fmt.Fprintf(os.Stderr, "%v=%v\n", kv[i], kv[i+1])
	}
}

// Middleware converts panics into RFC 9457 problem details responses when the
// response is still uncommitted. If the handler already committed headers or
// body bytes, the middleware logs the panic and aborts the request rather than
// preserving a misleading partial success response.
func Middleware() func(http.Handler) http.Handler {
	return New()
}

// New converts uncommitted panics into RFC 9457 problem details responses and
// logs them through a configurable logger. Once a response has been committed,
// recovery logs the panic and aborts the request instead of returning a partial
// success response.
func New(opts ...Option) func(http.Handler) http.Handler {
	cfg := config{
		log:      stderrLogger{},
		logStack: true,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.log == nil {
		cfg.log = ports.NopLogger{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := wrapResponseWriter(w)
			defer func() {
				if rec := recover(); rec != nil {
					if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
						panic(http.ErrAbortHandler)
					}
					fields := []any{"panic", fmt.Sprint(rec)}
					if cfg.logStack {
						fields = append(fields, "stack", string(debug.Stack()))
					}
					cfg.log.Error("panic recovered", fields...)
					if ww.Committed() {
						panic(http.ErrAbortHandler)
					}
					if err := httpx.WriteProblemChecked(w, http.StatusInternalServerError, httpx.Problem{
						Type:   httpx.DefaultTypeURI(httpx.TypeInternal),
						Title:  http.StatusText(http.StatusInternalServerError),
						Detail: "internal server error",
					}); err != nil {
						return
					}
				}
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
