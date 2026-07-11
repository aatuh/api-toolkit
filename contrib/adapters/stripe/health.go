package stripe

import (
	"context"
	"fmt"

	compatbilling "github.com/aatuh/api-toolkit/v3/compat/billing"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
)

// HealthChecker returns a Stripe payment provider checker or nil when provider is nil.
func HealthChecker(provider compatbilling.PaymentProvider) health.Checker {
	if provider == nil {
		return nil
	}
	return health.NewCustomChecker(
		"stripe",
		func(ctx context.Context) (health.Status, string, interface{}) {
			prices, err := provider.ListPrices(ctx)
			if err != nil {
				return health.StatusDegraded, fmt.Sprintf("payment provider check failed: %v", err), nil
			}
			return health.StatusHealthy, "payment provider healthy", map[string]interface{}{
				"price_count": len(prices),
			}
		},
	)
}
