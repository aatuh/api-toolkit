package stripe

import (
	"context"
	"fmt"

	compatbilling "github.com/aatuh/api-toolkit/v2/compat/billing"
	"github.com/aatuh/api-toolkit/v2/endpoints/health"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// HealthChecker returns a Stripe payment provider checker or nil when provider is nil.
func HealthChecker(provider compatbilling.PaymentProvider) ports.HealthChecker {
	if provider == nil {
		return nil
	}
	return health.NewCustomChecker(
		"stripe",
		func(ctx context.Context) (ports.HealthStatus, string, interface{}) {
			prices, err := provider.ListPrices(ctx)
			if err != nil {
				return ports.HealthStatusDegraded, fmt.Sprintf("payment provider check failed: %v", err), nil
			}
			return ports.HealthStatusHealthy, "payment provider healthy", map[string]interface{}{
				"price_count": len(prices),
			}
		},
	)
}
