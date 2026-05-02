package deprecation

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Config configures runtime deprecation and sunset response headers.
type Config struct {
	Disabled     bool
	DeprecatedAt time.Time
	SunsetAt     time.Time
	Links        []Link
}

// Link describes a Link header associated with deprecation or sunset policy.
type Link struct {
	URL   string
	Rel   string
	Type  string
	Title string
}

// Middleware emits deprecation headers without changing response behavior.
type Middleware struct {
	config Config
}

// New constructs deprecation middleware.
func New(config Config) (*Middleware, error) {
	if !config.DeprecatedAt.IsZero() && !config.SunsetAt.IsZero() && config.SunsetAt.Before(config.DeprecatedAt) {
		return nil, fmt.Errorf("sunset must not be before deprecation")
	}
	return &Middleware{config: config}, nil
}

// Handler constructs middleware and wraps next.
func Handler(config Config, next http.Handler) (http.Handler, error) {
	mw, err := New(config)
	if err != nil {
		return nil, err
	}
	return mw.Handler(next), nil
}

// Handler wraps the next handler.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if next == nil {
		next = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m != nil && !m.config.Disabled {
			writeHeaders(w.Header(), m.config)
		}
		next.ServeHTTP(w, r)
	})
}

func writeHeaders(header http.Header, config Config) {
	if !config.DeprecatedAt.IsZero() {
		header.Set("Deprecation", fmt.Sprintf("@%d", config.DeprecatedAt.UTC().Unix()))
	}
	if !config.SunsetAt.IsZero() {
		header.Set("Sunset", config.SunsetAt.UTC().Format(http.TimeFormat))
	}
	for _, link := range config.Links {
		value := linkHeader(link)
		if value != "" {
			header.Add("Link", value)
		}
	}
}

func linkHeader(link Link) string {
	url := strings.TrimSpace(link.URL)
	if url == "" {
		return ""
	}
	rel := strings.TrimSpace(link.Rel)
	if rel == "" {
		rel = "deprecation"
	}
	parts := []string{"<" + url + ">", `rel="` + quoteParam(rel) + `"`}
	if value := strings.TrimSpace(link.Type); value != "" {
		parts = append(parts, `type="`+quoteParam(value)+`"`)
	}
	if value := strings.TrimSpace(link.Title); value != "" {
		parts = append(parts, `title="`+quoteParam(value)+`"`)
	}
	return strings.Join(parts, "; ")
}

func quoteParam(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}
