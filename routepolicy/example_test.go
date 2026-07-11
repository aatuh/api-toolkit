package routepolicy_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v4/routepolicy"
	"github.com/aatuh/api-toolkit/v4/specs"
)

func ExampleApplyMetadata() {
	operation := routepolicy.ApplyMetadata(
		specs.Operation{},
		routepolicy.WithTenantRequired("path"),
		routepolicy.WithIdempotencyRequired(),
	)

	labels := routepolicy.ObservabilityLabelsFromOperation(operation).Map()
	fmt.Println(labels["tenant"])
	fmt.Println(labels["idempotency"])

	// Output:
	// required
	// required
}
