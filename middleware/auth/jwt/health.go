package jwt

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v3/endpoints/health"
)

// HealthChecker returns a JWKS health checker or nil when disabled.
func HealthChecker(cfg Config, client *http.Client) health.Checker {
	if !cfg.Enabled || cfg.JWKSURL == "" {
		return nil
	}
	timeout := cfg.JWKSRefreshTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if client == nil {
		client = http.DefaultClient
	}
	return health.NewCustomCheckerWithTimeout(
		"jwt",
		timeout,
		func(ctx context.Context) (health.Status, string, interface{}) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURL, nil)
			if err != nil {
				return health.StatusDegraded, fmt.Sprintf("jwks request failed: %v", err), nil
			}
			resp, err := client.Do(req)
			if err != nil {
				return health.StatusDegraded, fmt.Sprintf("jwks request failed: %v", err), nil
			}
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != http.StatusOK {
				return health.StatusDegraded, fmt.Sprintf("unexpected status %d from jwks", resp.StatusCode), map[string]interface{}{
					"status_code": resp.StatusCode,
					"url":         cfg.JWKSURL,
				}
			}

			return health.StatusHealthy, "jwks reachable", map[string]interface{}{
				"status_code": resp.StatusCode,
				"url":         cfg.JWKSURL,
			}
		},
	)
}
