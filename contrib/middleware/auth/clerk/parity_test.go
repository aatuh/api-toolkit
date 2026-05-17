package clerk

import (
	"testing"

	"github.com/MicahParks/keyfunc/v3"

	"github.com/aatuh/api-toolkit/v3/httpx/identity"
	"github.com/aatuh/api-toolkit/v3/testutil/authtest"
)

func TestParityBearerTokenCases(t *testing.T) {
	authtest.RunBearerTokenCases(t, parseBearerToken)
}

func TestParityAlgorithmNormalizationCases(t *testing.T) {
	authtest.RunAlgorithmNormalizationCases(t, normalizeAlgorithms)
}

func TestParityClaimRequirementDefaults(t *testing.T) {
	authtest.RunClaimRequirementDefaultCases(
		t,
		func() claimRequirements { return normalizeClaimRequirements(ClaimRequirements{}) },
		func(req claimRequirements) authtest.ClaimRequirements {
			return authtest.ClaimRequirements{
				RequireSubject:    req.requireSubject,
				RequireExpiration: req.requireExpiration,
				RequireIssuedAt:   req.requireIssuedAt,
				RequireNotBefore:  req.requireNotBefore,
			}
		},
	)
}

func TestParityClaimValidationCases(t *testing.T) {
	authtest.RunClaimValidationCases(
		t,
		func() claimRequirements { return normalizeClaimRequirements(ClaimRequirements{}) },
		func(req authtest.ClaimRequirements) claimRequirements {
			return claimRequirements{
				requireSubject:    req.RequireSubject,
				requireExpiration: req.RequireExpiration,
				requireIssuedAt:   req.RequireIssuedAt,
				requireNotBefore:  req.RequireNotBefore,
			}
		},
		validateRequiredClaims,
	)
}

func TestParitySkipHeaderCases(t *testing.T) {
	prefixes, err := identity.ParseTrustedProxies([]string{"203.0.113.0/24"})
	if err != nil {
		t.Fatalf("parse trusted proxies: %v", err)
	}

	mw := &Middleware{
		cfg: Config{
			SkipHeaderEnabled:         true,
			AllowDangerousDevBypasses: true,
		},
		skipHdr:      "X-Debug-Skip",
		skipResolver: identity.Resolver{TrustedProxies: prefixes},
	}

	authtest.RunSkipHeaderCases(t, mw.shouldSkip)
}

func TestParitySubjectAlgorithmCases(t *testing.T) {
	authtest.RunSubjectAlgorithmCases(t, func(t *testing.T, kf keyfunc.Keyfunc) func(string) (string, error) {
		mw := &Middleware{
			cfg: Config{
				Issuer:   "https://issuer.example",
				Audience: "example",
			},
			jwks:        kf,
			allowedAlgs: []string{"RS256"},
			claimReq:    normalizeClaimRequirements(ClaimRequirements{}),
		}
		return func(token string) (string, error) {
			subject, err := mw.subjectFromToken(token)
			if err != nil {
				return "", err
			}
			return subject.UserID, nil
		}
	})
}
