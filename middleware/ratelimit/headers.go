package ratelimit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aatuh/api-toolkit/v3/httpx"
)

// Quota describes a rate-limit quota snapshot for response headers.
type Quota struct {
	Limit      int
	Remaining  int
	Reset      time.Time
	RetryAfter time.Duration
}

// Decision describes an allow/deny decision with optional quota metadata.
type Decision struct {
	Allowed    bool
	Quota      Quota
	RetryAfter time.Duration
}

// HeaderConfig configures standard rate-limit response headers.
type HeaderConfig struct {
	Enabled          bool
	LimitHeader      string
	RemainingHeader  string
	ResetHeader      string
	RetryAfterHeader string
}

// DefaultHeaderConfig returns enabled RFC-compatible quota header names.
func DefaultHeaderConfig() HeaderConfig {
	return HeaderConfig{
		Enabled:          true,
		LimitHeader:      "RateLimit-Limit",
		RemainingHeader:  "RateLimit-Remaining",
		ResetHeader:      "RateLimit-Reset",
		RetryAfterHeader: "Retry-After",
	}
}

// SetRateLimitHeaders writes standard quota headers when config is enabled.
func SetRateLimitHeaders(w http.ResponseWriter, quota Quota, config HeaderConfig) {
	if w == nil || !config.Enabled {
		return
	}
	config = normalizeHeaderConfig(config)
	if quota.Limit > 0 {
		w.Header().Set(config.LimitHeader, strconv.Itoa(quota.Limit))
	}
	if quota.Remaining >= 0 {
		w.Header().Set(config.RemainingHeader, strconv.Itoa(quota.Remaining))
	}
	if !quota.Reset.IsZero() {
		w.Header().Set(config.ResetHeader, strconv.FormatInt(quota.Reset.UTC().Unix(), 10))
	}
	if quota.RetryAfter > 0 {
		w.Header().Set(config.RetryAfterHeader, itoa(retryAfterSeconds(quota.RetryAfter)))
	}
}

// WriteRateLimited writes a 429 Problem Details response with quota headers.
func WriteRateLimited(w http.ResponseWriter, decision Decision, config HeaderConfig) {
	quota := QuotaFromDecision(decision)
	SetRateLimitHeaders(w, quota, config)
	if quota.RetryAfter > 0 {
		w.Header().Set(normalizeHeaderConfig(config).RetryAfterHeader, itoa(retryAfterSeconds(quota.RetryAfter)))
	}
	httpx.WriteProblem(w, http.StatusTooManyRequests, httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeRateLimited),
		Title:  http.StatusText(http.StatusTooManyRequests),
		Detail: "rate limit exceeded",
	})
}

// QuotaFromDecision extracts quota metadata from a decision.
func QuotaFromDecision(decision Decision) Quota {
	quota := decision.Quota
	if quota.RetryAfter <= 0 {
		quota.RetryAfter = decision.RetryAfter
	}
	return quota
}

func normalizeHeaderConfig(config HeaderConfig) HeaderConfig {
	defaults := DefaultHeaderConfig()
	if config.LimitHeader == "" {
		config.LimitHeader = defaults.LimitHeader
	}
	if config.RemainingHeader == "" {
		config.RemainingHeader = defaults.RemainingHeader
	}
	if config.ResetHeader == "" {
		config.ResetHeader = defaults.ResetHeader
	}
	if config.RetryAfterHeader == "" {
		config.RetryAfterHeader = defaults.RetryAfterHeader
	}
	return config
}
