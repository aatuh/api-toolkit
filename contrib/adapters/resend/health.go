package resend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v3/endpoints/health"
)

// HealthChecker returns a Resend health checker or nil when disabled.
func HealthChecker(cfg Config, client *http.Client) health.Checker {
	if !cfg.Enabled || cfg.APIKey == "" {
		return nil
	}
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return health.NewCustomCheckerWithTimeout(
		"resend",
		5*time.Second,
		func(ctx context.Context) (health.Status, string, interface{}) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/domains", nil)
			if err != nil {
				return health.StatusDegraded, fmt.Sprintf("resend request failed: %v", err), nil
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

			resp, err := client.Do(req)
			if err != nil {
				return health.StatusDegraded, fmt.Sprintf("resend request failed: %v", err), nil
			}
			defer func() {
				_ = resp.Body.Close()
			}()

			type resendError struct {
				StatusCode int    `json:"statusCode"`
				Message    string `json:"message"`
				Name       string `json:"name"`
			}

			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			if resp.StatusCode == http.StatusOK {
				return health.StatusHealthy, "resend reachable", map[string]interface{}{
					"status_code": resp.StatusCode,
				}
			}

			var payload resendError
			_ = json.Unmarshal(body, &payload)
			if resp.StatusCode == http.StatusUnauthorized && payload.Name == "restricted_api_key" {
				return health.StatusHealthy, "resend key is send-only; domains listing restricted", map[string]interface{}{
					"status_code": resp.StatusCode,
					"name":        payload.Name,
				}
			}

			message := fmt.Sprintf("resend check failed with status %d", resp.StatusCode)
			if payload.Message != "" {
				message = fmt.Sprintf("resend check failed: %s", payload.Message)
			}
			return health.StatusDegraded, message, map[string]interface{}{
				"status_code": resp.StatusCode,
				"name":        payload.Name,
			}
		},
	)
}
