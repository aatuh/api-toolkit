package bootstrap

import (
	"context"
	"fmt"
	"strings"
)

// MiddlewareStage names a middleware responsibility in a bootstrap profile.
type MiddlewareStage string

const (
	MiddlewareRequestID      MiddlewareStage = "request_id"
	MiddlewareRecovery       MiddlewareStage = "recovery"
	MiddlewareTracing        MiddlewareStage = "tracing"
	MiddlewareCORS           MiddlewareStage = "cors"
	MiddlewareSecureHeaders  MiddlewareStage = "secure_headers"
	MiddlewareRateLimit      MiddlewareStage = "rate_limit"
	MiddlewareBodyLimit      MiddlewareStage = "body_limit"
	MiddlewareQueryLimit     MiddlewareStage = "query_limit"
	MiddlewareJSON           MiddlewareStage = "json"
	MiddlewareTimeout        MiddlewareStage = "timeout"
	MiddlewareRequestLogging MiddlewareStage = "request_logging"
	MiddlewareMetrics        MiddlewareStage = "metrics"
	MiddlewareAuth           MiddlewareStage = "auth"
	MiddlewareTenant         MiddlewareStage = "tenant"
	MiddlewareIdempotency    MiddlewareStage = "idempotency"
)

// StrictAPIMiddlewareOrder returns the required production API middleware
// sequence. Extra stages such as CORS may appear between these stages.
func StrictAPIMiddlewareOrder() []MiddlewareStage {
	return []MiddlewareStage{
		MiddlewareRequestID,
		MiddlewareRecovery,
		MiddlewareTracing,
		MiddlewareSecureHeaders,
		MiddlewareRateLimit,
		MiddlewareBodyLimit,
		MiddlewareQueryLimit,
		MiddlewareJSON,
		MiddlewareTimeout,
		MiddlewareRequestLogging,
		MiddlewareMetrics,
	}
}

// DevMiddlewareOrder returns the development profile middleware sequence.
func DevMiddlewareOrder() []MiddlewareStage {
	return []MiddlewareStage{
		MiddlewareRequestID,
		MiddlewareRecovery,
		MiddlewareTracing,
		MiddlewareCORS,
		MiddlewareSecureHeaders,
		MiddlewareBodyLimit,
		MiddlewareQueryLimit,
		MiddlewareJSON,
		MiddlewareTimeout,
		MiddlewareRequestLogging,
		MiddlewareMetrics,
	}
}

// ValidateMiddlewareOrder verifies that the required stages appear in the given
// order. The check allows extra stages between required stages so applications
// can add route- or deployment-specific middleware without losing the baseline
// production ordering contract.
func ValidateMiddlewareOrder(order []MiddlewareStage, required ...MiddlewareStage) error {
	if len(required) == 0 {
		return nil
	}
	index := 0
	for _, want := range required {
		want = cleanMiddlewareStage(want)
		if want == "" {
			continue
		}
		found := false
		for index < len(order) {
			got := cleanMiddlewareStage(order[index])
			index++
			if got == want {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("missing middleware stage %q in required order %s; got %s", want, middlewareStageList(required), middlewareStageList(order))
		}
	}
	return nil
}

// RequireMiddlewareOrder builds a startup check for applications that compose
// custom routers or route-level middleware outside the default bootstrap profile.
func RequireMiddlewareOrder(name string, order []MiddlewareStage, required ...MiddlewareStage) StartupCheck {
	return StartupCheck{
		Name: strings.TrimSpace(name),
		Check: func(context.Context) error {
			return ValidateMiddlewareOrder(order, required...)
		},
	}
}

func cleanMiddlewareStage(stage MiddlewareStage) MiddlewareStage {
	return MiddlewareStage(strings.TrimSpace(string(stage)))
}

func middlewareStageList(stages []MiddlewareStage) string {
	values := make([]string, 0, len(stages))
	for _, stage := range stages {
		stage = cleanMiddlewareStage(stage)
		if stage != "" {
			values = append(values, string(stage))
		}
	}
	return strings.Join(values, ",")
}
