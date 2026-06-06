package jwt

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/v3/httpx/identity"
	"github.com/aatuh/api-toolkit/v3/middleware/auth/shared"
	"github.com/aatuh/api-toolkit/v3/ports"
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
	state, err := shared.PrepareValidationState(ctx, shared.ValidationConfig{
		Enabled:                   cfg.Enabled,
		ProviderName:              "jwt",
		JWKSDescriptor:            "jwks",
		JWKSURL:                   cfg.JWKSURL,
		Issuer:                    cfg.Issuer,
		Audience:                  cfg.Audience,
		AllowedAlgorithms:         cfg.AllowedAlgorithms,
		AllowedClockSkew:          cfg.AllowedClockSkew,
		JWKSRefreshTimeout:        cfg.JWKSRefreshTimeout,
		JWKSRefreshInterval:       cfg.JWKSRefreshInterval,
		RequiredClaims:            shared.ClaimRequirementsInput(cfg.RequiredClaims),
		AllowDangerousDevBypasses: cfg.AllowDangerousDevBypasses,
		SkipHeaderEnabled:         cfg.SkipHeaderEnabled,
		SkipHeaderName:            cfg.SkipHeaderName,
		SkipTrustedProxies:        cfg.SkipTrustedProxies,
	})
	if err != nil {
		return nil, err
	}
	return &Middleware{
		cfg:          cfg,
		log:          log,
		jwks:         state.JWKS,
		enabled:      state.Enabled,
		skipHdr:      state.SkipHeader,
		skipResolver: state.SkipResolver,
		allowedAlgs:  state.AllowedAlgorithms,
		claimReq:     normalizeClaimRequirements(cfg.RequiredClaims),
		cancel:       state.Cancel,
	}, nil
}

// Handler returns the http middleware enforcing authentication.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return shared.RequiredBearerHandler(
		next,
		m.enabled,
		m.log,
		m.shouldSkip,
		shared.HandlerMessages{
			SkipLog:       "jwt auth skipped via header",
			FailureLog:    "jwt auth failed",
			MissingDetail: "invalid or missing authentication token",
			InvalidDetail: "invalid or missing authentication token",
		},
		tokenFromRequest,
		func(ctx context.Context, token string) (context.Context, error) {
			subj, err := m.subjectFromToken(token)
			if err != nil {
				return nil, err
			}
			return WithSubject(ctx, subj), nil
		},
	)
}

// OptionalHandler attaches a subject when a valid token is present,
// but allows requests without authentication to continue.
func (m *Middleware) OptionalHandler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return shared.OptionalBearerHandler(
		next,
		m.enabled,
		m.log,
		m.shouldSkip,
		shared.HandlerMessages{
			SkipLog:       "jwt auth skipped via header",
			FailureLog:    "jwt auth failed",
			MissingDetail: "invalid authentication token",
			InvalidDetail: "invalid authentication token",
		},
		tokenFromRequest,
		func(ctx context.Context, token string) (context.Context, error) {
			subj, err := m.subjectFromToken(token)
			if err != nil {
				return nil, err
			}
			return WithSubject(ctx, subj), nil
		},
	)
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
	return shared.ParseBearerTokenValues(r.Header.Values("Authorization"))
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

func copyClaims(claims jwt.MapClaims) map[string]any {
	return shared.CopyClaims(claims)
}
