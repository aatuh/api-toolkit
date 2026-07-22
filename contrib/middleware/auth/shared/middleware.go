package shared

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/httpx/identity"
	"github.com/aatuh/api-toolkit/v4/ports"
)

// ValidationConfig configures shared JWT/JWKS middleware setup.
type ValidationConfig struct {
	Enabled                   bool
	ProviderName              string
	JWKSDescriptor            string
	JWKSURL                   string
	Issuer                    string
	Audience                  string
	AllowedAlgorithms         []string
	AllowedClockSkew          time.Duration
	JWKSRefreshTimeout        time.Duration
	JWKSRefreshInterval       time.Duration
	RequiredClaims            ClaimRequirementsInput
	AllowDangerousDevBypasses bool
	SkipHeaderEnabled         bool
	SkipHeaderName            string
	SkipTrustedProxies        []string
}

// ValidationState contains the prepared runtime for JWT validation middleware.
type ValidationState struct {
	Enabled           bool
	SkipHeader        string
	SkipResolver      identity.Resolver
	AllowedAlgorithms []string
	ClaimRequirements ClaimRequirements
	JWKS              keyfunc.Keyfunc
	Cancel            context.CancelFunc
}

// HandlerMessages configures shared auth middleware logging and responses.
type HandlerMessages struct {
	SkipLog       string
	FailureLog    string
	MissingDetail string
	InvalidDetail string
}

// SubjectContextFunc resolves the authenticated request context from a token.
type SubjectContextFunc func(ctx context.Context, token string) (context.Context, error)

// TokenFromRequestFunc extracts a bearer token from an HTTP request.
type TokenFromRequestFunc func(r *http.Request) (string, bool, error)

// PrepareValidationState normalizes shared middleware runtime configuration.
func PrepareValidationState(ctx context.Context, cfg ValidationConfig) (ValidationState, error) {
	state := ValidationState{
		Enabled:    cfg.Enabled,
		SkipHeader: strings.TrimSpace(cfg.SkipHeaderName),
	}
	if !cfg.Enabled {
		return state, nil
	}
	if ctx == nil {
		return ValidationState{}, fmt.Errorf("context is required")
	}
	if cfg.JWKSURL == "" || cfg.Issuer == "" || cfg.Audience == "" {
		return ValidationState{}, fmt.Errorf("%s middleware missing mandatory configuration", cfg.ProviderName)
	}
	allowedAlgs, err := NormalizeAlgorithms(cfg.AllowedAlgorithms)
	if err != nil {
		return ValidationState{}, fmt.Errorf("%s allowed algorithms: %w", cfg.ProviderName, err)
	}
	if len(allowedAlgs) == 0 {
		allowedAlgs = []string{"RS256"}
	}
	state.AllowedAlgorithms = allowedAlgs
	state.ClaimRequirements = NormalizeClaimRequirements(cfg.RequiredClaims)

	if cfg.AllowDangerousDevBypasses && cfg.SkipHeaderEnabled {
		resolver, err := ParseSkipTrustedProxies(cfg.SkipTrustedProxies)
		if err != nil {
			if err.Error() == "skip header requires trusted proxies" {
				return ValidationState{}, fmt.Errorf("%s skip header requires trusted proxies", cfg.ProviderName)
			}
			return ValidationState{}, fmt.Errorf("%s skip trusted proxies: %w", cfg.ProviderName, err)
		}
		state.SkipResolver = resolver
	}

	jwksCtx, cancel := context.WithCancel(ctx)
	state.Cancel = cancel
	override := keyfunc.Override{
		HTTPTimeout:       cfg.JWKSRefreshTimeout,
		RefreshInterval:   cfg.JWKSRefreshInterval,
		ValidationSkipAll: false,
	}
	jwks, err := keyfunc.NewDefaultOverrideCtx(jwksCtx, []string{cfg.JWKSURL}, override)
	if err != nil {
		cancel()
		return ValidationState{}, fmt.Errorf("initializing %s: %w", cfg.JWKSDescriptor, err)
	}
	state.JWKS = jwks
	return state, nil
}

// RequiredBearerHandler enforces authentication and injects subject context.
func RequiredBearerHandler(
	next http.Handler,
	enabled bool,
	log ports.Logger,
	shouldSkip func(*http.Request) bool,
	messages HandlerMessages,
	tokenFromRequest TokenFromRequestFunc,
	subjectContext SubjectContextFunc,
) http.Handler {
	if !enabled {
		return next
	}
	if log == nil {
		log = ports.NopLogger{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldSkip != nil && shouldSkip(r) {
			log.Warn(messages.SkipLog)
			next.ServeHTTP(w, r)
			return
		}
		token, present, err := tokenFromRequest(r)
		if err != nil || !present {
			log.Warn(messages.FailureLog, "error", AuthErrorDetail(err, present))
			writeUnauthorized(w, messages.MissingDetail)
			return
		}
		ctx, err := subjectContext(r.Context(), token)
		if err != nil {
			log.Warn(messages.FailureLog, "error", err.Error())
			writeUnauthorized(w, messages.InvalidDetail)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalBearerHandler injects subject context when a valid token is present.
func OptionalBearerHandler(
	next http.Handler,
	enabled bool,
	log ports.Logger,
	shouldSkip func(*http.Request) bool,
	messages HandlerMessages,
	tokenFromRequest TokenFromRequestFunc,
	subjectContext SubjectContextFunc,
) http.Handler {
	if !enabled {
		return next
	}
	if log == nil {
		log = ports.NopLogger{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldSkip != nil && shouldSkip(r) {
			log.Warn(messages.SkipLog)
			next.ServeHTTP(w, r)
			return
		}
		token, present, err := tokenFromRequest(r)
		if err != nil {
			log.Warn(messages.FailureLog, "error", err.Error())
			writeUnauthorized(w, messages.InvalidDetail)
			return
		}
		if !present {
			next.ServeHTTP(w, r)
			return
		}
		ctx, err := subjectContext(r.Context(), token)
		if err != nil {
			log.Warn(messages.FailureLog, "error", err.Error())
			writeUnauthorized(w, messages.InvalidDetail)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeUnauthorized(w http.ResponseWriter, detail string) {
	httpx.WriteProblemChecked(w, http.StatusUnauthorized, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeUnauthorized),
		Title:  http.StatusText(http.StatusUnauthorized),
		Detail: detail,
	})
}
