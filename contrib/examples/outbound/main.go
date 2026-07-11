// Command outbound shows a guarded outbound HTTP client with retries and breakers.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/httpclient"
)

func main() {
	guard, err := httpclient.NewSSRFTransport(httpclient.SSRFOptions{
		AllowedHosts: []string{"api.example.com"},
		AllowedPorts: []int{443},
	})
	if err != nil {
		log.Fatalf("init ssrf guard: %v", err)
	}
	bulkhead, err := httpclient.NewSemaphoreBulkhead(50)
	if err != nil {
		log.Fatalf("init bulkhead: %v", err)
	}
	breaker := httpclient.NewCircuitBreaker(httpclient.CircuitBreakerOptions{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		OpenTimeout:         30 * time.Second,
		HalfOpenMaxInFlight: 1,
	})

	client := httpclient.New(httpclient.Options{
		Transport:     guard,
		CheckRedirect: guard.CheckRedirect,
		Retry: httpclient.RetryOptions{
			MaxRetries:     2,
			MaxElapsedTime: 2 * time.Second,
			UseRetryAfter:  true,
		},
		Breaker:  breaker,
		Bulkhead: bulkhead,
	})

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.example.com/health", nil)
	if err != nil {
		log.Fatalf("build request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("outbound request failed: %v", err)
	}
	if resp != nil && resp.Body != nil {
		if err := resp.Body.Close(); err != nil && !errors.Is(err, http.ErrBodyReadAfterClose) {
			log.Printf("close response body: %v", err)
		}
	}
}
