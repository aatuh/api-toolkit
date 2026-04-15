package recover

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/response_writer"
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

// Middleware converts panics into RFC 9457 problem details responses.
// It intentionally does not leak panic values to clients.
func Middleware() func(http.Handler) http.Handler {
	return New()
}

// New converts panics into RFC 9457 problem details responses and logs them
// through a configurable logger.
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
			ww := response_writer.Wrap(w)
			defer func() {
				if rec := recover(); rec != nil {
					fields := []any{"panic", fmt.Sprint(rec)}
					if cfg.logStack {
						fields = append(fields, "stack", string(debug.Stack()))
					}
					cfg.log.Error("panic recovered", fields...)
					if ww.Committed() {
						return
					}
					httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{
						Type:   httpx.DefaultTypeURI(httpx.TypeInternal),
						Title:  http.StatusText(http.StatusInternalServerError),
						Detail: "internal server error",
					})
				}
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
