package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/shared"
	"github.com/aatuh/api-toolkit/v4/httpx/identity"
	"github.com/aatuh/api-toolkit/v4/ports"
)

const (
	defaultTenantClaim = "tenant_id"
	defaultScopeClaim  = "scope"
)

// Config controls provider-neutral OIDC token validation.
type Config struct {
	Enabled      bool
	Issuer       string
	Audience     string
	DiscoveryURL string
	JWKSURL      string
	// TenantClaim maps the tenant/org claim into Subject.TenantID. Defaults to tenant_id.
	TenantClaim string
	// ScopeClaim maps the scope/permission claim into Subject.Scope. Defaults to scope.
	ScopeClaim string
	// DiscoveryHTTPClient overrides the HTTP client used for OIDC discovery.
	DiscoveryHTTPClient *http.Client
	// AllowedAlgorithms constrains JWT signing methods. Defaults to RS256.
	AllowedAlgorithms   []string
	AllowedClockSkew    time.Duration
	JWKSRefreshTimeout  time.Duration
	JWKSRefreshInterval time.Duration
	// RequiredClaims enforces presence of specific JWT claims. Defaults to sub + exp.
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

// Middleware validates OIDC bearer tokens and stores the subject.
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

// NewMiddleware creates an OIDC middleware instance.
// If JWKS refresh is enabled, Close must be called or the passed context canceled on shutdown.
func NewMiddleware(ctx context.Context, cfg Config, log ports.Logger) (*Middleware, error) {
	if log == nil {
		log = ports.NopLogger{}
	}
	cfg = normalizeConfig(cfg)
	if cfg.Enabled {
		if strings.TrimSpace(cfg.Issuer) == "" || strings.TrimSpace(cfg.Audience) == "" {
			return nil, errors.New("oidc middleware missing mandatory configuration")
		}
		if err := validateClaimName(cfg.TenantClaim); err != nil {
			return nil, fmt.Errorf("oidc tenant claim: %w", err)
		}
		if err := validateClaimName(cfg.ScopeClaim); err != nil {
			return nil, fmt.Errorf("oidc scope claim: %w", err)
		}
		jwksURL, err := ResolveJWKSURL(ctx, cfg, cfg.DiscoveryHTTPClient)
		if err != nil {
			return nil, err
		}
		cfg.JWKSURL = jwksURL
	}
	state, err := shared.PrepareValidationState(ctx, shared.ValidationConfig{
		Enabled:                   cfg.Enabled,
		ProviderName:              "oidc",
		JWKSDescriptor:            "oidc JWKS",
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

// Handler returns the HTTP middleware enforcing authentication.
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
			SkipLog:       "oidc auth skipped via header",
			FailureLog:    "oidc auth failed",
			MissingDetail: "invalid or missing authentication token",
			InvalidDetail: "invalid or missing authentication token",
		},
		tokenFromRequest,
		func(ctx context.Context, token string) (context.Context, error) {
			subj, err := m.subjectFromTokenContext(ctx, token)
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
			SkipLog:       "oidc auth skipped via header",
			FailureLog:    "oidc auth failed",
			MissingDetail: "invalid authentication token",
			InvalidDetail: "invalid authentication token",
		},
		tokenFromRequest,
		func(ctx context.Context, token string) (context.Context, error) {
			subj, err := m.subjectFromTokenContext(ctx, token)
			if err != nil {
				return nil, err
			}
			return WithSubject(ctx, subj), nil
		},
	)
}

// ResolveJWKSURL returns the configured JWKS URL or discovers it from OIDC metadata.
func ResolveJWKSURL(ctx context.Context, cfg Config, client *http.Client) (string, error) {
	jwksURL := strings.TrimSpace(cfg.JWKSURL)
	if jwksURL != "" {
		return jwksURL, nil
	}
	if ctx == nil {
		return "", errors.New("context is required")
	}
	discoveryURL := strings.TrimSpace(cfg.DiscoveryURL)
	if discoveryURL == "" {
		issuer := strings.TrimRight(strings.TrimSpace(cfg.Issuer), "/")
		if issuer == "" {
			return "", errors.New("oidc middleware missing mandatory configuration")
		}
		discoveryURL = issuer + "/.well-known/openid-configuration"
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", fmt.Errorf("oidc discovery request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oidc discovery failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oidc discovery failed: status %d", resp.StatusCode)
	}
	var doc discoveryDocument
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&doc); err != nil {
		return "", fmt.Errorf("oidc discovery decode: %w", err)
	}
	if strings.TrimSpace(doc.Issuer) != strings.TrimSpace(cfg.Issuer) {
		return "", fmt.Errorf("oidc discovery issuer mismatch")
	}
	if strings.TrimSpace(doc.JWKSURI) == "" {
		return "", fmt.Errorf("oidc discovery missing jwks_uri")
	}
	return strings.TrimSpace(doc.JWKSURI), nil
}

type discoveryDocument struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

func (m *Middleware) subjectFromToken(tokenStr string) (Subject, error) {
	return m.subjectFromTokenContext(context.Background(), tokenStr)
}

func (m *Middleware) subjectFromTokenContext(ctx context.Context, tokenStr string) (Subject, error) {
	if m.jwks == nil {
		return Subject{}, errors.New("jwks not configured")
	}
	if ctx == nil {
		return Subject{}, errors.New("oidc jwt verification context is required")
	}
	claims, err := shared.ParseTokenClaims(tokenStr, m.jwks.KeyfuncCtx(ctx), shared.TokenParserConfig{
		Audience:          m.cfg.Audience,
		Issuer:            m.cfg.Issuer,
		AllowedClockSkew:  m.cfg.AllowedClockSkew,
		AllowedAlgorithms: m.allowedAlgs,
		Requirements:      m.claimReq.shared(),
	})
	if err != nil {
		return Subject{}, err
	}
	return Subject{
		UserID:   stringClaim(claims, "sub"),
		Email:    stringClaim(claims, "email"),
		TenantID: claimAsString(claims, m.cfg.TenantClaim),
		Scope:    claimAsString(claims, m.cfg.ScopeClaim),
		Claims:   copyClaims(claims),
	}, nil
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

func claimAsString(claims jwt.MapClaims, key string) string {
	if strings.TrimSpace(key) == "" {
		return ""
	}
	value, ok := claims[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []string:
		return strings.Join(v, " ")
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
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

func copyClaims(claims jwt.MapClaims) map[string]any {
	return shared.CopyClaims(claims)
}

func normalizeConfig(cfg Config) Config {
	cfg.Issuer = strings.TrimSpace(cfg.Issuer)
	cfg.Audience = strings.TrimSpace(cfg.Audience)
	cfg.JWKSURL = strings.TrimSpace(cfg.JWKSURL)
	cfg.DiscoveryURL = strings.TrimSpace(cfg.DiscoveryURL)
	cfg.TenantClaim = strings.TrimSpace(cfg.TenantClaim)
	if cfg.TenantClaim == "" {
		cfg.TenantClaim = defaultTenantClaim
	}
	cfg.ScopeClaim = strings.TrimSpace(cfg.ScopeClaim)
	if cfg.ScopeClaim == "" {
		cfg.ScopeClaim = defaultScopeClaim
	}
	return cfg
}

func validateClaimName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("claim name is required")
	}
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == '.' {
			return errors.New("claim name must be a single safe claim key")
		}
	}
	return nil
}
