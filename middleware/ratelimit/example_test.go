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
