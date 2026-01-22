package secure

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aatuh/api-toolkit/httpx/identity"
	"github.com/aatuh/api-toolkit/ports"
)

// Options configures the security header middleware.
type Options struct {
	ContentSecurityPolicy     string
	ContentSecurityPolicyFunc func(*http.Request) string
	PermissionsPolicy         string
	ReferrerPolicy            string
	FrameOptions              string
	ContentTypeOptions        string
	HSTSMaxAge                time.Duration
	HSTSIncludeSubdomains     bool
	HSTSPreload               bool
	Resolver                  identity.Resolver
}

// Option applies a functional option to Options.
type Option func(*Options)

// WithCSP sets a static Content-Security-Policy header value.
func WithCSP(policy string) Option {
	return func(o *Options) {
		o.ContentSecurityPolicy = policy
	}
}

// WithCSPFunc sets a per-request Content-Security-Policy function.
func WithCSPFunc(fn func(*http.Request) string) Option {
	return func(o *Options) {
		o.ContentSecurityPolicyFunc = fn
	}
}

// WithPermissionsPolicy sets the Permissions-Policy header value.
func WithPermissionsPolicy(policy string) Option {
	return func(o *Options) {
		o.PermissionsPolicy = policy
	}
}

// WithResolver sets the trusted proxy resolver used for scheme detection.
func WithResolver(resolver identity.Resolver) Option {
	return func(o *Options) {
		o.Resolver = resolver
	}
}

// WithHSTS configures Strict-Transport-Security behavior.
func WithHSTS(maxAge time.Duration, includeSubdomains, preload bool) Option {
	return func(o *Options) {
		o.HSTSMaxAge = maxAge
		o.HSTSIncludeSubdomains = includeSubdomains
		o.HSTSPreload = preload
	}
}

// Handler adds a minimal set of sane security headers.
type Handler struct {
	opts      Options
	cspPolicy func(*http.Request) string
	hstsValue string
}

// New constructs a security header middleware with optional overrides.
func New(opts ...Option) ports.Middleware {
	cfg := Options{
		ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none'",
		ReferrerPolicy:        "no-referrer",
		FrameOptions:          "DENY",
		ContentTypeOptions:    "nosniff",
		HSTSMaxAge:            365 * 24 * time.Hour,
		HSTSIncludeSubdomains: true,
		Resolver: identity.Resolver{
			HeaderPolicy: identity.HeaderPolicyBoth,
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return &Handler{
		opts:      cfg,
		cspPolicy: buildCSPPolicy(cfg),
		hstsValue: buildHSTSValue(cfg),
	}
}

// Middleware returns the http.Handler middleware adapter.
func (h *Handler) Middleware() func(http.Handler) http.Handler {
	if h == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setIfEmpty(w.Header(), "X-Content-Type-Options", h.opts.ContentTypeOptions)
			setIfEmpty(w.Header(), "X-Frame-Options", h.opts.FrameOptions)
			setIfEmpty(w.Header(), "Referrer-Policy", h.opts.ReferrerPolicy)
			if h.opts.PermissionsPolicy != "" {
				setIfEmpty(w.Header(), "Permissions-Policy", h.opts.PermissionsPolicy)
			}
			if h.hstsValue != "" && h.opts.Resolver.Scheme(r) == "https" {
				setIfEmpty(w.Header(), "Strict-Transport-Security", h.hstsValue)
			}
			if h.cspPolicy != nil {
				if csp := h.cspPolicy(r); csp != "" {
					setIfEmpty(w.Header(), "Content-Security-Policy", csp)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func buildCSPPolicy(opts Options) func(*http.Request) string {
	if opts.ContentSecurityPolicyFunc != nil {
		return opts.ContentSecurityPolicyFunc
	}
	policy := opts.ContentSecurityPolicy
	return func(*http.Request) string {
		return policy
	}
}

func buildHSTSValue(opts Options) string {
	if opts.HSTSMaxAge <= 0 {
		return ""
	}
	age := int(opts.HSTSMaxAge.Seconds())
	if age <= 0 {
		return ""
	}
	value := "max-age=" + strconv.Itoa(age)
	if opts.HSTSIncludeSubdomains {
		value += "; includeSubDomains"
	}
	if opts.HSTSPreload {
		value += "; preload"
	}
	return value
}

func setIfEmpty(h http.Header, key, value string) {
	if value == "" {
		return
	}
	if h.Get(key) != "" {
		return
	}
	h.Set(key, value)
}
