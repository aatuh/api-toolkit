package secure

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v3/httpx/identity"
)

// Options configures the security header middleware.
type Options struct {
	ContentSecurityPolicy     string
	ContentSecurityPolicyFunc func(*http.Request) string
	PermissionsPolicy         string
	ReferrerPolicy            string
	FrameOptions              string
	ContentTypeOptions        string
	CrossOriginOpenerPolicy   string
	CrossOriginEmbedderPolicy string
	CrossOriginResourcePolicy string
	HSTSMaxAge                time.Duration
	HSTSIncludeSubdomains     bool
	HSTSPreload               bool
	Resolver                  identity.Resolver
}

// Option applies a functional option to Options.
type Option func(*Options)

// HeaderProfile configures a named security header preset.
type HeaderProfile string

const (
	// HeaderProfileAPIOnly targets JSON APIs and non-browser clients.
	HeaderProfileAPIOnly HeaderProfile = "api-only"
	// HeaderProfileDocsUI targets interactive API documentation UIs.
	HeaderProfileDocsUI HeaderProfile = "docs-ui"
	// HeaderProfileWebApp targets browser-facing web applications.
	HeaderProfileWebApp HeaderProfile = "web-app"
)

// CSPProfile configures a named CSP policy.
type CSPProfile string

const (
	// CSPProfileAPI is a conservative CSP for API-only services.
	CSPProfileAPI CSPProfile = "api"
	// CSPProfileAPIDocs allows Swagger UI assets for API documentation.
	CSPProfileAPIDocs CSPProfile = "api-docs"
	// CSPProfileWebApp is a baseline CSP for browser-facing apps.
	CSPProfileWebApp CSPProfile = "web-app"
)

const (
	referrerPolicyStrictOriginWhenCrossOrigin = "strict-origin-when-cross-origin"
	hstsMaxAgeRecommended                     = 63072000 * time.Second
	permissionsPolicyMinimal                  = "geolocation=(), camera=(), microphone=(), interest-cohort=()"
)

// APIOnly applies the API-only header profile.
func APIOnly() Option {
	return WithHeaderProfile(HeaderProfileAPIOnly)
}

// DocsUI applies the API documentation header profile.
func DocsUI() Option {
	return WithHeaderProfile(HeaderProfileDocsUI)
}

// WebApp applies the browser-facing web app header profile.
func WebApp() Option {
	return WithHeaderProfile(HeaderProfileWebApp)
}

// WithHeaderProfile applies a named header profile.
func WithHeaderProfile(profile HeaderProfile) Option {
	return func(o *Options) {
		applyHeaderProfile(o, profile)
	}
}

// CSPTemplate defines a CSP template string with placeholders.
type CSPTemplate string

const (
	// CSPTemplateWebApp is a baseline CSP template for browser apps.
	CSPTemplateWebApp CSPTemplate = "default-src 'self'; base-uri 'self'; object-src 'none'; " +
		"frame-ancestors 'none'; form-action 'self'; script-src 'self' {{nonce}} {{script-src}}; " +
		"style-src 'self' {{nonce}} {{style-src}}; img-src 'self' data: {{img-src}}; " +
		"connect-src 'self' {{connect-src}}; font-src 'self' {{font-src}}"
)

// CSPTemplateValues holds CSP placeholder replacements.
type CSPTemplateValues struct {
	Nonce      string
	ScriptSrc  []string
	StyleSrc   []string
	ImgSrc     []string
	ConnectSrc []string
	FontSrc    []string
}

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

// WithCSPProfile sets a named Content-Security-Policy profile.
func WithCSPProfile(profile CSPProfile) Option {
	return func(o *Options) {
		o.ContentSecurityPolicyFunc = nil
		o.ContentSecurityPolicy = CSPPolicy(profile)
	}
}

// WithCSPTemplate sets a Content-Security-Policy header from a template.
func WithCSPTemplate(template CSPTemplate, values CSPTemplateValues) Option {
	return func(o *Options) {
		o.ContentSecurityPolicyFunc = nil
		o.ContentSecurityPolicy = RenderCSPTemplate(template, values)
	}
}

