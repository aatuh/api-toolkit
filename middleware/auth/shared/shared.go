package shared

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/aatuh/api-toolkit/v3/httpx/identity"
)

// ClaimRequirementsInput configures required JWT claims.
type ClaimRequirementsInput struct {
	RequireSubject    *bool
	RequireExpiration *bool
	RequireIssuedAt   *bool
	RequireNotBefore  *bool
}

// ClaimRequirements is the normalized required-claim policy.
type ClaimRequirements struct {
	RequireSubject    bool
	RequireExpiration bool
	RequireIssuedAt   bool
	RequireNotBefore  bool
}

// TokenParserConfig configures JWT claim parsing and validation.
type TokenParserConfig struct {
	Audience          string
	Issuer            string
	AllowedClockSkew  time.Duration
	AllowedAlgorithms []string
	Requirements      ClaimRequirements
}

// SkipPolicy configures skip-header behavior for trusted proxies.
type SkipPolicy struct {
	Enabled                   bool
	AllowDangerousDevBypasses bool
	HeaderName                string
	Resolver                  identity.Resolver
}

// NormalizeAlgorithms canonicalizes the configured JWT algorithm allowlist.
func NormalizeAlgorithms(algs []string) ([]string, error) {
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

// NormalizeClaimRequirements applies the default required-claim policy.
func NormalizeClaimRequirements(input ClaimRequirementsInput) ClaimRequirements {
	out := ClaimRequirements{
		RequireSubject:    true,
		RequireExpiration: true,
		RequireIssuedAt:   false,
		RequireNotBefore:  false,
	}
	if input.RequireSubject != nil {
		out.RequireSubject = *input.RequireSubject
	}
	if input.RequireExpiration != nil {
		out.RequireExpiration = *input.RequireExpiration
	}
	if input.RequireIssuedAt != nil {
		out.RequireIssuedAt = *input.RequireIssuedAt
	}
	if input.RequireNotBefore != nil {
		out.RequireNotBefore = *input.RequireNotBefore
	}
	return out
}

// ValidateRequiredClaims enforces presence of the configured JWT claims.
func ValidateRequiredClaims(claims jwt.MapClaims, req ClaimRequirements) error {
	if req.RequireSubject && strings.TrimSpace(StringClaim(claims, "sub")) == "" {
		return errors.New("token missing subject")
	}
	if req.RequireExpiration {
		exp, err := claims.GetExpirationTime()
		if err != nil {
			return err
		}
		if exp == nil {
			return errors.New("token missing exp")
		}
	}
	if req.RequireIssuedAt {
		iat, err := claims.GetIssuedAt()
		if err != nil {
			return err
		}
		if iat == nil {
			return errors.New("token missing iat")
		}
	}
	if req.RequireNotBefore {
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

// ParseTokenClaims parses and validates JWT claims using the configured keyfunc.
func ParseTokenClaims(
	tokenStr string,
	keyfunc jwt.Keyfunc,
	cfg TokenParserConfig,
) (jwt.MapClaims, error) {
	if keyfunc == nil {
		return nil, errors.New("jwks not configured")
	}
	claims := jwt.MapClaims{}
	opts := []jwt.ParserOption{
		jwt.WithAudience(cfg.Audience),
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithLeeway(cfg.AllowedClockSkew),
	}
	if len(cfg.AllowedAlgorithms) > 0 {
		opts = append(opts, jwt.WithValidMethods(cfg.AllowedAlgorithms))
	}
	if cfg.Requirements.RequireExpiration {
		opts = append(opts, jwt.WithExpirationRequired())
	}
	if cfg.Requirements.RequireIssuedAt {
		opts = append(opts, jwt.WithIssuedAt())
	}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok || strings.TrimSpace(kid) == "" {
			return nil, errors.New("token missing kid")
		}
		return keyfunc(token)
	}, opts...)
	if err != nil {
		return nil, fmt.Errorf("token parse: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("token invalid")
	}
	if err := ValidateRequiredClaims(claims, cfg.Requirements); err != nil {
		return nil, err
	}
	return claims, nil
}

// ParseSkipTrustedProxies normalizes trusted proxy CIDRs for skip-header use.
func ParseSkipTrustedProxies(trustedProxies []string) (identity.Resolver, error) {
	prefixes, err := identity.ParseTrustedProxies(trustedProxies)
	if err != nil {
		return identity.Resolver{}, err
	}
	if len(prefixes) == 0 {
		return identity.Resolver{}, errors.New("skip header requires trusted proxies")
	}
	return identity.Resolver{TrustedProxies: prefixes}, nil
}

// ShouldSkipRequest reports whether auth should be skipped for the request.
func ShouldSkipRequest(r *http.Request, policy SkipPolicy) bool {
	if !policy.Enabled {
		return false
	}
	if strings.TrimSpace(policy.HeaderName) == "" {
		return false
	}
	if !policy.AllowDangerousDevBypasses {
		return false
	}
	if r == nil {
		return false
	}
	if !headerIsTrue(r.Header.Get(policy.HeaderName)) {
		return false
	}
	if len(policy.Resolver.TrustedProxies) == 0 {
		return false
	}
	return policy.Resolver.TrustsRemoteAddr(r.RemoteAddr)
}

// ParseBearerToken extracts a bearer token from the Authorization header.
func ParseBearerToken(header string) (string, bool, error) {
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

// ParseBearerTokenValues extracts a bearer token from Authorization header
// values and rejects duplicated headers instead of trusting the first value.
func ParseBearerTokenValues(values []string) (string, bool, error) {
	if len(values) == 0 {
		return "", false, nil
	}
	if len(values) > 1 {
		return "", true, errors.New("authorization header contains multiple values")
	}
	return ParseBearerToken(values[0])
}

// AuthErrorDetail converts header parse results into a stable log message.
func AuthErrorDetail(err error, present bool) string {
	if err != nil {
		return err.Error()
	}
	if !present {
		return "missing authorization header"
	}
	return "missing bearer token"
}

// StringClaim extracts a string claim when present.
func StringClaim(claims jwt.MapClaims, key string) string {
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

// FirstNonEmpty returns the first non-empty trimmed value.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// CopyClaims clones JWT claims into a plain map.
func CopyClaims(claims jwt.MapClaims) map[string]any {
	if len(claims) == 0 {
		return nil
	}
	out := make(map[string]any, len(claims))
	for k, v := range claims {
		out[k] = v
	}
	return out
}

func headerIsTrue(val string) bool {
	return strings.TrimSpace(val) == "true"
}
