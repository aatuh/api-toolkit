package bootstrap

import (
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/adapters/chi"
	"github.com/aatuh/api-toolkit/httpx/identity"
	recoverx "github.com/aatuh/api-toolkit/httpx/recover"
	"github.com/aatuh/api-toolkit/middleware/cors"
	jsonmw "github.com/aatuh/api-toolkit/middleware/json"
	maxbody "github.com/aatuh/api-toolkit/middleware/maxbody"
	metricsmw "github.com/aatuh/api-toolkit/middleware/metrics"
	oteltrace "github.com/aatuh/api-toolkit/middleware/oteltrace"
	rateln "github.com/aatuh/api-toolkit/middleware/ratelimit"
	requestlog "github.com/aatuh/api-toolkit/middleware/requestlog"
	securemw "github.com/aatuh/api-toolkit/middleware/secure"
	timeoutmw "github.com/aatuh/api-toolkit/middleware/timeout"
	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/specs"
)

// Profile describes a middleware stack and server options.
type Profile struct {
	Middlewares   []func(http.Handler) http.Handler
	ServerOptions []ServerOption
}

// Apply attaches the profile middlewares to the router.
func (p Profile) Apply(r ports.HTTPRouter) {
	if r == nil {
		return
	}
	if len(p.Middlewares) == 0 {
		return
	}
	r.Use(p.Middlewares...)
}

type profileConfig struct {
	log               ports.Logger
	metrics           metricsmw.MetricsRecorder
	rateLimit         rateln.Options
	enableRateLimit   bool
	corsOptions       ports.CORSOptions
	secureOptions     []securemw.Option
	timeout           time.Duration
	maxBodyBytes      int64
	jsonStrict        bool
	identityResolver  identity.Resolver
	requestLogOptions []requestlog.Option
	otelOptions       oteltrace.Options
}

// ProfileOption customizes profile defaults.
type ProfileOption func(*profileConfig)

// WithMetricsRecorder sets the metrics recorder.
func WithMetricsRecorder(rec metricsmw.MetricsRecorder) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.metrics = rec
	}
}

// WithRateLimitOptions overrides rate limiting settings.
func WithRateLimitOptions(opts rateln.Options) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.rateLimit = opts
	}
}

// WithRateLimitDisabled disables rate limiting.
func WithRateLimitDisabled() ProfileOption {
	return func(cfg *profileConfig) {
		cfg.enableRateLimit = false
	}
}

// WithCORSOptions overrides CORS options.
func WithCORSOptions(opts ports.CORSOptions) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.corsOptions = opts
	}
}

// WithSecureOptions appends secure middleware options.
func WithSecureOptions(opts ...securemw.Option) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.secureOptions = append(cfg.secureOptions, opts...)
	}
}

// WithRequestTimeout overrides per-request timeout.
func WithRequestTimeout(d time.Duration) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.timeout = d
	}
}

// WithMaxBodyBytes overrides max request body size.
func WithMaxBodyBytes(n int64) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.maxBodyBytes = n
	}
}

// WithJSONStrict toggles strict JSON parsing.
func WithJSONStrict(strict bool) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.jsonStrict = strict
	}
}

// WithIdentityResolver sets the trusted proxy resolver used by middleware.
func WithIdentityResolver(resolver identity.Resolver) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.identityResolver = resolver
	}
}

// WithRequestLogOptions appends request log options.
func WithRequestLogOptions(opts ...requestlog.Option) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.requestLogOptions = append(cfg.requestLogOptions, opts...)
	}
}

// WithOTelOptions overrides OpenTelemetry middleware options.
func WithOTelOptions(opts oteltrace.Options) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.otelOptions = opts
	}
}

