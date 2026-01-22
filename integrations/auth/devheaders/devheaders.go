package devheaders

import (
	"github.com/aatuh/api-toolkit/config"
	"github.com/aatuh/api-toolkit/middleware/auth/devheaders"
	"github.com/aatuh/api-toolkit/ports"
)

// Config aliases the dev headers middleware configuration.
type Config = devheaders.Config

// Middleware aliases the dev headers middleware.
type Middleware = devheaders.Middleware

// New constructs the dev headers middleware.
func New(cfg Config, log ports.Logger) *Middleware {
	return devheaders.New(cfg, log)
}

// LoadConfig reads dev headers configuration from environment.
func LoadConfig(loader *config.Loader) Config {
	return devheaders.LoadConfig(loader)
}
