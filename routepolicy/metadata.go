package routepolicy

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/aatuh/api-toolkit/v2/specs"
)

const (
	// ExtensionTenant describes tenant requirements for route contract tooling.
	ExtensionTenant = "x-tenant"
	// ExtensionIdempotencyKey describes idempotency-key requirements for unsafe write routes.
	ExtensionIdempotencyKey = "x-idempotency-key"
	// ExtensionRateLimit describes rate-limit policy metadata for route tooling.
	ExtensionRateLimit = "x-rate-limit"
	// ExtensionAdminPolicy marks an operator-only route as intentionally protected.
	ExtensionAdminPolicy = "x-admin-policy"
)

// MetadataOption mutates an operation with stable api-toolkit policy metadata.
type MetadataOption func(*specs.Operation)

// ApplyMetadata applies stable metadata options to an operation.
func ApplyMetadata(operation specs.Operation, opts ...MetadataOption) specs.Operation {
	for _, opt := range opts {
		if opt != nil {
			opt(&operation)
		}
	}
	return operation
}

// WithOperationID sets the OpenAPI operationId.
func WithOperationID(id string) MetadataOption {
	return func(operation *specs.Operation) {
		operation.OperationID = strings.TrimSpace(id)
	}
}

// WithAuth appends a security requirement and mirrors scopes onto the operation.
func WithAuth(scheme string, scopes ...string) MetadataOption {
	return func(operation *specs.Operation) {
		scheme = strings.TrimSpace(scheme)
		if scheme == "" {
			return
		}
		cleanScopes := sortedNonEmpty(scopes)
		operation.Security = append(operation.Security, specs.SecurityRequirement{
			Name:   scheme,
			Scopes: cleanScopes,
		})
		operation.Scopes = mergeStrings(operation.Scopes, cleanScopes)
	}
}

// WithTenantRequired marks a route as tenant-scoped.
func WithTenantRequired(source string) MetadataOption {
	return func(operation *specs.Operation) {
		ensureExtensions(operation)
		source = strings.TrimSpace(source)
		if source == "" {
			source = "context"
		}
		operation.Extensions[ExtensionTenant] = map[string]any{
			"required": true,
			"source":   source,
		}
	}
}

// WithIdempotencyRequired marks a write route as requiring an idempotency key.
func WithIdempotencyRequired() MetadataOption {
	return func(operation *specs.Operation) {
		ensureExtensions(operation)
		operation.Extensions[ExtensionIdempotencyKey] = map[string]any{"required": true}
	}
}

// WithRateLimit records the named rate-limit policy for a route.
func WithRateLimit(policy string) MetadataOption {
	return func(operation *specs.Operation) {
		policy = strings.TrimSpace(policy)
		if policy == "" {
			return
		}
		ensureExtensions(operation)
		operation.Extensions[ExtensionRateLimit] = policy
	}
}

// WithAdminPolicy marks an operator-only route as protected by the named policy.
func WithAdminPolicy(policy string) MetadataOption {
	return func(operation *specs.Operation) {
		policy = strings.TrimSpace(policy)
		if policy == "" {
			policy = "admin"
		}
		ensureExtensions(operation)
		operation.Extensions[ExtensionAdminPolicy] = policy
	}
}

// WithProblemResponses registers standard Problem Details responses for statuses.
func WithProblemResponses(statuses ...int) MetadataOption {
	return func(operation *specs.Operation) {
		if operation.Responses == nil {
			operation.Responses = map[int]specs.Response{}
		}
		for _, status := range statuses {
			if status < 400 || status > 599 {
				continue
			}
			if _, exists := operation.Responses[status]; exists {
				continue
			}
			operation.Responses[status] = specs.ProblemResponse(http.StatusText(status))
		}
	}
}

// LintOptions configures route contract linting.
type LintOptions struct {
	RequireOperationID                bool
	RequireUniqueOperationID          bool
	RequireSecurity                   bool
	RequireUnsafeWriteAuth            bool
	RequireUnsafeWriteTenant          bool
	RequireUnsafeWriteIdempotency     bool
	RequireUnsafeWriteRateLimit       bool
	RequireUnsafeWriteProblemResponse bool
	PublicPaths                       []string
	AdminPaths                        []string
}

// LintFinding describes one route contract lint finding.
type LintFinding struct {
	Method  string
	Path    string
	Code    string
	Message string
}

// Error returns a stable human-readable finding string.
func (f LintFinding) Error() string {
	return fmt.Sprintf("%s %s: %s: %s", f.Method, f.Path, f.Code, f.Message)
}

