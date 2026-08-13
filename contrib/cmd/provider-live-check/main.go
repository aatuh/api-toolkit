// Command provider-live-check records sanitized, non-mutating sandbox reachability
// evidence for supported provider adapters. It deliberately never emits provider
// endpoints, credentials, resource identifiers, or response bodies.
package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/resend"
	stripeadapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/stripe"
	clerkadapter "github.com/aatuh/api-toolkit/contrib/v4/middleware/auth/clerk"
	"github.com/aatuh/api-toolkit/v4/endpoints/health"
)

const timeout = 10 * time.Second

type providerStatus struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
}

type evidence struct {
	CheckedAt string           `json:"checked_at"`
	Status    string           `json:"status"`
	Providers []providerStatus `json:"providers"`
}

func main() {
	statuses, attempted := run(context.Background())
	result := evidence{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Status:    overallStatus(statuses, attempted),
		Providers: statuses,
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
	if result.Status == "failed" {
		os.Exit(1)
	}
}

func run(parent context.Context) ([]providerStatus, bool) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	statuses := make([]providerStatus, 0, 3)
	attempted := false
	if secret := strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")); secret != "" {
		attempted = true
		_, err := stripeadapter.New(secret, "").ListPrices(ctx)
		statuses = append(statuses, statusFor("stripe", err == nil))
	} else {
		statuses = append(statuses, providerStatus{Provider: "stripe", Status: "skipped_no_credentials"})
	}

	if key := strings.TrimSpace(os.Getenv("RESEND_API_KEY")); key != "" {
		attempted = true
		checker := resend.HealthChecker(resend.Config{Enabled: true, APIKey: key}, nil)
		statuses = append(statuses, statusFor("resend", checker != nil && checker.Check(ctx).Status == health.StatusHealthy))
	} else {
		statuses = append(statuses, providerStatus{Provider: "resend", Status: "skipped_no_credentials"})
	}

	if jwksURL := strings.TrimSpace(os.Getenv("CLERK_JWKS_URL")); jwksURL != "" {
		attempted = true
		checker := clerkadapter.HealthChecker(clerkadapter.Config{Enabled: true, JWKSURL: jwksURL}, nil)
		statuses = append(statuses, statusFor("clerk", checker != nil && checker.Check(ctx).Status == health.StatusHealthy))
	} else {
		statuses = append(statuses, providerStatus{Provider: "clerk", Status: "skipped_no_credentials"})
	}
	return statuses, attempted
}

func statusFor(provider string, passed bool) providerStatus {
	if passed {
		return providerStatus{Provider: provider, Status: "passed"}
	}
	return providerStatus{Provider: provider, Status: "failed"}
}

func overallStatus(statuses []providerStatus, attempted bool) string {
	if !attempted {
		return "skipped_no_credentials"
	}
	for _, status := range statuses {
		if status.Status == "failed" {
			return "failed"
		}
	}
	return "passed"
}
