package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type KeyFn func(*http.Request) string

type Options struct {
	Capacity   float64 // tokens
	RefillRate float64 // tokens per second
	Key        KeyFn   // how to key buckets
	RetryAfter time.Duration
}

type Middleware struct {
	opts Options
	mu   sync.Mutex
	m    map[string]*bucket
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

func New(opts Options) *Middleware {
	if opts.Capacity <= 0 {
		opts.Capacity = 20
	}
	if opts.RefillRate <= 0 {
		opts.RefillRate = 10
	}
	if opts.Key == nil {
		opts.Key = clientIP
	}
	return &Middleware{opts: opts, m: make(map[string]*bucket)}
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := m.opts.Key(r)
		now := time.Now()

		m.mu.Lock()
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
			w.Header().Set("Retry-After", itoa(int(ra.Seconds())))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		b.tokens -= 1
		m.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
