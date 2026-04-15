package securityprofile

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	maxbody "github.com/aatuh/api-toolkit/v2/middleware/maxbody"
	querylimits "github.com/aatuh/api-toolkit/v2/middleware/querylimits"
	ratelimit "github.com/aatuh/api-toolkit/v2/middleware/ratelimit"
	securemw "github.com/aatuh/api-toolkit/v2/middleware/secure"
	timeoutmw "github.com/aatuh/api-toolkit/v2/middleware/timeout"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// ErrorWriter allows overriding how security profile errors are written.
type ErrorWriter func(http.ResponseWriter, int, httpx.Problem)

// #nosec G101 -- explicit dev-only header label, not a credential or secret.
const defaultDevBypassHeader = "X-Debug-Auth-Bypass"

type options struct {
	maxBodyBytes     int64
	queryLimits      querylimits.Options
	enableQueryGuard bool
	timeout          time.Duration
	enableTimeout    bool
	rateLimit        ratelimit.Options
	enableRateLimit  bool
	requireAuth      bool
	authCheck        func(*http.Request) bool
	authAllowlist    []string
	devBypassHeader  string
	allowDevBypass   bool
	resolver         identity.Resolver
	secureOptions    []securemw.Option
	errorWriter      ErrorWriter
	routeOverrides   []RouteOverride
}

// Option customizes the security profile.
type Option func(*options)

// RouteOverride customizes limits for matching requests.
type RouteOverride struct {
	Pattern string
	Methods []string

	MaxBodyBytes       *int64
	QueryLimits        *querylimits.Options
	QueryLimitsEnabled *bool
	Timeout            *time.Duration
	TimeoutEnabled     *bool
	RateLimit          *ratelimit.Options
	RateLimitEnabled   *bool
}

// WithMaxBodyBytes sets the maximum request body size.
func WithMaxBodyBytes(n int64) Option {
	return func(o *options) {
		o.maxBodyBytes = n
	}
}

// WithQueryLimits overrides query limits middleware options.
func WithQueryLimits(opts querylimits.Options) Option {
	return func(o *options) {
		o.queryLimits = opts
		o.enableQueryGuard = true
	}
}

// WithQueryLimitsDisabled disables query limits enforcement.
func WithQueryLimitsDisabled() Option {
	return func(o *options) {
		o.enableQueryGuard = false
	}
}

// WithTimeout sets a cooperative per-request context deadline.
// It does not enforce a wall-clock response cutoff by itself.
func WithTimeout(d time.Duration) Option {
	return func(o *options) {
		o.timeout = d
		o.enableTimeout = true
	}
}

// WithTimeoutDisabled disables request context deadlines.
func WithTimeoutDisabled() Option {
	return func(o *options) {
		o.enableTimeout = false
	}
}

// WithRateLimitOptions configures rate limiting options.
func WithRateLimitOptions(opts ratelimit.Options) Option {
	return func(o *options) {
		o.rateLimit = opts
		o.enableRateLimit = true
	}
}

// WithRateLimitDisabled disables rate limiting.
func WithRateLimitDisabled() Option {
	return func(o *options) {
		o.enableRateLimit = false
	}
}

// WithRouteOverrides sets per-route limit overrides.
func WithRouteOverrides(overrides ...RouteOverride) Option {
	return func(o *options) {
		o.routeOverrides = append(o.routeOverrides, overrides...)
	}
}

// WithRequireAuth sets whether authentication is required by default.
func WithRequireAuth(required bool) Option {
	return func(o *options) {
		o.requireAuth = required
	}
}

// WithAuthCheck sets a function that determines whether a request is authenticated.
func WithAuthCheck(fn func(*http.Request) bool) Option {
	return func(o *options) {
		o.authCheck = fn
	}
}

// WithAuthAllowlist sets paths that bypass auth checks.
func WithAuthAllowlist(paths ...string) Option {
	return func(o *options) {
		o.authAllowlist = append([]string(nil), paths...)
	}
}

// WithDevBypassHeader sets a development-only auth bypass header.
func WithDevBypassHeader(header string, allow bool) Option {
	return func(o *options) {
		o.devBypassHeader = header
		o.allowDevBypass = allow
	}
}

// WithResolver sets the identity resolver for trusted proxy checks.
func WithResolver(resolver identity.Resolver) Option {
	return func(o *options) {
		o.resolver = resolver
	}
}

// WithSecureOptions appends secure header middleware options.
func WithSecureOptions(opts ...securemw.Option) Option {
	return func(o *options) {
		o.secureOptions = append(o.secureOptions, opts...)
	}
}

// WithErrorWriter overrides the error writer.
func WithErrorWriter(fn ErrorWriter) Option {
	return func(o *options) {
		o.errorWriter = fn
	}
}