// LintOperations validates operation metadata needed for production API contracts.
func LintOperations(operations []specs.Operation, opts LintOptions) []LintFinding {
	var findings []LintFinding
	operationIDs := map[string]specs.Operation{}
	for _, operation := range operations {
		method := strings.ToUpper(strings.TrimSpace(operation.Method))
		path := strings.TrimSpace(operation.Path)
		if method == "" || path == "" {
			continue
		}
		operationID := strings.TrimSpace(operation.OperationID)
		if opts.RequireOperationID && operationID == "" {
			findings = append(findings, finding(operation, "operation_id_required", "operationId is required for compatibility review"))
		}
		if opts.RequireUniqueOperationID && operationID != "" {
			if previous, ok := operationIDs[operationID]; ok {
				findings = append(findings, finding(operation, "operation_id_duplicate", fmt.Sprintf("operationId %q is already used by %s %s", operationID, strings.ToUpper(strings.TrimSpace(previous.Method)), strings.TrimSpace(previous.Path))))
			} else {
				operationIDs[operationID] = operation
			}
		}
		if opts.RequireSecurity && !matchesAnyPath(path, opts.PublicPaths) && !hasSecurity(operation) {
			findings = append(findings, finding(operation, "security_required", "non-public operations must declare security requirements or scopes"))
		}
		if isUnsafeWrite(method) {
			if opts.RequireUnsafeWriteAuth && !hasSecurity(operation) {
				findings = append(findings, finding(operation, "unsafe_write_auth_required", "unsafe write operations must declare auth or scopes"))
			}
			if opts.RequireUnsafeWriteTenant && !hasExtension(operation, ExtensionTenant) {
				findings = append(findings, finding(operation, "unsafe_write_tenant_required", "unsafe write operations must declare tenant policy"))
			}
			if opts.RequireUnsafeWriteIdempotency && !hasExtension(operation, ExtensionIdempotencyKey) {
				findings = append(findings, finding(operation, "unsafe_write_idempotency_required", "unsafe write operations must declare idempotency policy"))
			}
			if opts.RequireUnsafeWriteRateLimit && !hasExtension(operation, ExtensionRateLimit) {
				findings = append(findings, finding(operation, "unsafe_write_rate_limit_required", "unsafe write operations must declare rate-limit policy"))
			}
			if opts.RequireUnsafeWriteProblemResponse && !hasProblemResponse(operation) {
				findings = append(findings, finding(operation, "problem_response_required", "operation must document at least one Problem Details error response"))
			}
		}
		if matchesAnyPath(path, opts.AdminPaths) && !hasExtension(operation, ExtensionAdminPolicy) {
			findings = append(findings, finding(operation, "admin_policy_required", "operator-only route must declare admin policy metadata"))
		}
	}
	return findings
}

func ensureExtensions(operation *specs.Operation) {
	if operation.Extensions == nil {
		operation.Extensions = map[string]any{}
	}
}

func finding(operation specs.Operation, code, message string) LintFinding {
	return LintFinding{
		Method:  strings.ToUpper(strings.TrimSpace(operation.Method)),
		Path:    strings.TrimSpace(operation.Path),
		Code:    code,
		Message: message,
	}
}

func isUnsafeWrite(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func hasExtension(operation specs.Operation, name string) bool {
	if operation.Extensions == nil {
		return false
	}
	_, ok := operation.Extensions[name]
	return ok
}

func hasSecurity(operation specs.Operation) bool {
	if len(operation.Security) > 0 {
		return true
	}
	for _, scope := range operation.Scopes {
		if strings.TrimSpace(scope) != "" {
			return true
		}
	}
	return false
}

func hasProblemResponse(operation specs.Operation) bool {
	for status, response := range operation.Responses {
		if status < 400 || status > 599 {
			continue
		}
		if strings.Contains(response.Ref, "Problem") {
			return true
		}
		if _, ok := response.Content["application/problem+json"]; ok {
			return true
		}
		for _, contentType := range response.ContentTypes {
			if strings.EqualFold(strings.TrimSpace(contentType), "application/problem+json") {
				return true
			}
		}
	}
	return false
}

func matchesAnyPath(path string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "*") && strings.HasPrefix(path, strings.TrimSuffix(pattern, "*")) {
			return true
		}
		if path == pattern {
			return true
		}
	}
	return false
}

func sortedNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func mergeStrings(existing, values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range existing {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
