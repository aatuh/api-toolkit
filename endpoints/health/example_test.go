package health_test

import (
	"context"
	"fmt"

	"github.com/aatuh/api-toolkit/v3/endpoints/health"
)

func ExampleNewBasicChecker() {
	checker := health.NewBasicChecker()
	result := checker.Check(context.Background())

	fmt.Println(result.Status)

	// Output:
	// healthy
}
