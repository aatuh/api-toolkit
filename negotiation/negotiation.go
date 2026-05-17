package negotiation

import (
	"fmt"
	"mime"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/aatuh/api-toolkit/v3/httpx"
)

// MediaType is a normalized HTTP media type.
type MediaType string

// Accept is one parsed Accept header item.
type Accept struct {
	MediaType string
	Type      string
	Subtype   string
	Params    map[string]string
	Q         float64
	Order     int
}

// Config configures content negotiation middleware.
type Config struct {
	Accept       []MediaType
	ContentTypes []MediaType
}

// Middleware enforces Accept and Content-Type policies.
type Middleware struct {
	config Config
}

// New constructs content negotiation middleware.
func New(config Config) (*Middleware, error) {
	for _, mediaType := range append(append([]MediaType(nil), config.Accept...), config.ContentTypes...) {
		if _, err := parseMediaType(string(mediaType)); err != nil {
			return nil, err
		}
	}
	return &Middleware{config: config}, nil
}

// Middleware returns the standard middleware adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps next with Accept and Content-Type checks.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(m.config.Accept) > 0 {
			if _, ok := Negotiate(r.Header.Get("Accept"), m.config.Accept); !ok {
				writeNotAcceptable(w, m.config.Accept)
				return
			}
		}
		if len(m.config.ContentTypes) > 0 && shouldCheckContentType(r) {
			if !ContentTypeAllowed(r.Header.Get("Content-Type"), m.config.ContentTypes) {
				writeUnsupportedMediaType(w, m.config.ContentTypes)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAccept returns middleware that enforces response media negotiation.
func RequireAccept(allowed ...MediaType) func(http.Handler) http.Handler {
	mw, err := New(Config{Accept: allowed})
	if err != nil {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeNotAcceptable(w, allowed)
			})
		}
	}
	return mw.Middleware()
}

// RequireContentType returns middleware that enforces request Content-Type.
func RequireContentType(allowed ...MediaType) func(http.Handler) http.Handler {
	mw, err := New(Config{ContentTypes: allowed})
	if err != nil {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeUnsupportedMediaType(w, allowed)
			})
		}
	}
	return mw.Middleware()
}

// ParseAccept parses an Accept header and sorts values by client preference.
func ParseAccept(header string) ([]Accept, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil, nil
	}
	parts := strings.Split(header, ",")
	out := make([]Accept, 0, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		mediaType, params, err := mime.ParseMediaType(part)
		if err != nil {
			return nil, err
		}
		parsed, err := parseMediaType(mediaType)
		if err != nil {
			return nil, err
		}
		q := 1.0
		if rawQ := strings.TrimSpace(params["q"]); rawQ != "" {
			parsedQ, err := strconv.ParseFloat(rawQ, 64)
			if err != nil || parsedQ < 0 || parsedQ > 1 {
				return nil, fmt.Errorf("invalid Accept q value")
			}
			q = parsedQ
			delete(params, "q")
		}
		parsed.Params = params
		parsed.Q = q
		parsed.Order = i
		out = append(out, parsed)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Q != out[j].Q {
			return out[i].Q > out[j].Q
		}
		if specificity(out[i]) != specificity(out[j]) {
			return specificity(out[i]) > specificity(out[j])
		}
		return out[i].Order < out[j].Order
	})
	return out, nil
}

// Negotiate chooses the best offered media type for an Accept header.
func Negotiate(header string, offered []MediaType) (MediaType, bool) {
	normalized := normalizeOffered(offered)
	if len(normalized) == 0 {
		return "", false
	}
	accepts, err := ParseAccept(header)
	if err != nil {
		return "", false
	}
	if len(accepts) == 0 {
		return normalized[0], true
	}
	for _, accept := range accepts {
		if accept.Q == 0 {
			continue
		}
		for _, offered := range normalized {
			parsedOffered, err := parseMediaType(string(offered))
			if err != nil {
				continue
			}
			if acceptMatches(accept, parsedOffered) {
				return offered, true
			}
		}
	}
	return "", false
}

// ContentTypeAllowed reports whether a Content-Type is allowed.
func ContentTypeAllowed(header string, allowed []MediaType) bool {
	mediaType, err := parseMediaTypeFromHeader(header)
	if err != nil {
		return false
	}
	for _, allowedType := range normalizeOffered(allowed) {
		parsedAllowed, err := parseMediaType(string(allowedType))
		if err != nil {
			continue
		}
		if mediaTypeMatches(parsedAllowed, mediaType) {
			return true
		}
	}
	return false
}

func parseMediaTypeFromHeader(header string) (Accept, error) {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(header))
	if err != nil {
		return Accept{}, err
	}
	return parseMediaType(mediaType)
}

func parseMediaType(value string) (Accept, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Accept{}, fmt.Errorf("invalid media type %q", value)
	}
	return Accept{MediaType: value, Type: parts[0], Subtype: parts[1], Q: 1}, nil
}

func normalizeOffered(offered []MediaType) []MediaType {
	out := make([]MediaType, 0, len(offered))
	for _, value := range offered {
		parsed, err := parseMediaType(string(value))
		if err == nil {
			out = append(out, MediaType(parsed.MediaType))
		}
	}
	return out
}

func acceptMatches(accept, offered Accept) bool {
	if accept.Type != "*" && accept.Type != offered.Type {
		return false
	}
	if accept.Subtype == "*" || accept.Subtype == offered.Subtype {
		return true
	}
	return suffixCompatible(accept.Subtype, offered.Subtype)
}

func mediaTypeMatches(allowed, got Accept) bool {
	if allowed.Type != got.Type {
		return false
	}
	if allowed.Subtype == got.Subtype {
		return true
	}
	return suffixCompatible(allowed.Subtype, got.Subtype)
}

func suffixCompatible(a, b string) bool {
	return (a == "json" && strings.HasSuffix(b, "+json")) ||
		(b == "json" && strings.HasSuffix(a, "+json")) ||
		(strings.HasPrefix(a, "*+") && strings.HasSuffix(b, strings.TrimPrefix(a, "*"))) ||
		(strings.HasPrefix(b, "*+") && strings.HasSuffix(a, strings.TrimPrefix(b, "*")))
}

func specificity(accept Accept) int {
	score := 0
	if accept.Type != "*" {
		score++
	}
	if accept.Subtype != "*" {
		score++
	}
	return score
}

func shouldCheckContentType(r *http.Request) bool {
	if r == nil {
		return false
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return false
	}
	return r.ContentLength > 0 || len(r.TransferEncoding) > 0
}

func writeNotAcceptable(w http.ResponseWriter, allowed []MediaType) {
	problem := httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeNotAcceptable),
		Title:  http.StatusText(http.StatusNotAcceptable),
		Detail: "requested response media type is not available",
	}
	problem.With("acceptable", mediaTypeStrings(allowed))
	httpx.WriteProblem(w, http.StatusNotAcceptable, problem)
}

func writeUnsupportedMediaType(w http.ResponseWriter, allowed []MediaType) {
	problem := httpx.Problem{
		Type:   httpx.DefaultTypeURI(httpx.TypeUnsupportedMedia),
		Title:  http.StatusText(http.StatusUnsupportedMediaType),
		Detail: "request content type is not supported",
	}
	problem.With("supported", mediaTypeStrings(allowed))
	httpx.WriteProblem(w, http.StatusUnsupportedMediaType, problem)
}

func mediaTypeStrings(values []MediaType) []string {
	out := make([]string, 0, len(values))
	for _, value := range normalizeOffered(values) {
		out = append(out, string(value))
	}
	sort.Strings(out)
	return out
}
