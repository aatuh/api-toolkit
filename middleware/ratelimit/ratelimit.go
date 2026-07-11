package ratelimit

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/v3/httpx"
	"github.com/aatuh/api-toolkit/v3/httpx/identity"
	"github.com/aatuh/api-toolkit/v3/ports"
)

// KeyFn extracts a key for rate limiting buckets.
type KeyFn func(*http.Request) string

// Limiter is the package-local rate limiter contract for middleware users.
type Limiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}

// Options configures the rate limit middleware.
type Options struct {
	Capacity   float64 // tokens
	RefillRate float64 // tokens per second
	Key        KeyFn   // how to key buckets
	RetryAfter time.Duration
	Clock      ports.Clock
	// Limiter overrides the default in-memory limiter with a shared limiter.
	Limiter Limiter
	// ClientIPResolver derives client identity from trusted proxies.
	ClientIPResolver identity.Resolver
	// StateTTL evicts buckets that have been idle for this duration.
	StateTTL time.Duration
	// CleanupInterval controls how often eviction runs.
	CleanupInterval time.Duration

	// SkipEnabled toggles honoring the SkipHeader. Useful for tests/dev.
	SkipEnabled bool
	// SkipHeader contains the header name that, when present, bypasses limiting.
	// When empty, no bypass is applied.
	SkipHeader string
	// AllowDangerousDevBypasses enables skip headers only when request comes from trusted proxies.
	AllowDangerousDevBypasses bool
	// FailOpen controls whether requests pass through when limiter errors.
	FailOpen bool
	// OnError receives limiter errors, when present.
	OnError func(error)
	// HeaderConfig enables standard RateLimit-* response headers when configured.
	HeaderConfig HeaderConfig
}

