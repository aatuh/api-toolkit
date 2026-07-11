package stripe

import (
	"context"
	"net/http"
	"net/netip"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/stripe"
	"github.com/aatuh/api-toolkit/contrib/v3/config"
	compatbilling "github.com/aatuh/api-toolkit/v3/compat/billing"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
	"github.com/aatuh/api-toolkit/v3/httpx/identity"
)

// Provider aliases the Stripe payment provider type.
type Provider = stripe.Provider

// Option aliases the Stripe provider option type.
type Option = stripe.Option

// Config aliases the Stripe configuration struct.
type Config = stripe.Config

var (
	// WithSkipVerify disables webhook signature verification in dev mode.
	WithSkipVerify = stripe.WithSkipVerify
	// WithDevMode enables development-only webhook behavior.
	WithDevMode = stripe.WithDevMode
	// AllowInsecureWebhook marks webhook contexts safe for dev-only skipping.
	AllowInsecureWebhook = stripe.AllowInsecureWebhook
	// AllowInsecureWebhookFromRequest marks webhook contexts safe based on request IP.
	AllowInsecureWebhookFromRequest = stripe.AllowInsecureWebhookFromRequest
)

// New returns a Stripe-backed payment provider.
func New(secretKey, webhookSecret string, opts ...Option) *Provider {
	return stripe.New(secretKey, webhookSecret, opts...)
}

// LoadConfig reads Stripe config from environment.
func LoadConfig(loader *config.Loader) Config {
	return stripe.LoadConfig(loader)
}

// HealthChecker reports readiness of the Stripe provider.
func HealthChecker(provider compatbilling.PaymentProvider) health.Checker {
	return stripe.HealthChecker(provider)
}

// AllowInsecureWebhookContext marks a webhook request as safe for dev-only skipping.
func AllowInsecureWebhookContext(ctx context.Context, ip netip.Addr) context.Context {
	return stripe.AllowInsecureWebhook(ctx, ip)
}

// AllowInsecureWebhookContextFromRequest derives source IP via resolver and marks context.
func AllowInsecureWebhookContextFromRequest(ctx context.Context, r *http.Request, resolver identity.Resolver) context.Context {
	return stripe.AllowInsecureWebhookFromRequest(ctx, r, resolver)
}
