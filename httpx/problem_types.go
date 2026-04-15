package httpx

import "strings"

// DefaultTypeBase is the canonical base URI for toolkit problem types.
const DefaultTypeBase = "https://api-toolkit.dev/problems"

// Standard problem type slugs.
const (
	TypeBadRequest         = "bad-request"
	TypeValidation         = "validation-error"
	TypeUnsupportedMedia   = "unsupported-media-type"
	TypeUnauthorized       = "unauthorized"
	TypeForbidden          = "forbidden"
	TypeNotFound           = "not-found"
	TypeConflict           = "conflict"
	TypePayloadTooLarge    = "payload-too-large"
	TypeRateLimited        = "rate-limit-exceeded"
	TypeServiceUnavailable = "service-unavailable"
	TypeInternal           = "internal-error"
)

// TypeRegistry helps build consistent problem type URIs.
type TypeRegistry struct {
	base  string
	types map[string]string
}

// NewTypeRegistry registers slugs against a base URI.
func NewTypeRegistry(base string, slugs ...string) *TypeRegistry {
	r := &TypeRegistry{
		base:  strings.TrimRight(strings.TrimSpace(base), "/"),
		types: make(map[string]string, len(slugs)),
	}
	for _, slug := range slugs {
		r.Register(slug)
	}
	return r
}

// DefaultTypeRegistry returns a registry with the toolkit's standard type slugs.
func DefaultTypeRegistry() *TypeRegistry {
	return NewTypeRegistry(DefaultTypeBase,
		TypeBadRequest,
		TypeValidation,
		TypeUnsupportedMedia,
		TypeUnauthorized,
		TypeForbidden,
		TypeNotFound,
		TypeConflict,
		TypePayloadTooLarge,
		TypeRateLimited,
		TypeServiceUnavailable,
		TypeInternal,
	)
}

var defaultTypeRegistry = DefaultTypeRegistry()

// DefaultTypeURI resolves a standard type slug to a full URI.
func DefaultTypeURI(slug string) string {
	return defaultTypeRegistry.URI(slug)
}

// Register adds a slug to the registry and returns its URI.
func (r *TypeRegistry) Register(slug string) string {
	if r == nil {
		return ""
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ""
	}
	if isAbsoluteType(slug) {
		r.types[slug] = slug
		return slug
	}
	normalized := normalizeSlug(slug)
	uri := TypeURI(r.base, normalized)
	r.types[normalized] = uri
	return uri
}

// URI returns the full URI for a slug, falling back to base + slug.
func (r *TypeRegistry) URI(slug string) string {
	if r == nil {
		return ""
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ""
	}
	if isAbsoluteType(slug) {
		return slug
	}
	normalized := normalizeSlug(slug)
	if uri, ok := r.types[normalized]; ok {
		return uri
	}
	return TypeURI(r.base, normalized)
}

// Types returns a copy of the registered type map.
func (r *TypeRegistry) Types() map[string]string {
	if r == nil || len(r.types) == 0 {
		return nil
	}
	out := make(map[string]string, len(r.types))
	for k, v := range r.types {
		out[k] = v
	}
	return out
}

// TypeURI builds a type URI from base and slug.
func TypeURI(base, slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ""
	}
	if isAbsoluteType(slug) {
		return slug
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return slug
	}
	return base + "/" + normalizeSlug(slug)
}

func normalizeSlug(slug string) string {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return ""
	}
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = strings.ReplaceAll(slug, " ", "-")
	return slug
}

func isAbsoluteType(value string) bool {
	if strings.HasPrefix(value, "about:") || strings.HasPrefix(value, "urn:") {
		return true
	}
	return strings.Contains(value, "://")
}