// Middleware enforces in-memory token bucket rate limits.
type Middleware struct {
	opts        Options
	mu          sync.Mutex
	m           map[string]*bucket
	lastCleanup time.Time
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// New constructs a rate limiting middleware with defaults.
func New(opts Options) (*Middleware, error) {
	if opts.Capacity < 0 {
		return nil, errors.New("capacity must be non-negative")
	}
	if opts.RefillRate < 0 {
		return nil, errors.New("refill rate must be non-negative")
	}
	if opts.StateTTL < 0 {
		return nil, errors.New("state ttl must be non-negative")
	}
	if opts.CleanupInterval < 0 {
		return nil, errors.New("cleanup interval must be non-negative")
	}
	if opts.AllowDangerousDevBypasses {
		if strings.TrimSpace(opts.SkipHeader) == "" {
			return nil, errors.New("skip header is required when dangerous bypasses are enabled")
		}
		if len(opts.ClientIPResolver.TrustedProxies) == 0 {
			return nil, errors.New("trusted proxies are required when dangerous bypasses are enabled")
		}
	}
	if opts.Capacity == 0 {
		opts.Capacity = 20
	}
	if opts.RefillRate == 0 {
		opts.RefillRate = 10
	}
	if opts.StateTTL == 0 {
		opts.StateTTL = 10 * time.Minute
	}
	if opts.CleanupInterval == 0 {
		opts.CleanupInterval = opts.StateTTL / 2
		if opts.CleanupInterval < time.Second {
			opts.CleanupInterval = time.Second
		}
	}
	if opts.Clock == nil {
		opts.Clock = ports.SystemClock{}
	}
	if opts.ClientIPResolver.HeaderPolicy == identity.HeaderPolicyNone &&
		len(opts.ClientIPResolver.TrustedProxies) > 0 {
		opts.ClientIPResolver.HeaderPolicy = identity.HeaderPolicyBoth
	}
	if opts.Key == nil {
		resolver := opts.ClientIPResolver
		opts.Key = func(r *http.Request) string {
			if ip := resolver.ClientIPString(r); ip != "" {
				return ip
			}
			return r.RemoteAddr
		}
	}
	return &Middleware{opts: opts, m: make(map[string]*bucket)}, nil
}

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with rate limiting logic.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.opts.SkipEnabled && m.opts.SkipHeader != "" && m.opts.AllowDangerousDevBypasses {
			if headerIsTrue(r.Header.Get(m.opts.SkipHeader)) &&
				len(m.opts.ClientIPResolver.TrustedProxies) > 0 &&
				m.opts.ClientIPResolver.TrustsRemoteAddr(r.RemoteAddr) {
				next.ServeHTTP(w, r)
				return
			}
		}

		key := m.opts.Key(r)

		if limiter := m.opts.Limiter; limiter != nil {
			allowed, retryAfter, err := limiter.Allow(r.Context(), key)
			if err != nil {
				if m.opts.OnError != nil {
					m.opts.OnError(err)
				}
				if m.opts.FailOpen {
					next.ServeHTTP(w, r)
					return
				}
				httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.Problem{
					Type:   httpx.DefaultTypeURI(httpx.TypeServiceUnavailable),
					Title:  http.StatusText(http.StatusServiceUnavailable),
					Detail: "rate limiter unavailable",
				})
				return
			}
			if !allowed {
				ra := retryAfter
				if ra <= 0 {
					ra = m.opts.RetryAfter
				}
				if ra <= 0 {
					ra = time.Second
				}
				SetRateLimitHeaders(w, Quota{RetryAfter: ra, Reset: m.opts.Clock.Now().Add(ra)}, m.opts.HeaderConfig)
				w.Header().Set("Retry-After", itoa(retryAfterSeconds(ra)))
				httpx.WriteProblem(w, http.StatusTooManyRequests, httpx.Problem{
					Type:   httpx.DefaultTypeURI(httpx.TypeRateLimited),
					Title:  http.StatusText(http.StatusTooManyRequests),
					Detail: "rate limit exceeded",
				})
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		now := m.opts.Clock.Now()

		m.mu.Lock()
		m.cleanup(now)
		b := m.m[key]
		if b == nil {
			b = &bucket{tokens: m.opts.Capacity, lastSeen: now}
			m.m[key] = b
		}
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.tokens += elapsed * m.opts.RefillRate
		if b.tokens > m.opts.Capacity {
			b.tokens = m.opts.Capacity
		}
		b.lastSeen = now

		if b.tokens < 1 {
			m.mu.Unlock()
			ra := m.opts.RetryAfter
			if ra <= 0 {
				ra = time.Second
			}
			SetRateLimitHeaders(w, Quota{Limit: int(m.opts.Capacity), Remaining: 0, Reset: now.Add(ra), RetryAfter: ra}, m.opts.HeaderConfig)
			w.Header().Set("Retry-After", itoa(retryAfterSeconds(ra)))
			httpx.WriteProblem(w, http.StatusTooManyRequests, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeRateLimited),
				Title:  http.StatusText(http.StatusTooManyRequests),
				Detail: "rate limit exceeded",
			})
			return
		}
		b.tokens--
		remaining := int(math.Floor(b.tokens))
		reset := now
		if m.opts.RefillRate > 0 && b.tokens < m.opts.Capacity {
			reset = now.Add(time.Duration(((m.opts.Capacity - b.tokens) / m.opts.RefillRate) * float64(time.Second)))
		}
		m.mu.Unlock()

		SetRateLimitHeaders(w, Quota{Limit: int(m.opts.Capacity), Remaining: remaining, Reset: reset}, m.opts.HeaderConfig)

		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) cleanup(now time.Time) {
	if m.opts.StateTTL <= 0 || m.opts.CleanupInterval <= 0 {
		return
	}
	if !m.lastCleanup.IsZero() && now.Sub(m.lastCleanup) < m.opts.CleanupInterval {
		return
	}
	for key, b := range m.m {
		if now.Sub(b.lastSeen) > m.opts.StateTTL {
			delete(m.m, key)
		}
	}
	m.lastCleanup = now
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var a [12]byte
	i := len(a)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		a[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		a[i] = '-'
	}
	return string(a[i:])
}

func retryAfterSeconds(d time.Duration) int {
	if d <= 0 {
		return 0
	}
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		return 1
	}
	return seconds
}

func headerIsTrue(val string) bool {
	return strings.TrimSpace(val) == "true"
}
