// Package httpclient provides outbound HTTP clients with retry and tracing.
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"

	"github.com/aatuh/api-toolkit/contrib/v2/telemetry"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// Options configures the outbound HTTP client.
type Options struct {
	Timeout        time.Duration
	Transport      http.RoundTripper
	CheckRedirect  func(*http.Request, []*http.Request) error
	DisableTracing bool
	Sleep          func(time.Duration)
	Retry          RetryOptions
	Breaker        Breaker
	BreakerFailure FailureFunc
	Bulkhead       Bulkhead
}

// RetryOptions configures retry behavior.
type RetryOptions struct {
	Disable              bool
	MaxRetries           int
	MaxElapsedTime       time.Duration
	MinBackoff           time.Duration
	MaxBackoff           time.Duration
	RetryableStatusCodes []int
	RetryableMethods     []string
	UseRetryAfter        bool
	RetryOn              func(*http.Response, error) bool
}

type retryConfig struct {
	maxRetries       int
	maxElapsed       time.Duration
	minBackoff       time.Duration
	maxBackoff       time.Duration
	retryableStatus  map[int]struct{}
	retryableMethods map[string]struct{}
	useRetryAfter    bool
	retryOn          func(*http.Response, error) bool
}

// Client implements ports.HTTPClient.
type Client struct {
	client         *http.Client
	retry          retryConfig
	sleep          func(time.Duration)
	breaker        Breaker
	breakerFailure FailureFunc
	bulkhead       Bulkhead
}

var _ ports.HTTPClient = (*Client)(nil)

// New constructs an outbound HTTP client with sane defaults.
func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	baseTransport := opts.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	checkRedirect := opts.CheckRedirect
	if checkRedirect == nil {
		if cr, ok := baseTransport.(interface {
			CheckRedirect(*http.Request, []*http.Request) error
		}); ok {
			checkRedirect = cr.CheckRedirect
		}
	}
	if !opts.DisableTracing {
		baseTransport = telemetry.WrapHTTPTransport(baseTransport)
	}
	if opts.Sleep == nil {
		opts.Sleep = time.Sleep
	}
	return &Client{
		client: &http.Client{
			Timeout:       opts.Timeout,
			Transport:     baseTransport,
			CheckRedirect: checkRedirect,
		},
		retry:          normalizeRetry(opts.Retry),
		sleep:          opts.Sleep,
		breaker:        opts.Breaker,
		breakerFailure: normalizeBreakerFailure(opts.BreakerFailure),
		bulkhead:       opts.Bulkhead,
	}
}

// Do issues the HTTP request with retry behavior when configured.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("http client not configured")
	}
	if req == nil {
		return nil, errors.New("request is nil")
	}
	release, err := c.acquireBulkhead(req.Context())
	if err != nil {
		return nil, err
	}
	if release != nil {
		defer release()
	}
	do := func() (*http.Response, error) {
		return c.do(req)
	}
	if c.breaker != nil {
		return c.breaker.Execute(do, c.breakerFailure)
	}
	return do()
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.retry.maxRetries <= 0 || !c.methodRetryable(req.Method) {
		return c.client.Do(req)
	}

	start := time.Now()

	var resp *http.Response
	var err error

	bo := newBackoff(c.retry)
	for attempt := 0; attempt <= c.retry.maxRetries; attempt++ {
		if attempt > 0 {
			if err := resetBody(req); err != nil {
				return nil, err
			}
		}

		resp, err = c.client.Do(req)
		if !c.shouldRetry(resp, err) {
			return resp, err
		}
		if attempt == c.retry.maxRetries {
			return resp, err
		}
		delay := retryDelay(resp, c.retry, bo, time.Since(start))
		if delay == backoff.Stop {
			return resp, err
		}
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		if delay > 0 {
			c.sleep(delay)
		}
	}
	return resp, err
}

func normalizeRetry(opts RetryOptions) retryConfig {
	maxRetries := opts.MaxRetries
	if opts.Disable {
		maxRetries = 0
	} else if maxRetries <= 0 {
		maxRetries = 2
	}
	minBackoff := opts.MinBackoff
	if minBackoff <= 0 {
		minBackoff = 100 * time.Millisecond
	}
	maxBackoff := opts.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 2 * time.Second
	}
	statuses := opts.RetryableStatusCodes
	if len(statuses) == 0 {
		statuses = []int{
			http.StatusRequestTimeout,
			http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		}
	}
	methods := opts.RetryableMethods
	if len(methods) == 0 {
		methods = []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions}
	}
	return retryConfig{
		maxRetries:       maxRetries,
		maxElapsed:       opts.MaxElapsedTime,
		minBackoff:       minBackoff,
		maxBackoff:       maxBackoff,
		retryableStatus:  toIntSet(statuses),
		retryableMethods: toStringSet(methods),
		useRetryAfter:    opts.UseRetryAfter,
		retryOn:          opts.RetryOn,
	}
}

func newBackoff(cfg retryConfig) backoff.BackOff {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = cfg.minBackoff
	bo.MaxInterval = cfg.maxBackoff
	bo.Reset()
	return bo
}

func (c *Client) methodRetryable(method string) bool {
	if method == "" {
		return false
	}
	_, ok := c.retry.retryableMethods[strings.ToUpper(method)]
	return ok
}

func (c *Client) shouldRetry(resp *http.Response, err error) bool {
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		if c.retry.retryOn != nil {
			return c.retry.retryOn(resp, err)
		}
		return true
	}
	if c.retry.retryOn != nil {
		return c.retry.retryOn(resp, err)
	}
	if resp == nil {
		return false
	}
	_, ok := c.retry.retryableStatus[resp.StatusCode]
	return ok
}

func retryDelay(resp *http.Response, cfg retryConfig, bo backoff.BackOff, elapsed time.Duration) time.Duration {
	if cfg.maxElapsed > 0 && elapsed >= cfg.maxElapsed {
		return backoff.Stop
	}
	if cfg.useRetryAfter && resp != nil {
		if delay, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			if cfg.maxElapsed > 0 && elapsed+delay > cfg.maxElapsed {
				return backoff.Stop
			}
			return delay
		}
	}
	delay := bo.NextBackOff()
	if delay == backoff.Stop {
		return delay
	}
	if cfg.maxElapsed > 0 && elapsed+delay > cfg.maxElapsed {
		return backoff.Stop
	}
	return delay
}

func parseRetryAfter(value string) (time.Duration, bool) {
	if strings.TrimSpace(value) == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		if secs <= 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(value); err == nil {
		delay := time.Until(t)
		if delay <= 0 {
			return 0, false
		}
		return delay, true
	}
	return 0, false
}

func resetBody(req *http.Request) error {
	if req == nil || req.Body == nil {
		return nil
	}
	if req.GetBody == nil {
		return fmt.Errorf("request body is not replayable")
	}
	body, err := req.GetBody()
	if err != nil {
		return err
	}
	req.Body = body
	return nil
}

func (c *Client) acquireBulkhead(ctx context.Context) (func(), error) {
	if c.bulkhead == nil {
		return func() {}, nil
	}
	if ctx == nil {
		return nil, errors.New("context is nil")
	}
	return c.bulkhead.Acquire(ctx)
}

func toIntSet(values []int) map[int]struct{} {
	out := make(map[int]struct{}, len(values))
	for _, v := range values {
		out[v] = struct{}{}
	}
	return out
}

func toStringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		key := strings.ToUpper(strings.TrimSpace(v))
		if key == "" {
			continue
		}
		out[key] = struct{}{}
	}
	return out
}
