package devheaders

import (
	"errors"
	"net/http"
	"strings"

	"github.com/aatuh/api-toolkit/httpx"
	jwtauth "github.com/aatuh/api-toolkit/middleware/auth/jwt"
	"github.com/aatuh/api-toolkit/ports"
)

// Config controls dev header-based auth.
type Config struct {
	Enabled         bool
	UserIDHeader    string
	EmailHeader     string
	FirstNameHeader string
	LastNameHeader  string
	DefaultLanguage string
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
	if cfg.Enabled && cfg.UserIDHeader == "" {
		return nil, errors.New("user id header is required when dev auth is enabled")
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
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{
				Title:  http.StatusText(http.StatusUnauthorized),
				Detail: "missing development auth headers",
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
