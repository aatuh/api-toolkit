package ratelimit

import (
	"container/list"
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/httpx/identity"
	"github.com/aatuh/api-toolkit/v4/ports"
)

const (
	defaultCleanupBatchSize = 64
	anonymousRateLimitKey   = "anonymous"
)

// KeyFn extracts a key for rate limiting buckets.
type KeyFn func(*http.Request) string

// Limiter is the package-local rate limiter contract for middleware users.
type Limiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}

// DecisionLimiter returns complete quota metadata for a rate-limit decision.
// Use it for distributed enforcement when clients need consistent rate-limit
// headers from the authoritative limiter.
type DecisionLimiter interface {
	// Allow returns the authoritative decision and quota metadata for key.
	Allow(ctx context.Context, key string) (Decision, error)
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
	// DecisionLimiter overrides the default in-memory limiter with a shared
	// limiter that supplies complete quota metadata. It cannot be combined with
	// Limiter because both contracts make the enforcement decision.
	DecisionLimiter DecisionLimiter
	// ClientIPResolver derives client identity from trusted proxies.
	ClientIPResolver identity.Resolver
	// StateTTL evicts buckets that have been idle for this duration.
	StateTTL time.Duration
	// CleanupInterval controls how often eviction runs.
	CleanupInterval time.Duration
	// CleanupBatchSize bounds the number of expired in-memory buckets removed by
	// one request-path cleanup pass.
	CleanupBatchSize int

	// SkipEnabled toggles honoring the SkipHeader. Useful for tests/dev.
	SkipEnabled bool
	// SkipHeader contains the header name that, when present, bypasses limiting.
	// When empty, no bypass is applied.
	SkipHeader string
	// AllowDangerousDevBypasses enables skip headers only when request comes from trusted proxies.
	AllowDangerousDevBypasses bool
	// FailOpen controls whether requests pass through when limiter errors.
	FailOpen bool
	// OnError receives limiter and response-write errors, when present.
	OnError func(error)
	// HeaderConfig enables standard RateLimit-* response headers when configured.
	HeaderConfig HeaderConfig
}

// Middleware enforces in-memory token bucket rate limits.
type Middleware struct {
	opts        Options
	mu          sync.Mutex
	m           map[string]*bucket
	lru         list.List
	lastCleanup time.Time
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
	entry    *list.Element
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
	if opts.CleanupBatchSize < 0 {
		return nil, errors.New("cleanup batch size must be non-negative")
	}
	if opts.Limiter != nil && opts.DecisionLimiter != nil {
		return nil, errors.New("limiter and decision limiter cannot both be configured")
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
	if opts.CleanupBatchSize == 0 {
		opts.CleanupBatchSize = defaultCleanupBatchSize
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

		key := normalizeRateLimitKey(m.opts.Key(r))

		if limiter := m.decisionLimiter(); limiter != nil {
			decision, err := limiter.Allow(r.Context(), key)
			if err != nil {
				m.handleLimiterError(w, r, next, err)
				return
			}
			if !m.writeDecision(w, decision) {
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		now := m.opts.Clock.Now()

		m.mu.Lock()
		m.cleanup(now)
		b := m.m[key]
		if b != nil && m.expired(now, b) {
			m.removeBucket(key, b)
			b = nil
		}
		if b == nil {
			b = &bucket{tokens: m.opts.Capacity, lastSeen: now}
			b.entry = m.lru.PushBack(key)
			m.m[key] = b
		} else {
			m.lru.MoveToBack(b.entry)
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
			m.writeProblem(w, http.StatusTooManyRequests, httpx.Problem{
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

func (m *Middleware) decisionLimiter() DecisionLimiter {
	if m.opts.DecisionLimiter != nil {
		return m.opts.DecisionLimiter
	}
	if m.opts.Limiter != nil {
		return legacyDecisionLimiter{limiter: m.opts.Limiter}
	}
	return nil
}

func (m *Middleware) handleLimiterError(w http.ResponseWriter, r *http.Request, next http.Handler, err error) {
	if m.opts.OnError != nil {
		m.opts.OnError(err)
	}
	if m.opts.FailOpen {
		next.ServeHTTP(w, r)
		return
	}
	m.writeProblem(w, http.StatusServiceUnavailable, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeServiceUnavailable),
		Title:  http.StatusText(http.StatusServiceUnavailable),
		Detail: "rate limiter unavailable",
	})
}

func (m *Middleware) writeDecision(w http.ResponseWriter, decision Decision) bool {
	quota := QuotaFromDecision(decision)
	if !decision.Allowed {
		if quota.RetryAfter <= 0 {
			quota.RetryAfter = m.opts.RetryAfter
		}
		if quota.RetryAfter <= 0 {
			quota.RetryAfter = time.Second
		}
		if quota.Reset.IsZero() {
			quota.Reset = m.opts.Clock.Now().Add(quota.RetryAfter)
		}
	}
	if decisionHasQuota(decision) || !decision.Allowed {
		SetRateLimitHeaders(w, quota, m.opts.HeaderConfig)
	}
	if decision.Allowed {
		return true
	}
	w.Header().Set(normalizeHeaderConfig(m.opts.HeaderConfig).RetryAfterHeader, itoa(retryAfterSeconds(quota.RetryAfter)))
	m.writeProblem(w, http.StatusTooManyRequests, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeRateLimited),
		Title:  http.StatusText(http.StatusTooManyRequests),
		Detail: "rate limit exceeded",
	})
	return false
}

func decisionHasQuota(decision Decision) bool {
	return decision.Limit > 0 || decision.Remaining != 0 || !decision.Reset.IsZero() || decision.RetryAfter > 0 ||
		decision.Quota.Limit > 0 || decision.Quota.Remaining != 0 || !decision.Quota.Reset.IsZero() || decision.Quota.RetryAfter > 0
}

func normalizeRateLimitKey(key string) string {
	if strings.TrimSpace(key) == "" {
		return anonymousRateLimitKey
	}
	return key
}

func (m *Middleware) writeProblem(w http.ResponseWriter, status int, problem httpx.Problem) {
	if err := httpx.WriteProblemChecked(w, status, problem); err != nil && m.opts.OnError != nil {
		m.opts.OnError(err)
	}
}

func (m *Middleware) cleanup(now time.Time) int {
	if m.opts.StateTTL <= 0 || m.opts.CleanupInterval <= 0 {
		return 0
	}
	if !m.lastCleanup.IsZero() && now.Sub(m.lastCleanup) < m.opts.CleanupInterval {
		return 0
	}
	removed := 0
	for removed < m.opts.CleanupBatchSize {
		entry := m.lru.Front()
		if entry == nil {
			break
		}
		key, _ := entry.Value.(string)
		b := m.m[key]
		if b == nil {
			m.lru.Remove(entry)
			continue
		}
		if !m.expired(now, b) {
			break
		}
		m.removeBucket(key, b)
		removed++
	}
	m.lastCleanup = now
	return removed
}

func (m *Middleware) expired(now time.Time, b *bucket) bool {
	return b != nil && now.Sub(b.lastSeen) > m.opts.StateTTL
}

func (m *Middleware) removeBucket(key string, b *bucket) {
	if b != nil && b.entry != nil {
		m.lru.Remove(b.entry)
		b.entry = nil
	}
	delete(m.m, key)
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

type legacyDecisionLimiter struct {
	limiter Limiter
}

func (l legacyDecisionLimiter) Allow(ctx context.Context, key string) (Decision, error) {
	allowed, retryAfter, err := l.limiter.Allow(ctx, key)
	return Decision{Allowed: allowed, RetryAfter: retryAfter}, err
}
