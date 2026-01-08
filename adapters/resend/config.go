package resend

import "github.com/aatuh/api-toolkit/config"

// Config describes Resend email integration.
type Config struct {
	Enabled   bool
	APIKey    string
	From      string
	ContactTo []string
	BaseURL   string
}

// LoadConfig reads Resend config from environment.
func LoadConfig(loader *config.Loader) Config {
	if loader == nil {
		loader = config.NewLoader()
	}
	return Config{
		Enabled:   loader.Bool("RESEND_ENABLED", false),
		APIKey:    loader.String("RESEND_API_KEY", ""),
		From:      loader.String("RESEND_FROM", ""),
		ContactTo: loader.CSV("RESEND_CONTACT_TO"),
		BaseURL:   loader.String("RESEND_BASE_URL", ""),
	}
}
