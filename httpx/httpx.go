package httpx

import (
	"encoding/json"
	"net/http"
)

// Problem represents an RFC 7807 problem+json response body.
// See: https://datatracker.ietf.org/doc/html/rfc7807
type Problem struct {
	Type     string         `json:"type,omitempty"`
	Title    string         `json:"title,omitempty"`
	Status   int            `json:"status,omitempty"`
	Detail   string         `json:"detail,omitempty"`
	Instance string         `json:"instance,omitempty"`
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

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

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
	_ = json.NewEncoder(w).Encode(out)
}

// WriteSimpleProblem is a convenience for common cases.
func WriteSimpleProblem(w http.ResponseWriter, status int, title, detail string) {
	WriteProblem(w, status, Problem{Title: title, Detail: detail})
}
