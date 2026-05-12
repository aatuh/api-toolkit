package routepolicy

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/v2/middleware/deprecation"
	"github.com/aatuh/api-toolkit/v2/negotiation"
	"github.com/aatuh/api-toolkit/v2/specs"
)

// ErrUnsupportedPolicy reports metadata the configured policy cannot enforce.
var ErrUnsupportedPolicy = errors.New("routepolicy: unsupported policy")

// AuthPolicyFunc builds auth middleware from operation security and scopes.
type AuthPolicyFunc func(specs.Operation) (func(http.Handler) http.Handler, error)

// IdempotencyPolicyFunc builds idempotency middleware from operation metadata.
type IdempotencyPolicyFunc func(specs.Operation) (func(http.Handler) http.Handler, error)

// RateLimitPolicyFunc builds rate-limit middleware from operation metadata.
type RateLimitPolicyFunc func(specs.Operation) (func(http.Handler) http.Handler, error)

// ProblemCatalogPolicy validates or enriches operation problem-response policy.
type ProblemCatalogPolicy func(specs.Operation) error

// Config configures policy derivation.
type Config struct {
	EnableDeprecation   bool
	EnableNegotiation   bool
	EmitPolicyExtension bool
	Auth                AuthPolicyFunc
	Idempotency         IdempotencyPolicyFunc
	RateLimit           RateLimitPolicyFunc
	ProblemCatalog      ProblemCatalogPolicy
}

// Policy derives middleware and operation metadata from specs.Operation.
type Policy struct {
	config Config
}

// New constructs a route policy.
func New(config Config) *Policy {
	return &Policy{config: config}
}

// Apply derives middleware and optional deterministic policy extensions.
func (policy *Policy) Apply(operation specs.Operation) (specs.Operation, []func(http.Handler) http.Handler, error) {
	if policy == nil {
		return operation, nil, nil
	}
	config := policy.config
	var middleware []func(http.Handler) http.Handler
	applied := map[string]any{}
	if config.EnableDeprecation && (operation.Deprecated || strings.TrimSpace(operation.Sunset) != "") {
		mw, err := deprecationMiddleware(operation)
		if err != nil {
			return operation, nil, err
		}
		middleware = append(middleware, mw)
		applied["deprecation"] = true
	}
	if config.EnableNegotiation {
		mw, ok, err := negotiationMiddleware(operation)
		if err != nil {
			return operation, nil, err
		}
		if ok {
			middleware = append(middleware, mw)
			applied["negotiation"] = true
		}
	}
	if config.Auth != nil && (len(operation.Security) > 0 || len(operation.Scopes) > 0) {
		mw, err := config.Auth(operation)
		if err != nil {
			return operation, nil, err
		}
		if mw != nil {
			middleware = append(middleware, mw)
			applied["auth"] = true
		}
	}
	if config.Idempotency != nil && operation.Extensions != nil {
		if _, ok := operation.Extensions[ExtensionIdempotencyKey]; ok {
			mw, err := config.Idempotency(operation)
			if err != nil {
				return operation, nil, err
			}
			if mw != nil {
				middleware = append(middleware, mw)
				applied["idempotency"] = true
			}
		}
	}
	if config.RateLimit != nil && operation.Extensions != nil {
		if _, ok := operation.Extensions[ExtensionRateLimit]; ok {
			mw, err := config.RateLimit(operation)
			if err != nil {
				return operation, nil, err
			}
			if mw != nil {
				middleware = append(middleware, mw)
				applied["rate_limit"] = true
			}
		}
	}
	if config.ProblemCatalog != nil {
		if err := config.ProblemCatalog(operation); err != nil {
			return operation, nil, err
		}
		applied["problem_catalog"] = true
	}
	if config.EmitPolicyExtension && len(applied) > 0 {
		operation.Extensions = copyExtensions(operation.Extensions)
		operation.Extensions["x-api-toolkit-policy"] = stablePolicyExtension(applied)
	}
	return operation, middleware, nil
}

func deprecationMiddleware(operation specs.Operation) (func(http.Handler) http.Handler, error) {
	config := deprecation.Config{}
	if operation.Deprecated {
		config.DeprecatedAt = time.Unix(0, 0).UTC()
	}
	if strings.TrimSpace(operation.Sunset) != "" {
		sunset, err := http.ParseTime(strings.TrimSpace(operation.Sunset))
		if err != nil {
			return nil, err
		}
		config.SunsetAt = sunset
	}
	mw, err := deprecation.New(config)
	if err != nil {
		return nil, err
	}
	return mw.Handler, nil
}

func negotiationMiddleware(operation specs.Operation) (func(http.Handler) http.Handler, bool, error) {
	accept := responseMediaTypes(operation)
	contentTypes := requestMediaTypes(operation)
	if len(accept) == 0 && len(contentTypes) == 0 {
		return nil, false, nil
	}
	mw, err := negotiation.New(negotiation.Config{Accept: accept, ContentTypes: contentTypes})
	if err != nil {
		return nil, false, err
	}
	return mw.Middleware(), true, nil
}

func responseMediaTypes(operation specs.Operation) []negotiation.MediaType {
	set := map[string]struct{}{}
	for _, response := range operation.Responses {
		for mediaType := range response.Content {
			if strings.TrimSpace(mediaType) != "" {
				set[strings.TrimSpace(mediaType)] = struct{}{}
			}
		}
	}
	return sortedMediaTypes(set)
}

func requestMediaTypes(operation specs.Operation) []negotiation.MediaType {
	set := map[string]struct{}{}
	if operation.RequestBody == nil {
		return nil
	}
	for mediaType := range operation.RequestBody.Content {
		if strings.TrimSpace(mediaType) != "" {
			set[strings.TrimSpace(mediaType)] = struct{}{}
		}
	}
	return sortedMediaTypes(set)
}

func sortedMediaTypes(set map[string]struct{}) []negotiation.MediaType {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	out := make([]negotiation.MediaType, 0, len(values))
	for _, value := range values {
		out = append(out, negotiation.MediaType(value))
	}
	return out
}

func copyExtensions(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stablePolicyExtension(applied map[string]any) map[string]any {
	keys := make([]string, 0, len(applied))
	for key := range applied {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		out[key] = applied[key]
	}
	return out
}
