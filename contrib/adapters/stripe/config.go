package stripe

import "github.com/aatuh/api-toolkit/contrib/v3/config"

// Config describes Stripe credentials and URLs.
type Config struct {
	Enabled           bool
	SecretKey         string
	WebhookSecret     string
	WebhookSkipVerify bool
	WebhookDevMode    bool
	FrontendBaseURL   string
}

// LoadConfig reads Stripe config from environment.
func LoadConfig(loader *config.Loader) Config {
	if loader == nil {
		loader = config.NewLoader()
	}
	return Config{
		Enabled:           loader.Bool("STRIPE_ENABLED", false),
		SecretKey:         loader.String("STRIPE_SECRET_KEY", ""),
		WebhookSecret:     loader.String("STRIPE_WEBHOOK_SECRET", ""),
		WebhookSkipVerify: loader.Bool("STRIPE_WEBHOOK_SKIP_VERIFY", false),
		WebhookDevMode:    loader.Bool("STRIPE_WEBHOOK_DEV_MODE", false),
		FrontendBaseURL:   loader.String("FRONTEND_BASE_URL", "http://localhost:3000"),
	}
}
