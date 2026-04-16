package bootstrap

import (
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v2/adapters/chi"
	"github.com/aatuh/api-toolkit/contrib/v2/middleware/cors"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v2/middleware/metrics"
	oteltrace "github.com/aatuh/api-toolkit/contrib/v2/middleware/oteltrace"
	requestlog "github.com/aatuh/api-toolkit/contrib/v2/middleware/requestlog"
	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	recoverx "github.com/aatuh/api-toolkit/v2/httpx/recover"
	jsonmw "github.com/aatuh/api-toolkit/v2/middleware/json"
	maxbody "github.com/aatuh/api-toolkit/v2/middleware/maxbody"
	querylimits "github.com/aatuh/api-toolkit/v2/middleware/querylimits"
	rateln "github.com/aatuh/api-toolkit/v2/middleware/ratelimit"
	securemw "github.com/aatuh/api-toolkit/v2/middleware/secure"
	timeoutmw "github.com/aatuh/api-toolkit/v2/middleware/timeout"
	"github.com/aatuh/api-toolkit/v2/ports"
	"github.com/aatuh/api-toolkit/v2/specs"
)

// Profile describes a middleware stack and server options.
type Profile struct {
	Middlewares   []func(http.Handler) http.Handler
	ServerOptions []ServerOption
}

// Apply attaches the profile middlewares to the router.
func (p Profile) Apply(r ports.HTTPRouter) {
	p.ApplyTo(r)
}

