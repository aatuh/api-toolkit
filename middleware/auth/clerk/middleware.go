package clerk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/aatuh/api-toolkit/httpx"
	"github.com/aatuh/api-toolkit/ports"
	"github.com/golang-jwt/jwt/v5"
)

// Config controls Clerk JWT validation.
type Config struct {
	Enabled             bool
	JWKSURL             string
	Issuer              string
	Audience            string
	AllowedClockSkew    time.Duration
	JWKSRefreshTimeout  time.Duration
	JWKSRefreshInterval time.Duration
	SkipHeaderEnabled   bool
	SkipHeaderName      string
}

// Middleware validates Clerk-issued JWTs and stores the subject.
type Middleware struct {
	cfg     Config
	log     ports.Logger
	jwks    keyfunc.Keyfunc
	enabled bool
	skipHdr string
}

// NewMiddleware creates a middleware instance.
func NewMiddleware(ctx context.Context, cfg Config, log ports.Logger) (*Middleware, error) {
	mw := &Middleware{
		cfg:     cfg,
		log:     log,
		enabled: cfg.Enabled,
		skipHdr: strings.TrimSpace(cfg.SkipHeaderName),
	}
	if !cfg.Enabled {
		return mw, nil
	}
	if cfg.JWKSURL == "" || cfg.Issuer == "" || cfg.Audience == "" {
		return nil, fmt.Errorf("clerk middleware missing mandatory configuration")
	}
	override := keyfunc.Override{
		HTTPTimeout:       cfg.JWKSRefreshTimeout,
		RefreshInterval:   cfg.JWKSRefreshInterval,
		ValidationSkipAll: false,
	}
	jwks, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{cfg.JWKSURL}, override)
	if err != nil {
		return nil, fmt.Errorf("initializing clerk JWKS: %w", err)
	}
	mw.jwks = jwks
	return mw, nil
}

// Handler returns the http middleware.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if !m.enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.shouldSkip(r) {
			m.log.Warn("clerk auth skipped via header")
			next.ServeHTTP(w, r)
			return
		}
		subj, err := m.subjectFromRequest(r)
		if err != nil {
			m.log.Warn("clerk auth failed", "error", err.Error())
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
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
	if !m.enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.shouldSkip(r) {
			m.log.Warn("clerk auth skipped via header")
			next.ServeHTTP(w, r)
			return
		}
		if bearerToken(r.Header.Get("Authorization")) == "" {
			next.ServeHTTP(w, r)
			return
		}
		subj, err := m.subjectFromRequest(r)
		if err != nil {
			m.log.Warn("clerk auth failed", "error", err.Error())
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "invalid authentication token",
			})
			return
		}
		ctx := WithSubject(r.Context(), subj)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) subjectFromRequest(r *http.Request) (Subject, error) {
	tokenStr := bearerToken(r.Header.Get("Authorization"))
	if tokenStr == "" {
		return Subject{}, errors.New("missing bearer token")
	}
	if m.jwks == nil {
		return Subject{}, errors.New("jwks not configured")
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		m.jwks.Keyfunc,
		jwt.WithAudience(m.cfg.Audience),
		jwt.WithIssuer(m.cfg.Issuer),
		jwt.WithLeeway(m.cfg.AllowedClockSkew),
	)
	if err != nil {
		return Subject{}, fmt.Errorf("token parse: %w", err)
	}
	if !token.Valid {
		return Subject{}, errors.New("token invalid")
	}

	subj := Subject{
		UserID:   stringClaim(claims, "sub"),
		Email:    firstNonEmpty(stringClaim(claims, "email_address"), stringClaim(claims, "email")),
		First:    stringClaim(claims, "first_name"),
		Last:     stringClaim(claims, "last_name"),
		Language: stringClaim(claims, "preferred_language"),
	}
	if subj.UserID == "" {
		return Subject{}, errors.New("token missing subject")
	}
	return subj, nil
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
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

func (m *Middleware) shouldSkip(r *http.Request) bool {
	if !m.cfg.SkipHeaderEnabled {
		return false
	}
	if m.skipHdr == "" {
		return false
	}
	return r.Header.Get(m.skipHdr) != ""
}
