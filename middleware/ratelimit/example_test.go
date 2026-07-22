package ratelimit_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"time"

	"github.com/aatuh/api-toolkit/v4/middleware/ratelimit"
)

type exampleLimiter struct{}

func (exampleLimiter) Allow(context.Context, string) (bool, time.Duration, error) {
	return true, 0, nil
}

func ExampleLimiter() {
	var limiter ratelimit.Limiter = exampleLimiter{}
	_ = limiter
}

type exampleDecisionLimiter struct{}

func (exampleDecisionLimiter) Allow(context.Context, string) (ratelimit.Decision, error) {
	return ratelimit.Decision{
		Allowed:   true,
		Limit:     10,
		Remaining: 9,
		Reset:     time.Unix(1_700_000_000, 0).UTC(),
	}, nil
}

func ExampleDecisionLimiter() {
	var limiter ratelimit.DecisionLimiter = exampleDecisionLimiter{}
	decision, _ := limiter.Allow(context.Background(), "customer-42")
	options := ratelimit.Options{
		DecisionLimiter:  limiter,
		CleanupBatchSize: 64,
		HeaderConfig:     ratelimit.DefaultHeaderConfig(),
	}

	fmt.Println(decision.Limit, decision.Remaining, decision.Reset.UTC().Unix())
	fmt.Println(options.CleanupBatchSize)

	// Output:
	// 10 9 1700000000
	// 64
}

func ExampleSetRateLimitHeaders() {
	recorder := httptest.NewRecorder()
	ratelimit.SetRateLimitHeaders(recorder, ratelimit.Quota{
		Limit:     10,
		Remaining: 9,
	}, ratelimit.DefaultHeaderConfig())

	fmt.Println(recorder.Header().Get("RateLimit-Limit"))
	fmt.Println(recorder.Header().Get("RateLimit-Remaining"))

	// Output:
	// 10
	// 9
}
