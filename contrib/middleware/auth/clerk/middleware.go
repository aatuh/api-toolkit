package clerk

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
	"github.com/aatuh/api-toolkit/v2/middleware/auth/shared"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// Config controls Clerk JWT validation.
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

// Middleware validates Clerk-issued JWTs and stores the subject.
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
		return nil, fmt.Errorf("clerk middleware missing mandatory configuration")
	}
	allowedAlgs, err := normalizeAlgorithms(cfg.AllowedAlgorithms)
	if err != nil {
		return nil, fmt.Errorf("clerk allowed algorithms: %w", err)
	}
	if len(allowedAlgs) == 0 {
		allowedAlgs = []string{"RS256"}
	}
	mw.allowedAlgs = allowedAlgs
	mw.claimReq = normalizeClaimRequirements(cfg.RequiredClaims)

	if cfg.AllowDangerousDevBypasses && cfg.SkipHeaderEnabled {
		resolver, err := shared.ParseSkipTrustedProxies(cfg.SkipTrustedProxies)
		if err != nil {
			if err.Error() == "skip header requires trusted proxies" {
				return nil, fmt.Errorf("clerk skip header requires trusted proxies")
			}
			return nil, fmt.Errorf("clerk skip trusted proxies: %w", err)
		}
		mw.skipResolver = resolver
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
		return nil, fmt.Errorf("initializing clerk JWKS: %w", err)
	}
	mw.jwks = jwks
	return mw, nil
}

// Handler returns the http middleware.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	if !m.enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.shouldSkip(r) {
			m.log.Warn("clerk auth skipped via header")
			next.ServeHTTP(w, r)
			return
		}
		token, present, err := tokenFromRequest(r)
		if err != nil || !present {
			m.log.Warn("clerk auth failed", "error", authErrorDetail(err, present))
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
				Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "invalid or missing authentication token",
			})
			return
		}
		subj, err := m.subjectFromToken(token)
		if err != nil {
			m.log.Warn("clerk auth failed", "error", err.Error())
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
			m.log.Warn("clerk auth skipped via header")
			next.ServeHTTP(w, r)
			return
		}
		token, present, err := tokenFromRequest(r)
		if err != nil {
			m.log.Warn("clerk auth failed", "error", err.Error())
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
			m.log.Warn("clerk auth failed", "error", err.Error())
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
	claims, err := shared.ParseTokenClaims(tokenStr, m.jwks.Keyfunc, shared.TokenParserConfig{
		Audience:          m.cfg.Audience,
		Issuer:            m.cfg.Issuer,
		AllowedClockSkew:  m.cfg.AllowedClockSkew,
		AllowedAlgorithms: m.allowedAlgs,
		Requirements:      m.claimReq.shared(),
	})
	if err != nil {
		return Subject{}, err
	}

	subj := Subject{
		UserID:   stringClaim(claims, "sub"),
		Email:    firstNonEmpty(stringClaim(claims, "email_address"), stringClaim(claims, "email")),
		First:    stringClaim(claims, "first_name"),
		Last:     stringClaim(claims, "last_name"),
		Language: stringClaim(claims, "preferred_language"),
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
	return shared.ParseBearerToken(header)
}

func stringClaim(claims jwt.MapClaims, key string) string {
	return shared.StringClaim(claims, key)
}

func firstNonEmpty(values ...string) string {
	return shared.FirstNonEmpty(values...)
}

type claimRequirements struct {
	requireSubject    bool
	requireExpiration bool
	requireIssuedAt   bool
	requireNotBefore  bool
}

func normalizeClaimRequirements(req ClaimRequirements) claimRequirements {
	return claimRequirementsFromShared(shared.NormalizeClaimRequirements(shared.ClaimRequirementsInput{
		RequireSubject:    req.RequireSubject,
		RequireExpiration: req.RequireExpiration,
		RequireIssuedAt:   req.RequireIssuedAt,
		RequireNotBefore:  req.RequireNotBefore,
	}))
}

func claimRequirementsFromShared(req shared.ClaimRequirements) claimRequirements {
	return claimRequirements{
		requireSubject:    req.RequireSubject,
		requireExpiration: req.RequireExpiration,
		requireIssuedAt:   req.RequireIssuedAt,
		requireNotBefore:  req.RequireNotBefore,
	}
}

func (r claimRequirements) shared() shared.ClaimRequirements {
	return shared.ClaimRequirements{
		RequireSubject:    r.requireSubject,
		RequireExpiration: r.requireExpiration,
		RequireIssuedAt:   r.requireIssuedAt,
		RequireNotBefore:  r.requireNotBefore,
	}
}

func validateRequiredClaims(claims jwt.MapClaims, req claimRequirements) error {
	return shared.ValidateRequiredClaims(claims, req.shared())
}

func authErrorDetail(err error, present bool) string {
	return shared.AuthErrorDetail(err, present)
}

func (m *Middleware) shouldSkip(r *http.Request) bool {
	return shared.ShouldSkipRequest(r, shared.SkipPolicy{
		Enabled:                   m.cfg.SkipHeaderEnabled,
		AllowDangerousDevBypasses: m.cfg.AllowDangerousDevBypasses,
		HeaderName:                m.skipHdr,
		Resolver:                  m.skipResolver,
	})
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
	return shared.NormalizeAlgorithms(algs)
}