// Profile describes a composed security middleware stack.
type Profile struct {
	Middlewares []func(http.Handler) http.Handler
}

// Apply attaches the profile middlewares to the router.
func (p Profile) Apply(r ports.HTTPRouter) {
	p.ApplyTo(r)
}

// ApplyTo attaches the profile middlewares to a minimal middleware chain.
func (p Profile) ApplyTo(r ports.MiddlewareChain) {
	if r == nil || len(p.Middlewares) == 0 {
		return
	}
	r.Use(p.Middlewares...)
}

// OWASPBaseline returns a security profile that aligns with OWASP API resource limits.
func OWASPBaseline(opts ...Option) (Profile, error) {
	base := []Option{
		WithMaxBodyBytes(1 << 20),
		WithTimeout(5 * time.Second),
		WithRateLimitOptions(ratelimit.Options{
			Capacity:   30,
			RefillRate: 15,
			RetryAfter: time.Second,
		}),
	}
	return New(append(base, opts...)...)
}

// New builds a security profile using the provided options.
func New(opts ...Option) (Profile, error) {
	cfg := options{
		maxBodyBytes:     1 << 20,
		enableQueryGuard: true,
		queryLimits:      querylimits.Options{},
		requireAuth:      true,
		devBypassHeader:  defaultDevBypassHeader,
		resolver: identity.Resolver{
			HeaderPolicy: identity.HeaderPolicyBoth,
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.requireAuth && cfg.authCheck == nil {
		return Profile{}, errors.New("security profile requires an auth check")
	}
	if cfg.errorWriter == nil {
		cfg.errorWriter = defaultErrorWriter
	}
	ensureQueryErrorWriter(&cfg.queryLimits, cfg.errorWriter)

	secureOptions := append([]securemw.Option{
		securemw.WithResolver(cfg.resolver),
		securemw.APIOnly(),
	}, cfg.secureOptions...)

	baseLimits := limitConfig{
		maxBodyBytes:     cfg.maxBodyBytes,
		queryLimits:      cfg.queryLimits,
		enableQueryGuard: cfg.enableQueryGuard,
		timeout:          cfg.timeout,
		enableTimeout:    cfg.enableTimeout,
		rateLimit:        cfg.rateLimit,
		enableRateLimit:  cfg.enableRateLimit,
		resolver:         cfg.resolver,
		errorWriter:      cfg.errorWriter,
	}
	limits, err := buildLimitsMiddleware(baseLimits, cfg.routeOverrides)
	if err != nil {
		return Profile{}, err
	}

	secure, err := securemw.New(secureOptions...)
	if err != nil {
		return Profile{}, fmt.Errorf("secure middleware: %w", err)
	}
	chain := []func(http.Handler) http.Handler{
		secure.Middleware(),
		limits.Middleware(),
	}
	if cfg.requireAuth {
		chain = append(chain, authMiddleware(cfg))
	}
	return Profile{Middlewares: chain}, nil
}

func authMiddleware(cfg options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if pathAllowed(r.URL.Path, cfg.authAllowlist) {
				next.ServeHTTP(w, r)
				return
			}
			if cfg.allowDevBypass && devBypassEnabled() && headerIsTrue(r.Header.Get(cfg.devBypassHeader)) {
				if len(cfg.resolver.TrustedProxies) > 0 && cfg.resolver.TrustsRemoteAddr(r.RemoteAddr) {
					next.ServeHTTP(w, r)
					return
				}
			}
			if cfg.authCheck != nil && cfg.authCheck(r) {
				next.ServeHTTP(w, r)
				return
			}
			cfg.errorWriter(w, http.StatusUnauthorized, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "authentication required",
			})
		})
	}
}

func pathAllowed(path string, allowlist []string) bool {
	for _, pattern := range allowlist {
		if pathMatches(path, pattern) {
			return true
		}
	}
	return false
}

func pathMatches(path, pattern string) bool {
	p := strings.TrimSpace(pattern)
	if p == "" {
		return false
	}
	if strings.HasSuffix(p, "*") {
		return strings.HasPrefix(path, strings.TrimSuffix(p, "*"))
	}
	return path == p
}

func headerIsTrue(val string) bool {
	return strings.TrimSpace(val) == "true"
}

func defaultErrorWriter(w http.ResponseWriter, status int, p httpx.Problem) {
	httpx.WriteProblem(w, status, p)
}

type limitConfig struct {
	maxBodyBytes     int64
	queryLimits      querylimits.Options
	enableQueryGuard bool
	timeout          time.Duration
	enableTimeout    bool
	rateLimit        ratelimit.Options
	enableRateLimit  bool
	resolver         identity.Resolver
	errorWriter      ErrorWriter
}

type limitsMiddleware struct {
	baseline  func(http.Handler) http.Handler
	overrides []routeOverride
}

