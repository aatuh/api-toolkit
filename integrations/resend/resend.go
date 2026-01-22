package resend

import (
	"net/http"

	"github.com/aatuh/api-toolkit/adapters/resend"
	"github.com/aatuh/api-toolkit/config"
	"github.com/aatuh/api-toolkit/ports"
)

// Client aliases the Resend client type.
type Client = resend.Client

// Option aliases the Resend client option type.
type Option = resend.Option

// Config aliases the Resend configuration struct.
type Config = resend.Config

var (
	// WithBaseURL overrides the Resend API base URL.
	WithBaseURL = resend.WithBaseURL
	// WithHTTPClient overrides the HTTP client used for requests.
	WithHTTPClient = resend.WithHTTPClient
)

// New constructs a Resend client.
func New(apiKey string, opts ...Option) *Client {
	return resend.New(apiKey, opts...)
}

// LoadConfig reads Resend config from environment.
func LoadConfig(loader *config.Loader) Config {
	return resend.LoadConfig(loader)
}

// HealthChecker returns a health checker for Resend.
func HealthChecker(cfg Config, client *http.Client) ports.HealthChecker {
	return resend.HealthChecker(cfg, client)
}
