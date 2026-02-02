package stripe

import (
	"context"
	"fmt"

	"github.com/aatuh/api-toolkit/endpoints/health"
	"github.com/aatuh/api-toolkit/ports"
)

// HealthChecker returns a Stripe payment provider checker or nil when provider is nil.
func HealthChecker(provider ports.PaymentProvider) ports.HealthChecker {
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
