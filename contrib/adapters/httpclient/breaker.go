package httpclient

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

// ErrBreakerOpen is returned when the circuit is open.
var ErrBreakerOpen = errors.New("circuit breaker open")

// FailureFunc determines whether a response or error should count as a failure.
type FailureFunc func(*http.Response, error) bool

// Breaker executes a function with circuit breaker semantics.
type Breaker interface {
	Execute(fn func() (*http.Response, error), isFailure FailureFunc) (*http.Response, error)
}

// CircuitBreakerOptions configures a circuit breaker.
type CircuitBreakerOptions struct {
	FailureThreshold    int
	SuccessThreshold    int
	OpenTimeout         time.Duration
	HalfOpenMaxInFlight int
	Now                 func() time.Time
}

// CircuitBreaker provides a simple failure-count breaker.
type CircuitBreaker struct {
	mu                  sync.Mutex
	state               breakerState
	failures            int
	successes           int
	openedAt            time.Time
	halfOpenInFlight    int
	failureThreshold    int
	successThreshold    int
	openTimeout         time.Duration
	halfOpenMaxInFlight int
	now                 func() time.Time
}

type breakerState int

const (
	breakerClosed breakerState = iota
	breakerOpen
	breakerHalfOpen
)

// NewCircuitBreaker creates a circuit breaker with sane defaults.
func NewCircuitBreaker(opts CircuitBreakerOptions) *CircuitBreaker {
	if opts.FailureThreshold <= 0 {
		opts.FailureThreshold = 5
	}
	if opts.SuccessThreshold <= 0 {
		opts.SuccessThreshold = 2
	}
	if opts.OpenTimeout <= 0 {
		opts.OpenTimeout = 30 * time.Second
	}
	if opts.HalfOpenMaxInFlight <= 0 {
		opts.HalfOpenMaxInFlight = 1
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &CircuitBreaker{
		state:               breakerClosed,
		failureThreshold:    opts.FailureThreshold,
		successThreshold:    opts.SuccessThreshold,
		openTimeout:         opts.OpenTimeout,
		halfOpenMaxInFlight: opts.HalfOpenMaxInFlight,
		now:                 opts.Now,
	}
}

// Execute runs fn when the circuit allows it, updating breaker state afterward.
func (b *CircuitBreaker) Execute(fn func() (*http.Response, error), isFailure FailureFunc) (*http.Response, error) {
	if b == nil {
		return nil, errors.New("circuit breaker is nil")
	}
	if fn == nil {
		return nil, errors.New("execute func is nil")
	}
	if isFailure == nil {
		isFailure = defaultBreakerFailure
	}
	now := b.now()
	if err := b.allow(now); err != nil {
		return nil, err
	}
	var (
		resp *http.Response
		err  error
	)
	defer func() {
		if recovered := recover(); recovered != nil {
			b.report(now, true)
			panic(recovered)
		}
	}()
	resp, err = fn()
	failure := isFailure(resp, err)
	b.report(now, failure)
	return resp, err
}

func (b *CircuitBreaker) allow(now time.Time) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case breakerOpen:
		if now.Sub(b.openedAt) < b.openTimeout {
			return ErrBreakerOpen
		}
		b.state = breakerHalfOpen
		b.failures = 0
		b.successes = 0
		b.halfOpenInFlight = 0
	case breakerHalfOpen:
		if b.halfOpenInFlight >= b.halfOpenMaxInFlight {
			return ErrBreakerOpen
		}
	case breakerClosed:
	}
	if b.state == breakerHalfOpen {
		b.halfOpenInFlight++
	}
	return nil
}

func (b *CircuitBreaker) report(now time.Time, failure bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case breakerHalfOpen:
		if b.halfOpenInFlight > 0 {
			b.halfOpenInFlight--
		}
		if failure {
			b.state = breakerOpen
			b.openedAt = now
			b.failures = 0
			b.successes = 0
			b.halfOpenInFlight = 0
			return
		}
		b.successes++
		if b.successes >= b.successThreshold {
			b.state = breakerClosed
			b.failures = 0
			b.successes = 0
		}
	case breakerClosed:
		if failure {
			b.failures++
			if b.failures >= b.failureThreshold {
				b.state = breakerOpen
				b.openedAt = now
				b.failures = 0
				b.successes = 0
			}
			return
		}
		b.failures = 0
	case breakerOpen:
	}
}

func normalizeBreakerFailure(fn FailureFunc) FailureFunc {
	if fn == nil {
		return defaultBreakerFailure
	}
	return fn
}

func defaultBreakerFailure(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return true
	}
	return resp.StatusCode >= http.StatusInternalServerError
}
