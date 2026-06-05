package ratelimit_test

import (
	"fmt"
	"net/http/httptest"

	"github.com/aatuh/api-toolkit/v3/middleware/ratelimit"
)

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
