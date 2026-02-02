package jwt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/v2/httpx"
	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// Config controls JWT validation.
type Config struct {
	Enabled  bool
	JWKSURL  string
	Issuer   string
	Audience string
	// AllowedAlgorithms constrains JWT signing methods (defaults to RS256).
	AllowedAlgorithms   []string
	AllowedClockSkew    time.Duration
	JWKSRefreshTimeout  time.Duration
	JWKSRefreshInterval time.Duration
	// RequiredClaims enforces presence of specific JWT claims (defaults to sub + exp).
	RequiredClaims ClaimRequirements
	// AllowDangerousDevBypasses enables skip headers only from trusted proxies.
	AllowDangerousDevBypasses bool
	SkipHeaderEnabled         bool
	SkipHeaderName            string
	// SkipTrustedProxies configures trusted CIDRs for skip header usage.
	SkipTrustedProxies []string
}

// ClaimRequirements configures required JWT claims (nil preserves defaults).
type ClaimRequirements struct {
	RequireSubject    *bool
	RequireExpiration *bool
	RequireIssuedAt   *bool
	RequireNotBefore  *bool
}

// Middleware validates JWTs and stores the subject.
type Middleware struct {
	cfg          Config
	log          ports.Logger
	jwks         keyfunc.Keyfunc
	enabled      bool
	skipHdr      string
	skipResolver identity.Resolver
	allowedAlgs  []string
	claimReq     claimRequirements
	cancel       context.CancelFunc
	closeOnce    sync.Once
}

// NewMiddleware creates a middleware instance.
// If JWKS refresh is enabled, Close() must be called or the passed context canceled on shutdown.
func NewMiddleware(ctx context.Context, cfg Config, log ports.Logger) (*Middleware, error) {
	if log == nil {
		log = ports.NopLogger{}
	}
	mw := &Middleware{
		cfg:     cfg,
		log:     log,
		enabled: cfg.Enabled,
		skipHdr: strings.TrimSpace(cfg.SkipHeaderName),
	}
	if !cfg.Enabled {
		return mw, nil
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if cfg.JWKSURL == "" || cfg.Issuer == "" || cfg.Audience == "" {
		return nil, fmt.Errorf("jwt middleware missing mandatory configuration")
	}
	allowedAlgs, err := normalizeAlgorithms(cfg.AllowedAlgorithms)
	if err != nil {
		return nil, fmt.Errorf("jwt allowed algorithms: %w", err)
	}
	if len(allowedAlgs) == 0 {
		allowedAlgs = []string{"RS256"}
	}
	mw.allowedAlgs = allowedAlgs
	mw.claimReq = normalizeClaimRequirements(cfg.RequiredClaims)

	if cfg.AllowDangerousDevBypasses && cfg.SkipHeaderEnabled {
		prefixes, err := identity.ParseTrustedProxies(cfg.SkipTrustedProxies)
		if err != nil {
			return nil, fmt.Errorf("jwt skip trusted proxies: %w", err)
		}
		if len(prefixes) == 0 {
			return nil, fmt.Errorf("jwt skip header requires trusted proxies")
		}
		mw.skipResolver = identity.Resolver{TrustedProxies: prefixes}
	}
	jwksCtx, cancel := context.WithCancel(ctx)
	mw.cancel = cancel
	override := keyfunc.Override{
		HTTPTimeout:       cfg.JWKSRefreshTimeout,
		RefreshInterval:   cfg.JWKSRefreshInterval,
		ValidationSkipAll: false,
	}
	jwks, err := keyfunc.NewDefaultOverrideCtx(jwksCtx, []string{cfg.JWKSURL}, override)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("initializing jwks: %w", err)
	}
	mw.jwks = jwks
	return mw, nil
}

// Handler returns the http middleware enforcing authentication.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	if !m.enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.shouldSkip(r) {
			m.log.Warn("jwt auth skipped via header")
			next.ServeHTTP(w, r)
			return
		}
		token, present, err := tokenFromRequest(r)
		if err != nil || !present {
			m.log.Warn("jwt auth failed", "error", authErrorDetail(err, present))
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "invalid or missing authentication token",
			})
			return
		}
		subj, err := m.subjectFromToken(token)
		if err != nil {
			m.log.Warn("jwt auth failed", "error", err.Error())
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "invalid or missing authentication token",
			})
			return
		}
		ctx := WithSubject(r.Context(), subj)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalHandler attaches a subject when a valid token is present,
// but allows requests without authentication to continue.
func (m *Middleware) OptionalHandler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	if !m.enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.shouldSkip(r) {
			m.log.Warn("jwt auth skipped via header")
			next.ServeHTTP(w, r)
			return
		}
		token, present, err := tokenFromRequest(r)
		if err != nil {
			m.log.Warn("jwt auth failed", "error", err.Error())
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "invalid authentication token",
			})
			return
		}
		if !present {
			next.ServeHTTP(w, r)
			return
		}
		subj, err := m.subjectFromToken(token)
		if err != nil {
			m.log.Warn("jwt auth failed", "error", err.Error())
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "invalid authentication token",
			})
			return
		}
		ctx := WithSubject(r.Context(), subj)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) subjectFromToken(tokenStr string) (Subject, error) {
	if m.jwks == nil {
		return Subject{}, errors.New("jwks not configured")
	}
	claims := jwt.MapClaims{}
	opts := []jwt.ParserOption{
		jwt.WithAudience(m.cfg.Audience),
		jwt.WithIssuer(m.cfg.Issuer),
		jwt.WithLeeway(m.cfg.AllowedClockSkew),
	}
	if len(m.allowedAlgs) > 0 {
		opts = append(opts, jwt.WithValidMethods(m.allowedAlgs))
	}
	if m.claimReq.requireExpiration {
		opts = append(opts, jwt.WithExpirationRequired())
	}
	if m.claimReq.requireIssuedAt {
		opts = append(opts, jwt.WithIssuedAt())
	}
	token, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		m.jwks.Keyfunc,
		opts...,
	)
	if err != nil {
		return Subject{}, fmt.Errorf("token parse: %w", err)
	}
	if !token.Valid {
		return Subject{}, errors.New("token invalid")
	}
	if err := validateRequiredClaims(claims, m.claimReq); err != nil {
		return Subject{}, err
	}
	subj := Subject{
		UserID:   stringClaim(claims, "sub"),
		Email:    firstNonEmpty(stringClaim(claims, "email"), stringClaim(claims, "email_address")),
		First:    firstNonEmpty(stringClaim(claims, "given_name"), stringClaim(claims, "first_name")),
		Last:     firstNonEmpty(stringClaim(claims, "family_name"), stringClaim(claims, "last_name")),
		Language: firstNonEmpty(stringClaim(claims, "locale"), stringClaim(claims, "preferred_language")),
		Claims:   copyClaims(claims),
	}
	return subj, nil
}