type routeOverride struct {
	pattern    string
	methods    []string
	middleware func(http.Handler) http.Handler
}

func (m *limitsMiddleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mw := m.baseline
			for _, override := range m.overrides {
				if !pathMatches(r.URL.Path, override.pattern) {
					continue
				}
				if !methodMatches(r.Method, override.methods) {
					continue
				}
				mw = override.middleware
				break
			}
			mw(next).ServeHTTP(w, r)
		})
	}
}

func buildLimitsMiddleware(base limitConfig, overrides []RouteOverride) (*limitsMiddleware, error) {
	base = normalizeLimitConfig(base)
	compiled := make([]routeOverride, 0, len(overrides))
	for _, override := range overrides {
		if strings.TrimSpace(override.Pattern) == "" {
			return nil, errors.New("route override requires a pattern")
		}
		cfg := mergeOverride(base, override)
		cfg = normalizeLimitConfig(cfg)
		mw, err := buildLimitChain(cfg)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, routeOverride{
			pattern:    override.Pattern,
			methods:    normalizeMethods(override.Methods),
			middleware: mw,
		})
	}
	baseline, err := buildLimitChain(base)
	if err != nil {
		return nil, err
	}
	return &limitsMiddleware{
		baseline:  baseline,
		overrides: compiled,
	}, nil
}

func mergeOverride(base limitConfig, override RouteOverride) limitConfig {
	cfg := base
	if override.MaxBodyBytes != nil {
		cfg.maxBodyBytes = *override.MaxBodyBytes
	}
	if override.QueryLimits != nil {
		cfg.queryLimits = *override.QueryLimits
		cfg.enableQueryGuard = true
	}
	if override.QueryLimitsEnabled != nil {
		cfg.enableQueryGuard = *override.QueryLimitsEnabled
	}
	if override.Timeout != nil {
		cfg.timeout = *override.Timeout
		cfg.enableTimeout = true
	}
	if override.TimeoutEnabled != nil {
		cfg.enableTimeout = *override.TimeoutEnabled
	}
	if override.RateLimit != nil {
		cfg.rateLimit = *override.RateLimit
		cfg.enableRateLimit = true
	}
	if override.RateLimitEnabled != nil {
		cfg.enableRateLimit = *override.RateLimitEnabled
	}
	return cfg
}

func normalizeLimitConfig(cfg limitConfig) limitConfig {
	ensureQueryErrorWriter(&cfg.queryLimits, cfg.errorWriter)
	if cfg.enableRateLimit && cfg.rateLimit.Key == nil {
		cfg.rateLimit.ClientIPResolver = cfg.resolver
	}
	return cfg
}

func buildLimitChain(cfg limitConfig) (func(http.Handler) http.Handler, error) {
	chain := make([]func(http.Handler) http.Handler, 0, 4)
	if cfg.enableTimeout && cfg.timeout > 0 {
		mw, err := timeoutmw.New(timeoutmw.Options{Timeout: cfg.timeout})
		if err != nil {
			return nil, fmt.Errorf("timeout middleware: %w", err)
		}
		chain = append(chain, mw.Middleware())
	}
	if cfg.enableRateLimit {
		mw, err := ratelimit.New(cfg.rateLimit)
		if err != nil {
			return nil, fmt.Errorf("rate limit middleware: %w", err)
		}
		chain = append(chain, mw.Middleware())
	}
	if cfg.maxBodyBytes > 0 {
		mw, err := maxbody.New(maxbody.Options{MaxBytes: cfg.maxBodyBytes})
		if err != nil {
			return nil, fmt.Errorf("max body middleware: %w", err)
		}
		chain = append(chain, mw.Middleware())
	}
	if cfg.enableQueryGuard {
		mw, err := querylimits.New(cfg.queryLimits)
		if err != nil {
			return nil, fmt.Errorf("query limits middleware: %w", err)
		}
		chain = append(chain, mw.Middleware())
	}
	return func(next http.Handler) http.Handler {
		for i := len(chain) - 1; i >= 0; i-- {
			next = chain[i](next)
		}
		return next
	}, nil
}

func methodMatches(method string, methods []string) bool {
	if len(methods) == 0 {
		return true
	}
	for _, m := range methods {
		if method == m {
			return true
		}
	}
	return false
}

func normalizeMethods(methods []string) []string {
	out := make([]string, 0, len(methods))
	for _, m := range methods {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		out = append(out, strings.ToUpper(m))
	}
	return out
}

func ensureQueryErrorWriter(opts *querylimits.Options, writer ErrorWriter) {
	if opts == nil || opts.ErrorWriter != nil || writer == nil {
		return
	}
	opts.ErrorWriter = func(w http.ResponseWriter, status int, p httpx.Problem) {
		writer(w, status, p)
	}
}
