package health_test

import (
	"context"
	"fmt"
	"time"

	"github.com/aatuh/api-toolkit/v4/endpoints/health"
)

func ExampleNewBasicChecker() {
	checker := health.NewBasicChecker()
	result := checker.Check(context.Background())

	fmt.Println(result.Status)

	// Output:
	// healthy
}

func ExampleNewManager() {
	config := health.DefaultConfig()
	var clock health.Clock = exampleManagerClock{}
	config.Clock = clock
	if err := config.Validate(); err != nil {
		fmt.Println(err)
		return
	}
	_ = config.Clock.Now()

	manager, err := health.NewManager(config)
	if err != nil {
		fmt.Println(err)
		return
	}
	if err := manager.RegisterCheckerChecked(health.NewBasicChecker()); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(manager.GetReadiness(context.Background()).Status)

	// Output:
	// healthy
}

type exampleManagerClock struct{}

func (exampleManagerClock) Now() time.Time {
	return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
}