func tokenFromRequest(r *http.Request) (string, bool, error) {
	if r == nil {
		return "", false, errors.New("request is nil")
	}
	return parseBearerToken(r.Header.Get("Authorization"))
}

func parseBearerToken(header string) (string, bool, error) {
	if header == "" {
		return "", false, nil
	}
	if strings.Contains(header, ",") {
		return "", true, errors.New("authorization header contains multiple values")
	}
	if header != strings.TrimSpace(header) {
		return "", true, errors.New("authorization header has leading/trailing whitespace")
	}
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", true, errors.New("authorization scheme is not bearer")
	}
	token := header[len(prefix):]
	if token == "" {
		return "", true, errors.New("bearer token is empty")
	}
	if strings.ContainsAny(token, " \t") {
		return "", true, errors.New("bearer token contains whitespace")
	}
	return token, true, nil
}

func stringClaim(claims jwt.MapClaims, key string) string {
	if v, ok := claims[key]; ok {
		switch vv := v.(type) {
		case string:
			return vv
		case fmt.Stringer:
			return vv.String()
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

type claimRequirements struct {
	requireSubject    bool
	requireExpiration bool
	requireIssuedAt   bool
	requireNotBefore  bool
}

func normalizeClaimRequirements(req ClaimRequirements) claimRequirements {
	out := claimRequirements{
		requireSubject:    true,
		requireExpiration: true,
		requireIssuedAt:   false,
		requireNotBefore:  false,
	}
	if req.RequireSubject != nil {
		out.requireSubject = *req.RequireSubject
	}
	if req.RequireExpiration != nil {
		out.requireExpiration = *req.RequireExpiration
	}
	if req.RequireIssuedAt != nil {
		out.requireIssuedAt = *req.RequireIssuedAt
	}
	if req.RequireNotBefore != nil {
		out.requireNotBefore = *req.RequireNotBefore
	}
	return out
}

func validateRequiredClaims(claims jwt.MapClaims, req claimRequirements) error {
	if req.requireSubject && strings.TrimSpace(stringClaim(claims, "sub")) == "" {
		return errors.New("token missing subject")
	}
	if req.requireExpiration {
		exp, err := claims.GetExpirationTime()
		if err != nil {
			return err
		}
		if exp == nil {
			return errors.New("token missing exp")
		}
	}
	if req.requireIssuedAt {
		iat, err := claims.GetIssuedAt()
		if err != nil {
			return err
		}
		if iat == nil {
			return errors.New("token missing iat")
		}
	}
	if req.requireNotBefore {
		nbf, err := claims.GetNotBefore()
		if err != nil {
			return err
		}
		if nbf == nil {
			return errors.New("token missing nbf")
		}
	}
	return nil
}

func authErrorDetail(err error, present bool) string {
	if err != nil {
		return err.Error()
	}
	if !present {
		return "missing authorization header"
	}
	return "missing bearer token"
}

func (m *Middleware) shouldSkip(r *http.Request) bool {
	if !m.cfg.SkipHeaderEnabled {
		return false
	}
	if m.skipHdr == "" {
		return false
	}
	if !m.cfg.AllowDangerousDevBypasses {
		return false
	}
	if !headerIsTrue(r.Header.Get(m.skipHdr)) {
		return false
	}
	if len(m.skipResolver.TrustedProxies) == 0 {
		return false
	}
	if r == nil {
		return false
	}
	return m.skipResolver.TrustsRemoteAddr(r.RemoteAddr)
}

// Close stops background JWKS refresh work, if enabled.
func (m *Middleware) Close() {
	if m == nil {
		return
	}
	m.closeOnce.Do(func() {
		if m.cancel != nil {
			m.cancel()
		}
	})
}

func normalizeAlgorithms(algs []string) ([]string, error) {
	seen := make(map[string]struct{}, len(algs))
	out := make([]string, 0, len(algs))
	for _, raw := range algs {
		val := strings.ToUpper(strings.TrimSpace(raw))
		if val == "" {
			continue
		}
		if val == "NONE" {
			return nil, errors.New("algorithm none is not allowed")
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		out = append(out, val)
	}
	return out, nil
}

func headerIsTrue(val string) bool {
	return strings.TrimSpace(val) == "true"
}

func copyClaims(claims jwt.MapClaims) map[string]any {
	if len(claims) == 0 {
		return nil
	}
	out := make(map[string]any, len(claims))
	for k, v := range claims {
		out[k] = v
	}
	return out
}
