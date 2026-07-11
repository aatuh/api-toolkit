package clerk

import (
	"context"
	"net/http"

	"github.com/aatuh/api-toolkit/contrib/v4/config"
	"github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/clerk"
	"github.com/aatuh/api-toolkit/v4/endpoints/health"
	"github.com/aatuh/api-toolkit/v4/ports"
)

// Config aliases the Clerk middleware configuration.
type Config = clerk.Config

// Middleware aliases the Clerk JWT middleware.
type Middleware = clerk.Middleware

// Subject aliases the Clerk subject payload.
type Subject = clerk.Subject

// NewMiddleware creates a Clerk JWT middleware.
func NewMiddleware(ctx context.Context, cfg Config, log ports.Logger) (*Middleware, error) {
	return clerk.NewMiddleware(ctx, cfg, log)
}

// LoadConfig reads Clerk configuration from environment.
func LoadConfig(loader *config.Loader) Config {
	return clerk.LoadConfig(loader)
}

// WithSubject stores the authenticated subject in context.
func WithSubject(ctx context.Context, subj Subject) context.Context {
	return clerk.WithSubject(ctx, subj)
}

// SubjectFromContext retrieves the subject from context.
func SubjectFromContext(ctx context.Context) (Subject, bool) {
	return clerk.SubjectFromContext(ctx)
}

// HealthChecker returns a health checker for Clerk.
func HealthChecker(cfg Config, client *http.Client) health.Checker {
	return clerk.HealthChecker(cfg, client)
}
