package health_test

import (
	"context"
	"fmt"
	"time"

	"github.com/aatuh/api-toolkit/v4/endpoints/health"
)

type exampleManagerClock struct {
	now time.Time
}

func (c exampleManagerClock) Now() time.Time { return c.now }

func ExampleNewBasicChecker() {
	checker := health.NewBasicChecker()
	result := checker.Check(context.Background())

	fmt.Println(result.Status)

	// Output:
	// healthy
}

func ExampleNewManager() {
	config := health.DefaultConfig()
	config.Clock = exampleManagerClock{now: time.Date(2026, time.August, 22, 12, 0, 0, 0, time.UTC)}
	if err := config.Validate(); err != nil {
		panic(err)
	}

	manager, err := health.NewManager(config)
	if err != nil {
		panic(err)
	}
	if err := manager.RegisterCheckerChecked(health.NewBasicChecker()); err != nil {
		panic(err)
	}

	fmt.Println(manager.GetReadiness(context.Background()).Status)

	// Output:
	// healthy
}