// ApplyTo attaches the profile middlewares to a minimal middleware chain.
func (p Profile) ApplyTo(r ports.MiddlewareChain) {
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
	queryLimits       querylimits.Options
	enableQueryLimits bool
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

// WithQueryLimitsOptions overrides query parameter limits.
func WithQueryLimitsOptions(opts querylimits.Options) ProfileOption {
	return func(cfg *profileConfig) {
		cfg.queryLimits = opts
		cfg.enableQueryLimits = true
	}
}

// WithQueryLimitsDisabled disables query parameter guardrails.
func WithQueryLimitsDisabled() ProfileOption {
	return func(cfg *profileConfig) {
		cfg.enableQueryLimits = false
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
func ProfileStrictAPI(log ports.Logger, opts ...ProfileOption) (Profile, error) {
	cfg := profileConfig{
		log: log,
		rateLimit: rateln.Options{
			Capacity:    30,
			RefillRate:  15,
			RetryAfter:  time.Second,
			SkipEnabled: false,
		},
		enableRateLimit:   true,
		corsOptions:       ports.CORSOptions{},
		timeout:           5 * time.Second,
		maxBodyBytes:      1 << 20,
		enableQueryLimits: true,
		jsonStrict:        true,
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
	if cfg.metrics == nil {
		cfg.metrics = metricsmw.NewPrometheusRecorder(nil, nil)
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

	traceMw, err := oteltrace.New(cfg.otelOptions)
	if err != nil {
		return Profile{}, err
	}
	secureMw, err := securemw.New(secureOptions...)
	if err != nil {
		return Profile{}, err
	}
	maxBodyMw, err := maxbody.New(maxbody.Options{MaxBytes: cfg.maxBodyBytes})
	if err != nil {
		return Profile{}, err
	}
	queryMw, err := querylimits.New(cfg.queryLimits)
	if err != nil {
		return Profile{}, err
	}
	jsonMw, err := jsonmw.New(jsonmw.Options{RequireJSON: cfg.jsonStrict})
	if err != nil {
		return Profile{}, err
	}
	timeoutMw, err := timeoutmw.New(timeoutmw.Options{Timeout: cfg.timeout})
	if err != nil {
		return Profile{}, err
	}
	requestLogMw, err := requestlog.New(cfg.log, requestLogOpts...)
	if err != nil {
		return Profile{}, err
	}
	metricsMw, err := metricsmw.New(metricsmw.Options{Recorder: cfg.metrics})
	if err != nil {
		return Profile{}, err
	}

	chain := []func(http.Handler) http.Handler{
		mw.RequestID(),
		traceMw.Middleware(),
		recoverx.New(recoverx.WithLogger(cfg.log)),
		corsMiddleware(corsh, cfg.corsOptions),
		secureMw.Middleware(),
	}
	if cfg.enableRateLimit {
		cfg.rateLimit.ClientIPResolver = cfg.identityResolver
		rateMw, err := rateln.New(cfg.rateLimit)
		if err != nil {
			return Profile{}, err
		}
		chain = append(chain, rateMw.Middleware())
	}
	chain = append(chain,
		maxBodyMw.Middleware(),
		queryLimitsMiddleware(cfg.enableQueryLimits, queryMw),
		jsonMw.Middleware(),
		timeoutMw.Middleware(),
		// Request logging and metrics stay inside recovery. Their implementations
		// are expected to emit from defer on panic paths so uncommitted panics can
		// still be observed as 500s and committed panics can be observed before
		// recovery aborts the request.
		requestLogMw.Middleware(),
		metricsMw.Middleware(),
	)

	return Profile{
		Middlewares: chain,
	}, nil
}

// ProfileDev builds a developer-friendly profile with relaxed protections.
func ProfileDev(log ports.Logger, opts ...ProfileOption) (Profile, error) {
	cfg := profileConfig{
		log:               log,
		enableRateLimit:   false,
		corsOptions:       cors.DefaultOptions(),
		timeout:           30 * time.Second,
		maxBodyBytes:      4 << 20,
		enableQueryLimits: false,
		jsonStrict:        false,
		identityResolver: identity.Resolver{
			HeaderPolicy: identity.HeaderPolicyBoth,
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.metrics == nil {
		cfg.metrics = metricsmw.NewPrometheusRecorder(nil, nil)
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

	traceMw, err := oteltrace.New(cfg.otelOptions)
	if err != nil {
		return Profile{}, err
	}
	secureMw, err := securemw.New(secureOptions...)
	if err != nil {
		return Profile{}, err
	}
	maxBodyMw, err := maxbody.New(maxbody.Options{MaxBytes: cfg.maxBodyBytes})
	if err != nil {
		return Profile{}, err
	}
	queryMw, err := querylimits.New(cfg.queryLimits)
	if err != nil {
		return Profile{}, err
	}
	jsonMw, err := jsonmw.New(jsonmw.Options{RequireJSON: cfg.jsonStrict})
	if err != nil {
		return Profile{}, err
	}
	timeoutMw, err := timeoutmw.New(timeoutmw.Options{Timeout: cfg.timeout})
	if err != nil {
		return Profile{}, err
	}
	requestLogMw, err := requestlog.New(cfg.log, requestLogOpts...)
	if err != nil {
		return Profile{}, err
	}
	metricsMw, err := metricsmw.New(metricsmw.Options{Recorder: cfg.metrics})
	if err != nil {
		return Profile{}, err
	}

	chain := []func(http.Handler) http.Handler{
		mw.RequestID(),
		traceMw.Middleware(),
		recoverx.New(recoverx.WithLogger(cfg.log)),
		corsMiddleware(corsh, cfg.corsOptions),
		secureMw.Middleware(),
		maxBodyMw.Middleware(),
		queryLimitsMiddleware(cfg.enableQueryLimits, queryMw),
		jsonMw.Middleware(),
		timeoutMw.Middleware(),
		requestLogMw.Middleware(),
		metricsMw.Middleware(),
	}
	return Profile{
		Middlewares: chain,
	}, nil
}

func defaultCSPPolicy(r *http.Request) string {
	if r == nil {
		return ""
	}
	path := r.URL.Path
	if strings.HasPrefix(path, specs.Docs) {
		return securemw.CSPPolicy(securemw.CSPProfileAPIDocs)
	}
	return securemw.CSPPolicy(securemw.CSPProfileAPI)
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

func queryLimitsMiddleware(enabled bool, mw *querylimits.Middleware) func(http.Handler) http.Handler {
	if !enabled || mw == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return mw.Middleware()
}

func corsMiddleware(handler ports.CORSHandler, opts ports.CORSOptions) func(http.Handler) http.Handler {
	if handler == nil || len(opts.AllowedOrigins) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return handler.Handler(opts)
}