// ProfileStrictAPI builds a hardened API profile.
func ProfileStrictAPI(log ports.Logger, opts ...ProfileOption) Profile {
	cfg := profileConfig{
		log:     log,
		metrics: metricsmw.NewPrometheusRecorder(nil, nil),
		rateLimit: rateln.Options{
			Capacity:    30,
			RefillRate:  15,
			RetryAfter:  time.Second,
			SkipEnabled: false,
		},
		enableRateLimit: true,
		corsOptions:     cors.DefaultOptions(),
		timeout:         5 * time.Second,
		maxBodyBytes:    1 << 20,
		jsonStrict:      true,
		identityResolver: identity.Resolver{
			HeaderPolicy: identity.HeaderPolicyBoth,
		},
		otelOptions: oteltrace.Options{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	secureOptions := []securemw.Option{
		securemw.WithResolver(cfg.identityResolver),
		securemw.WithCSPFunc(defaultCSPPolicy),
	}
	secureOptions = append(secureOptions, cfg.secureOptions...)

	corsh := cors.New()
	mw := chi.NewMiddleware()
	requestLogOpts := append([]requestlog.Option{
		requestlog.WithResolver(cfg.identityResolver),
	}, cfg.requestLogOptions...)

	chain := []func(http.Handler) http.Handler{
		mw.RequestID(),
		oteltrace.New(cfg.otelOptions).Middleware(),
		recoverx.Middleware(),
		corsh.Handler(cfg.corsOptions),
		securemw.New(secureOptions...).Middleware(),
	}
	if cfg.enableRateLimit {
		cfg.rateLimit.ClientIPResolver = cfg.identityResolver
		chain = append(chain, rateln.New(cfg.rateLimit).Middleware())
	}
	chain = append(chain,
		maxbody.New(cfg.maxBodyBytes).Middleware(),
		jsonmw.New(cfg.jsonStrict).Middleware(),
		timeoutmw.New(cfg.timeout).Middleware(),
		requestlog.New(cfg.log, requestLogOpts...).Middleware(),
		metricsmw.New(cfg.metrics).Middleware(),
	)

	return Profile{
		Middlewares: chain,
	}
}

// ProfileDev builds a developer-friendly profile with relaxed protections.
func ProfileDev(log ports.Logger, opts ...ProfileOption) Profile {
	cfg := profileConfig{
		log:             log,
		metrics:         metricsmw.NewPrometheusRecorder(nil, nil),
		enableRateLimit: false,
		corsOptions:     cors.DefaultOptions(),
		timeout:         30 * time.Second,
		maxBodyBytes:    4 << 20,
		jsonStrict:      false,
		identityResolver: identity.Resolver{
			HeaderPolicy: identity.HeaderPolicyBoth,
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	secureOptions := []securemw.Option{
		securemw.WithResolver(cfg.identityResolver),
		securemw.WithCSPFunc(devCSPPolicy),
	}
	secureOptions = append(secureOptions, cfg.secureOptions...)

	corsh := cors.New()
	mw := chi.NewMiddleware()
	requestLogOpts := append([]requestlog.Option{
		requestlog.WithResolver(cfg.identityResolver),
	}, cfg.requestLogOptions...)

	chain := []func(http.Handler) http.Handler{
		mw.RequestID(),
		oteltrace.New(cfg.otelOptions).Middleware(),
		recoverx.Middleware(),
		corsh.Handler(cfg.corsOptions),
		securemw.New(secureOptions...).Middleware(),
		maxbody.New(cfg.maxBodyBytes).Middleware(),
		jsonmw.New(cfg.jsonStrict).Middleware(),
		timeoutmw.New(cfg.timeout).Middleware(),
		requestlog.New(cfg.log, requestLogOpts...).Middleware(),
		metricsmw.New(cfg.metrics).Middleware(),
	}
	return Profile{
		Middlewares: chain,
	}
}

func defaultCSPPolicy(r *http.Request) string {
	if r == nil {
		return ""
	}
	path := r.URL.Path
	if strings.HasPrefix(path, specs.Docs) {
		return "default-src 'self' https://cdn.jsdelivr.net; " +
			"script-src 'self' https://cdn.jsdelivr.net; " +
			"style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; " +
			"img-src 'self' data: https://cdn.jsdelivr.net; " +
			"font-src https://cdn.jsdelivr.net"
	}
	return "default-src 'none'; frame-ancestors 'none'"
}

func devCSPPolicy(r *http.Request) string {
	if r == nil {
		return ""
	}
	if strings.HasPrefix(r.URL.Path, specs.Docs) {
		return "default-src 'self' https://cdn.jsdelivr.net 'unsafe-inline' 'unsafe-eval'; " +
			"script-src 'self' https://cdn.jsdelivr.net 'unsafe-inline' 'unsafe-eval'; " +
			"style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; " +
			"img-src 'self' data: https://cdn.jsdelivr.net; " +
			"font-src https://cdn.jsdelivr.net"
	}
	return "default-src 'self'; frame-ancestors 'none'"
}
