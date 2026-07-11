package oidc

import (
	"context"
	"net/http"

	"github.com/aatuh/api-toolkit/contrib/v4/config"
	"github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/oidc"
	"github.com/aatuh/api-toolkit/v4/endpoints/health"
	"github.com/aatuh/api-toolkit/v4/ports"
)

// Config aliases the OIDC middleware configuration.
type Config = oidc.Config

// Middleware aliases the OIDC middleware.
type Middleware = oidc.Middleware

// Subject aliases the OIDC subject payload.
type Subject = oidc.Subject

// NewMiddleware creates an OIDC middleware.
func NewMiddleware(ctx context.Context, cfg Config, log ports.Logger) (*Middleware, error) {
	return oidc.NewMiddleware(ctx, cfg, log)
}

// LoadConfig reads OIDC configuration from environment.
func LoadConfig(loader *config.Loader) Config {
	return oidc.LoadConfig(loader)
}

// WithSubject stores the authenticated subject in context.
func WithSubject(ctx context.Context, subj Subject) context.Context {
	return oidc.WithSubject(ctx, subj)
}

// SubjectFromContext retrieves the subject from context.
func SubjectFromContext(ctx context.Context) (Subject, bool) {
	return oidc.SubjectFromContext(ctx)
}

// HealthChecker returns a health checker for OIDC JWKS.
func HealthChecker(cfg Config, client *http.Client) health.Checker {
	return oidc.HealthChecker(cfg, client)
}
