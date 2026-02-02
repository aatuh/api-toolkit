package clerk

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v2/endpoints/health"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// HealthChecker returns a Clerk JWKS health checker or nil when disabled.
func HealthChecker(cfg Config, client *http.Client) ports.HealthChecker {
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
		"clerk",
		timeout,
		func(ctx context.Context) (ports.HealthStatus, string, interface{}) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.JWKSURL, nil)
			if err != nil {
				return ports.HealthStatusDegraded, fmt.Sprintf("clerk jwks request failed: %v", err), nil
			}
			resp, err := client.Do(req)
			if err != nil {
				return ports.HealthStatusDegraded, fmt.Sprintf("clerk jwks request failed: %v", err), nil
			}
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != http.StatusOK {
				return ports.HealthStatusDegraded, fmt.Sprintf("unexpected status %d from clerk jwks", resp.StatusCode), map[string]interface{}{
					"status_code": resp.StatusCode,
					"url":         cfg.JWKSURL,
				}
			}

			return ports.HealthStatusHealthy, "clerk jwks reachable", map[string]interface{}{
				"status_code": resp.StatusCode,
				"url":         cfg.JWKSURL,
			}
		},
	)
}
