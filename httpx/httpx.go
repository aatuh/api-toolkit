package httpx

import (
	"net/http"
)

// Problem represents an RFC 9457 problem details response body.
// RFC 9457 obsoletes RFC 7807.
// See: https://www.rfc-editor.org/rfc/rfc9457.html
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

// WriteProblem writes a problem details response with the provided status code.
// It merges extension fields after the standard members, per RFC 9457.
func WriteProblem(w http.ResponseWriter, status int, p Problem) {
	if err := WriteProblemChecked(w, status, p); responseWriteStage(err) == ResponseWriteStageEncode {
		// Encoding failed before the response was committed. Preserve the legacy
		// void API's best-effort fallback only in that safe case.
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// WriteProblemChecked writes an RFC 9457 problem details response and returns
// an error when encoding, header commitment, or body writing fails. It never
// writes a fallback response after a failure.
func WriteProblemChecked(w http.ResponseWriter, status int, p Problem) error {
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
	return writeJSONChecked(w, status, "application/problem+json", out)
}

// WriteSimpleProblem is a convenience for common cases.
func WriteSimpleProblem(w http.ResponseWriter, status int, title, detail string) {
	WriteProblem(w, status, Problem{Title: title, Detail: detail})
}
