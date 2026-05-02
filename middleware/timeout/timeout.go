package timeout

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/v2/httpx"
)

const defaultHardTimeoutStatus = http.StatusGatewayTimeout
const defaultHardTimeoutMaxCaptureBytes int64 = 1 << 20
const defaultHardTimeoutCaptureOverflowStatus = http.StatusInternalServerError

var defaultHardTimeoutProblem = httpx.Problem{
	Type:   httpx.TypeURI(httpx.DefaultTypeBase, "timeout"),
	Title:  http.StatusText(defaultHardTimeoutStatus),
	Detail: "request timed out",
}

var defaultHardTimeoutCaptureOverflowProblem = httpx.Problem{
	Type:   httpx.TypeURI(httpx.DefaultTypeBase, "timeout-capture-overflow"),
	Title:  http.StatusText(defaultHardTimeoutCaptureOverflowStatus),
	Detail: "response capture exceeded the configured hard-timeout limit",
}

var ErrHardTimeoutCaptureLimitExceeded = errors.New("hard timeout response capture limit exceeded")

// HardTimeout applies a per-request context deadline and sends a timeout
// response when the deadline expires before the handler returns. Handler writes
// after the deadline are discarded. Responses are buffered up to
// MaxCaptureBytes, which defaults to 1 MiB when unset.
type HardTimeout struct {
	Timeout         time.Duration
	MaxCaptureBytes int64
}

// Propagator applies a per-request context deadline without writing timeout
// responses. Handlers and downstream calls must observe ctx.Done for the
// deadline to take effect.
type Propagator struct {
	Timeout time.Duration
}

// Middleware is kept as a backward-compatible alias for Propagator.
type Middleware = Propagator

// Options configures the timeout middleware.
type Options struct {
	Timeout         time.Duration
	MaxCaptureBytes int64
}

// NewPropagator constructs a cooperative request-deadline propagator with the
// given duration.
func NewPropagator(opts Options) (*Propagator, error) {
	if opts.Timeout <= 0 {
		return nil, errors.New("timeout must be greater than zero")
	}
	return &Propagator{Timeout: opts.Timeout}, nil
}

// New constructs a cooperative request-deadline propagator.
// Deprecated: use NewPropagator to make the non-aborting behavior explicit.
func New(opts Options) (*Propagator, error) {
	return NewPropagator(opts)
}

// NewHard constructs a hard wall-clock timeout middleware. It keeps the
// cooperative request context deadline and also synthesizes a 504 Problem
// Details response when the wrapped handler does not return in time.
func NewHard(opts Options) (*HardTimeout, error) {
	if opts.Timeout <= 0 {
		return nil, errors.New("timeout must be greater than zero")
	}
	if opts.MaxCaptureBytes < 0 {
		return nil, errors.New("max capture bytes must be greater than or equal to zero")
	}
	return &HardTimeout{
		Timeout:         opts.Timeout,
		MaxCaptureBytes: hardTimeoutCaptureLimit(opts.MaxCaptureBytes),
	}, nil
}

// Middleware implements ports.Middleware via Handler adapter.
func (m *Propagator) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with a context deadline.
// It does not abort writes or synthesize timeout responses.
func (m *Propagator) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), m.Timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Middleware implements ports.Middleware via Handler adapter.
func (m *HardTimeout) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with a context deadline and a hard timeout
// response. It cannot stop CPU work in a handler that ignores ctx.Done, but it
// does stop that handler from writing the client response after the deadline.
func (m *HardTimeout) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), m.Timeout)
		defer cancel()

		capture := newHardTimeoutCapture(m.captureLimit())
		done := make(chan struct{})
		go func() {
			defer close(done)
			next.ServeHTTP(capture, r.WithContext(ctx))
		}()

		select {
		case <-done:
			if capture.overflowed() {
				httpx.WriteProblem(w, defaultHardTimeoutCaptureOverflowStatus, defaultHardTimeoutCaptureOverflowProblem)
				return
			}
			capture.flushTo(w)
		case <-ctx.Done():
			capture.timeout()
			httpx.WriteProblem(w, defaultHardTimeoutStatus, defaultHardTimeoutProblem)
		}
	})
}

func (m *HardTimeout) captureLimit() int64 {
	if m == nil {
		return defaultHardTimeoutMaxCaptureBytes
	}
	return hardTimeoutCaptureLimit(m.MaxCaptureBytes)
}

func hardTimeoutCaptureLimit(limit int64) int64 {
	if limit <= 0 {
		return defaultHardTimeoutMaxCaptureBytes
	}
	return limit
}

type hardTimeoutCapture struct {
	mu        sync.Mutex
	header    http.Header
	body      bytes.Buffer
	status    int
	wrote     bool
	timedOut  bool
	committed bool
	maxBytes  int64
	overflow  bool
}

func newHardTimeoutCapture(maxBytes int64) *hardTimeoutCapture {
	return &hardTimeoutCapture{header: make(http.Header), maxBytes: hardTimeoutCaptureLimit(maxBytes)}
}

func (c *hardTimeoutCapture) Header() http.Header {
	return c.header
}

func (c *hardTimeoutCapture) WriteHeader(status int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timedOut || c.wrote {
		return
	}
	c.status = status
	c.wrote = true
}

func (c *hardTimeoutCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	if !c.wrote {
		c.status = http.StatusOK
		c.wrote = true
	}
	if c.overflow {
		return 0, ErrHardTimeoutCaptureLimitExceeded
	}
	remaining := c.maxBytes - int64(c.body.Len())
	if remaining <= 0 || int64(len(p)) > remaining {
		c.overflow = true
		return 0, ErrHardTimeoutCaptureLimitExceeded
	}
	return c.body.Write(p)
}

func (c *hardTimeoutCapture) timeout() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timedOut = true
}

func (c *hardTimeoutCapture) overflowed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.overflow
}

func (c *hardTimeoutCapture) flushTo(w http.ResponseWriter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timedOut || c.committed {
		return
	}
	c.committed = true
	for name, values := range c.header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	status := c.status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(c.body.Bytes())
}
