package httpx

import (
	"net/http"
)

// Problem represents an RFC 7807 problem+json response body.
// See: https://datatracker.ietf.org/doc/html/rfc7807
type Problem struct {
	Type     string         `json:"type,omitempty"`     // "https://example.com/validation-error"`
	Title    string         `json:"title,omitempty"`    // "Bad Request"`
	Status   int            `json:"status,omitempty"`   // 400
	Detail   string         `json:"detail,omitempty"`   // "name is required"
	Instance string         `json:"instance,omitempty"` // "/api/v1/foo/123"
	Ext      map[string]any `json:"-"`
}

// With adds an extension field to the problem payload.
func (p *Problem) With(key string, value any) *Problem {
	if key == "" {
		return p
	}
	if p.Ext == nil {
		p.Ext = make(map[string]any)
	}
	p.Ext[key] = value
	return p
}

// WriteProblem writes a problem+json response with the provided status code.
// It merges extension fields after the standard members, per RFC 7807.
func WriteProblem(w http.ResponseWriter, status int, p Problem) {
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	p.Status = status

	// Marshal with extensions by composing a map to preserve standard fields.
	out := map[string]any{}
	if p.Type != "" {
		out["type"] = p.Type
	}
	if p.Title != "" {
		out["title"] = p.Title
	}
	if p.Status != 0 {
		out["status"] = p.Status
	}
	if p.Detail != "" {
		out["detail"] = p.Detail
	}
	if p.Instance != "" {
		out["instance"] = p.Instance
	}
	for k, v := range p.Ext {
		if k == "type" || k == "title" || k == "status" ||
			k == "detail" || k == "instance" {
			continue
		}
		out[k] = v
	}
	if err := writeJSON(w, status, "application/problem+json", out); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// WriteSimpleProblem is a convenience for common cases.
func WriteSimpleProblem(w http.ResponseWriter, status int, title, detail string) {
	WriteProblem(w, status, Problem{Title: title, Detail: detail})
}
