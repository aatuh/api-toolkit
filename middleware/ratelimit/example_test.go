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
		Limit:     100,
		Remaining: 99,
		Reset:     time.Unix(1_000, 0).UTC(),
	}, nil
}

func ExampleDecisionLimiter() {
	var limiter ratelimit.DecisionLimiter = exampleDecisionLimiter{}
	decision, err := limiter.Allow(context.Background(), "client-1")
	if err != nil {
		return
	}
	fmt.Println(decision.Limit)
	fmt.Println(decision.Remaining)
	fmt.Println(decision.Reset.Unix())

	// Output:
	// 100
	// 99
	// 1000
}

func ExampleOptions_DecisionLimiter() {
	middleware, err := ratelimit.New(ratelimit.Options{
		DecisionLimiter: exampleDecisionLimiter{},
		HeaderConfig:    ratelimit.DefaultHeaderConfig(),
	})
	if err != nil {
		return
	}
	fmt.Println(middleware != nil)

	// Output:
	// true
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
