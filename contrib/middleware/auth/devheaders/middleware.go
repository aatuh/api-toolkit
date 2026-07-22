package devheaders

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	jwtauth "github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/jwt"
	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/httpx/identity"
	"github.com/aatuh/api-toolkit/v4/ports"
)

// Config controls dev header-based auth.
type Config struct {
	Enabled         bool
	UserIDHeader    string
	EmailHeader     string
	FirstNameHeader string
	LastNameHeader  string
	DefaultLanguage string
	// AllowDangerousDevBypasses must be explicitly enabled for debug-header auth.
	AllowDangerousDevBypasses bool
	// TrustedProxies is a comma-separated list of direct peers allowed to supply
	// debug auth headers.
	TrustedProxies string
}

// Middleware injects an auth subject in local environments without a JWT provider.
type Middleware struct {
	cfg Config
	log ports.Logger
}

// New constructs the middleware.
func New(cfg Config, log ports.Logger) (*Middleware, error) {
	if log == nil {
		log = ports.NopLogger{}
	}
	cfg.UserIDHeader = strings.TrimSpace(cfg.UserIDHeader)
	cfg.EmailHeader = strings.TrimSpace(cfg.EmailHeader)
	cfg.FirstNameHeader = strings.TrimSpace(cfg.FirstNameHeader)
	cfg.LastNameHeader = strings.TrimSpace(cfg.LastNameHeader)
	cfg.DefaultLanguage = strings.TrimSpace(cfg.DefaultLanguage)
	cfg.TrustedProxies = strings.TrimSpace(cfg.TrustedProxies)
	if cfg.Enabled && cfg.UserIDHeader == "" {
		return nil, errors.New("user id header is required when dev auth is enabled")
	}
	if _, err := trustedProxyResolver(cfg); err != nil {
		return nil, err
	}
	return &Middleware{cfg: cfg, log: log}, nil
}

// Handler attaches a subject from debug headers if JWT auth is disabled.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	if !m.cfg.Enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := jwtauth.SubjectFromContext(r.Context()); ok {
			next.ServeHTTP(w, r)
			return
		}
		userID := strings.TrimSpace(r.Header.Get(m.cfg.UserIDHeader))
		if userID == "" {
			httpx.WriteProblemChecked(w, http.StatusUnauthorized, httpx.Problem{
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "missing development auth headers",
			})
			return
		}
		if !m.trustsRemoteAddr(r.RemoteAddr) {
			httpx.WriteProblemChecked(w, http.StatusUnauthorized, httpx.Problem{
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "development auth headers are not allowed from untrusted remote addresses",
			})
			return
		}
		subj := jwtauth.Subject{
			UserID:   userID,
			Email:    strings.TrimSpace(r.Header.Get(m.cfg.EmailHeader)),
			First:    r.Header.Get(m.cfg.FirstNameHeader),
			Last:     r.Header.Get(m.cfg.LastNameHeader),
			Language: m.cfg.DefaultLanguage,
		}
		ctx := jwtauth.WithSubject(r.Context(), subj)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalHandler attaches a subject from debug headers if present,
// otherwise allows anonymous access.
func (m *Middleware) OptionalHandler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	if !m.cfg.Enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := jwtauth.SubjectFromContext(r.Context()); ok {
			next.ServeHTTP(w, r)
			return
		}
		userID := strings.TrimSpace(r.Header.Get(m.cfg.UserIDHeader))
		if userID == "" {
			next.ServeHTTP(w, r)
			return
		}
		if !m.trustsRemoteAddr(r.RemoteAddr) {
			httpx.WriteProblemChecked(w, http.StatusUnauthorized, httpx.Problem{
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "development auth headers are not allowed from untrusted remote addresses",
			})
			return
		}
		subj := jwtauth.Subject{
			UserID:   userID,
			Email:    strings.TrimSpace(r.Header.Get(m.cfg.EmailHeader)),
			First:    r.Header.Get(m.cfg.FirstNameHeader),
			Last:     r.Header.Get(m.cfg.LastNameHeader),
			Language: m.cfg.DefaultLanguage,
		}
		ctx := jwtauth.WithSubject(r.Context(), subj)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func trustedProxyResolver(cfg Config) (identity.Resolver, error) {
	if !cfg.Enabled {
		return identity.Resolver{}, nil
	}
	if !cfg.AllowDangerousDevBypasses {
		return identity.Resolver{}, errors.New("dangerous dev bypasses must be explicitly allowed when dev auth is enabled")
	}
	prefixes, err := identity.ParseTrustedProxies(splitTrustedProxies(cfg.TrustedProxies))
	if err != nil {
		return identity.Resolver{}, fmt.Errorf("dev auth trusted proxies: %w", err)
	}
	if len(prefixes) == 0 {
		return identity.Resolver{}, errors.New("trusted proxies are required when dev auth is enabled")
	}
	return identity.Resolver{TrustedProxies: prefixes}, nil
}

func (m *Middleware) trustsRemoteAddr(remoteAddr string) bool {
	resolver, err := trustedProxyResolver(m.cfg)
	if err != nil {
		return false
	}
	return resolver.TrustsRemoteAddr(remoteAddr)
}

func splitTrustedProxies(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := parts[:0]
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