// WithCSPTemplateFunc sets a per-request Content-Security-Policy from a template.
func WithCSPTemplateFunc(template CSPTemplate, fn func(*http.Request) CSPTemplateValues) Option {
	return func(o *Options) {
		if fn == nil {
			return
		}
		o.ContentSecurityPolicy = ""
		o.ContentSecurityPolicyFunc = func(r *http.Request) string {
			return RenderCSPTemplate(template, fn(r))
		}
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

// WithCrossOriginIsolation enables cross-origin isolation headers.
func WithCrossOriginIsolation() Option {
	return func(o *Options) {
		o.CrossOriginOpenerPolicy = "same-origin"
		o.CrossOriginEmbedderPolicy = "require-corp"
		o.CrossOriginResourcePolicy = "same-origin"
	}
}

// WithCOOP sets the Cross-Origin-Opener-Policy header value.
func WithCOOP(policy string) Option {
	return func(o *Options) {
		o.CrossOriginOpenerPolicy = policy
	}
}

// WithCOEP sets the Cross-Origin-Embedder-Policy header value.
func WithCOEP(policy string) Option {
	return func(o *Options) {
		o.CrossOriginEmbedderPolicy = policy
	}
}

// WithCORP sets the Cross-Origin-Resource-Policy header value.
func WithCORP(policy string) Option {
	return func(o *Options) {
		o.CrossOriginResourcePolicy = policy
	}
}

// WithCrossOriginPolicies sets cross-origin isolation header values explicitly.
func WithCrossOriginPolicies(coop, coep, corp string) Option {
	return func(o *Options) {
		o.CrossOriginOpenerPolicy = coop
		o.CrossOriginEmbedderPolicy = coep
		o.CrossOriginResourcePolicy = corp
	}
}

// Handler adds a minimal set of sane security headers.
type Handler struct {
	opts      Options
	cspPolicy func(*http.Request) string
	hstsValue string
}

// New constructs a security header middleware with optional overrides.
func New(opts ...Option) (*Handler, error) {
	cfg := Options{
		ContentSecurityPolicy: CSPPolicy(CSPProfileAPI),
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
	if cfg.HSTSMaxAge < 0 {
		return nil, errors.New("hsts max age must be non-negative")
	}
	return &Handler{
		opts:      cfg,
		cspPolicy: buildCSPPolicy(cfg),
		hstsValue: buildHSTSValue(cfg),
	}, nil
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
			if h.opts.CrossOriginOpenerPolicy != "" {
				setIfEmpty(w.Header(), "Cross-Origin-Opener-Policy", h.opts.CrossOriginOpenerPolicy)
			}
			if h.opts.CrossOriginEmbedderPolicy != "" {
				setIfEmpty(w.Header(), "Cross-Origin-Embedder-Policy", h.opts.CrossOriginEmbedderPolicy)
			}
			if h.opts.CrossOriginResourcePolicy != "" {
				setIfEmpty(w.Header(), "Cross-Origin-Resource-Policy", h.opts.CrossOriginResourcePolicy)
			}
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

// RenderCSPTemplate renders a CSP template with the provided values.
func RenderCSPTemplate(template CSPTemplate, values CSPTemplateValues) string {
	out := string(template)
	out = strings.ReplaceAll(out, "{{nonce}}", renderNonce(values.Nonce))
	out = strings.ReplaceAll(out, "{{script-src}}", joinCSPSources(values.ScriptSrc))
	out = strings.ReplaceAll(out, "{{style-src}}", joinCSPSources(values.StyleSrc))
	out = strings.ReplaceAll(out, "{{img-src}}", joinCSPSources(values.ImgSrc))
	out = strings.ReplaceAll(out, "{{connect-src}}", joinCSPSources(values.ConnectSrc))
	out = strings.ReplaceAll(out, "{{font-src}}", joinCSPSources(values.FontSrc))
	return normalizeCSP(out)
}

// CSPPolicy returns the named CSP policy string.
func CSPPolicy(profile CSPProfile) string {
	switch profile {
	case CSPProfileAPIDocs:
		return "default-src 'self' https://cdn.jsdelivr.net; " +
			"script-src 'self' https://cdn.jsdelivr.net; " +
			"style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; " +
			"img-src 'self' data: https://cdn.jsdelivr.net; " +
			"font-src https://cdn.jsdelivr.net; " +
			"frame-ancestors 'none'"
	case CSPProfileWebApp:
		return RenderCSPTemplate(CSPTemplateWebApp, CSPTemplateValues{})
	case CSPProfileAPI:
		return "default-src 'none'; frame-ancestors 'none'"
	default:
		return "default-src 'none'; frame-ancestors 'none'"
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

func applyHeaderProfile(o *Options, profile HeaderProfile) {
	if o == nil {
		return
	}
	switch profile {
	case HeaderProfileDocsUI:
		o.ContentSecurityPolicyFunc = nil
		o.ContentSecurityPolicy = CSPPolicy(CSPProfileAPIDocs)
		o.ReferrerPolicy = referrerPolicyStrictOriginWhenCrossOrigin
		o.FrameOptions = "DENY"
		o.ContentTypeOptions = "nosniff"
		o.PermissionsPolicy = permissionsPolicyMinimal
		o.HSTSMaxAge = hstsMaxAgeRecommended
		o.HSTSIncludeSubdomains = true
		o.HSTSPreload = false
	case HeaderProfileWebApp:
		o.ContentSecurityPolicyFunc = nil
		o.ContentSecurityPolicy = CSPPolicy(CSPProfileWebApp)
		o.ReferrerPolicy = referrerPolicyStrictOriginWhenCrossOrigin
		o.FrameOptions = "DENY"
		o.ContentTypeOptions = "nosniff"
		o.PermissionsPolicy = permissionsPolicyMinimal
		o.HSTSMaxAge = hstsMaxAgeRecommended
		o.HSTSIncludeSubdomains = true
		o.HSTSPreload = false
	case HeaderProfileAPIOnly:
		o.ContentSecurityPolicyFunc = nil
		o.ContentSecurityPolicy = CSPPolicy(CSPProfileAPI)
		o.ReferrerPolicy = referrerPolicyStrictOriginWhenCrossOrigin
		o.FrameOptions = "DENY"
		o.ContentTypeOptions = "nosniff"
		o.PermissionsPolicy = ""
		o.HSTSMaxAge = hstsMaxAgeRecommended
		o.HSTSIncludeSubdomains = true
		o.HSTSPreload = false
	}
}

func renderNonce(value string) string {
	nonce := strings.TrimSpace(value)
	if nonce == "" {
		return ""
	}
	return "'nonce-" + nonce + "'"
}

func joinCSPSources(values []string) string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return strings.Join(out, " ")
}

func normalizeCSP(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
