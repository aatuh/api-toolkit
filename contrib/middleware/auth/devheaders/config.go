package devheaders

import "github.com/aatuh/api-toolkit-contrib/config"

// LoadConfig reads dev-auth header config from environment.
func LoadConfig(loader *config.Loader) Config {
	if loader == nil {
		loader = config.NewLoader()
	}
	return Config{
		Enabled:         loader.Bool("DEV_AUTH_FALLBACK_ENABLED", false),
		UserIDHeader:    loader.String("DEV_AUTH_USER_HEADER", "X-Debug-User"),
		EmailHeader:     loader.String("DEV_AUTH_EMAIL_HEADER", "X-Debug-Email"),
		FirstNameHeader: loader.String("DEV_AUTH_FIRST_NAME_HEADER", "X-Debug-First-Name"),
		LastNameHeader:  loader.String("DEV_AUTH_LAST_NAME_HEADER", "X-Debug-Last-Name"),
		DefaultLanguage: loader.String("DEV_AUTH_DEFAULT_LANGUAGE", "fi"),
	}
}
