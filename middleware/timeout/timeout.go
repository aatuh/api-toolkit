package timeout

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/v3/httpx"
)

const defaultHardTimeoutStatus = http.StatusGatewayTimeout
const defaultHardTimeoutMaxCaptureBytes int64 = 1 << 20
const defaultHardTimeoutCaptureOverflowStatus = http.StatusInternalServerError
const defaultHardTimeoutPanicStatus = http.StatusInternalServerError

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

var defaultHardTimeoutPanicProblem = httpx.Problem{
	Type:   httpx.TypeURI(httpx.DefaultTypeBase, "timeout-panic"),
	Title:  http.StatusText(defaultHardTimeoutPanicStatus),
	Detail: "handler panicked while running under hard timeout",
}

var ErrHardTimeoutCaptureLimitExceeded = errors.New("hard timeout response capture limit exceeded")

// HardTimeout applies a per-request context deadline and sends a timeout
// response when the deadline expires before the handler returns. Handler writes
// after the deadline are discarded. Responses are buffered up to
// MaxCaptureBytes, which defaults to 1 MiB when unset.
type HardTimeout struct {
	Timeout         time.Duration
	MaxCaptureBytes int64
	EventHooks      *HardTimeoutEventHooks
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
	EventHooks      *HardTimeoutEventHooks
}

// HardTimeoutOutcome classifies observable hard-timeout outcomes.
type HardTimeoutOutcome string

const (
	HardTimeoutOutcomeTimeout         HardTimeoutOutcome = "timeout"
	HardTimeoutOutcomePanic           HardTimeoutOutcome = "panic"
	HardTimeoutOutcomeCaptureOverflow HardTimeoutOutcome = "capture_overflow"
)

// HardTimeoutEvent contains bounded, low-cardinality metadata for operator
// hooks. It intentionally excludes panic values, paths, query strings, headers,
// and response bodies.
type HardTimeoutEvent struct {
	Outcome         HardTimeoutOutcome
	Method          string
	Status          int
	TimedOut        bool
	Panicked        bool
	CaptureOverflow bool
	Duration        time.Duration
	Timeout         time.Duration
	CaptureLimit    int64
}

// HardTimeoutEventHooks configures operator callbacks for hard-timeout
// outcomes. Keep callbacks non-blocking; panics from callbacks are contained.
type HardTimeoutEventHooks struct {
	// OnEvent receives bounded metadata for timeout, panic, and
	// capture-overflow outcomes.
	OnEvent func(HardTimeoutEvent)
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
		EventHooks:      opts.EventHooks,
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
// Panics from the wrapped handler are recovered inside the child goroutine. A
// panic before the timeout wins returns a deterministic 500 Problem Details
// response unless capture overflow already occurred. A panic after the timeout
// response has been sent is contained and dropped.
func (m *HardTimeout) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx, cancel := context.WithTimeout(r.Context(), m.Timeout)
		defer cancel()

		capture := newHardTimeoutCapture(m.captureLimit(), ctx.Done())
		done := make(chan hardTimeoutResult, 1)
		go func() {
			result := hardTimeoutResult{}
			defer func() {
				if recovered := recover(); recovered != nil {
					result.panicked = true
				}
				done <- result
			}()
			next.ServeHTTP(capture, r.WithContext(ctx))
		}()

		select {
		case result := <-done:
			if capture.timedOutOrDeadlineReached() {
				m.emitHardTimeoutEvent(r, start, HardTimeoutOutcomeTimeout, defaultHardTimeoutStatus)
				httpx.WriteProblem(w, defaultHardTimeoutStatus, defaultHardTimeoutProblem)
				return
			}
			if capture.overflowed() {
				m.emitHardTimeoutEvent(r, start, HardTimeoutOutcomeCaptureOverflow, defaultHardTimeoutCaptureOverflowStatus)
				httpx.WriteProblem(w, defaultHardTimeoutCaptureOverflowStatus, defaultHardTimeoutCaptureOverflowProblem)
				return
			}
			if result.panicked {
				m.emitHardTimeoutEvent(r, start, HardTimeoutOutcomePanic, defaultHardTimeoutPanicStatus)
				httpx.WriteProblem(w, defaultHardTimeoutPanicStatus, defaultHardTimeoutPanicProblem)
				return
			}
			capture.flushTo(w)
		case <-ctx.Done():
			capture.timeout()
			m.emitHardTimeoutEvent(r, start, HardTimeoutOutcomeTimeout, defaultHardTimeoutStatus)
			httpx.WriteProblem(w, defaultHardTimeoutStatus, defaultHardTimeoutProblem)
		}
	})
}

type hardTimeoutResult struct {
	panicked bool
}

func (m *HardTimeout) captureLimit() int64 {
	if m == nil {
		return defaultHardTimeoutMaxCaptureBytes
	}
	return hardTimeoutCaptureLimit(m.MaxCaptureBytes)
}

func (m *HardTimeout) emitHardTimeoutEvent(r *http.Request, start time.Time, outcome HardTimeoutOutcome, status int) {
	if m == nil || m.EventHooks == nil || m.EventHooks.OnEvent == nil {
		return
	}
	event := HardTimeoutEvent{
		Outcome:      outcome,
		Method:       hardTimeoutMethodLabel(r),
		Status:       status,
		Duration:     time.Since(start),
		Timeout:      m.Timeout,
		CaptureLimit: m.captureLimit(),
	}
	switch outcome {
	case HardTimeoutOutcomeTimeout:
		event.TimedOut = true
	case HardTimeoutOutcomePanic:
		event.Panicked = true
	case HardTimeoutOutcomeCaptureOverflow:
		event.CaptureOverflow = true
	}
	defer func() {
		_ = recover()
	}()
	m.EventHooks.OnEvent(event)
}

func hardTimeoutMethodLabel(r *http.Request) string {
	if r == nil {
		return "UNKNOWN"
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodTrace, http.MethodConnect:
		return r.Method
	case "":
		return "UNKNOWN"
	default:
		return "OTHER"
	}
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
	timeoutCh <-chan struct{}
}

func newHardTimeoutCapture(maxBytes int64, timeoutCh <-chan struct{}) *hardTimeoutCapture {
	return &hardTimeoutCapture{
		header:    make(http.Header),
		maxBytes:  hardTimeoutCaptureLimit(maxBytes),
		timeoutCh: timeoutCh,
	}
}

func (c *hardTimeoutCapture) Header() http.Header {
	return c.header
}

func (c *hardTimeoutCapture) WriteHeader(status int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deadlineReachedLocked() || c.wrote {
		return
	}
	c.status = status
	c.wrote = true
}

func (c *hardTimeoutCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deadlineReachedLocked() {
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

func (c *hardTimeoutCapture) timedOutOrDeadlineReached() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deadlineReachedLocked()
}

func (c *hardTimeoutCapture) deadlineReachedLocked() bool {
	if c.timedOut {
		return true
	}
	if c.timeoutCh == nil {
		return false
	}
	select {
	case <-c.timeoutCh:
		c.timedOut = true
		return true
	default:
		return false
	}
}

func (c *hardTimeoutCapture) overflowed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.overflow
}

func (c *hardTimeoutCapture) flushTo(w http.ResponseWriter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deadlineReachedLocked() || c.committed {
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
