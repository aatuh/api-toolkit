package specs

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Info describes OpenAPI metadata for a registry.
type Info struct {
	Title       string
	Description string
	Version     string
}

// Server describes an OpenAPI server entry.
type Server struct {
	URL         string
	Description string
}

// RequestBody describes request content metadata.
type RequestBody struct {
	Description  string
	Required     bool
	ContentTypes []string
}

// Response describes response content metadata.
type Response struct {
	Description  string
	ContentTypes []string
}

// Operation describes an API operation for registry usage.
type Operation struct {
	Method      string
	Path        string
	Summary     string
	Description string
	Tags        []string
	Deprecated  bool
	RequestBody *RequestBody
	Responses   map[int]Response
}

// Registry collects operations and produces a minimal OpenAPI document.
type Registry struct {
	mu      sync.RWMutex
	info    Info
	servers []Server
	ops     []Operation
}

// NewRegistry constructs an OpenAPI registry with the provided info.
func NewRegistry(info Info) *Registry {
	return &Registry{info: info}
}

// SetServers replaces the server list.
func (r *Registry) SetServers(servers []Server) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = append([]Server(nil), servers...)
}

// Register registers an operation in the registry.
func (r *Registry) Register(op Operation) {
	if r == nil {
		return
	}
	if op.Path == "" || op.Method == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ops = append(r.ops, op)
}

// OpenAPI returns a JSON-encoded OpenAPI 3.0 document.
func (r *Registry) OpenAPI() ([]byte, error) {
	if r == nil {
		return json.Marshal(defaultOpenAPI(Info{}))
	}
	r.mu.RLock()
	info := r.info
	servers := append([]Server(nil), r.servers...)
	ops := append([]Operation(nil), r.ops...)
	r.mu.RUnlock()

	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Path == ops[j].Path {
			return strings.ToUpper(ops[i].Method) < strings.ToUpper(ops[j].Method)
		}
		return ops[i].Path < ops[j].Path
	})

	paths := map[string]map[string]any{}
	for _, op := range ops {
		method := strings.ToLower(strings.TrimSpace(op.Method))
		if method == "" || op.Path == "" {
			continue
		}
		if _, ok := paths[op.Path]; !ok {
			paths[op.Path] = map[string]any{}
		}
		paths[op.Path][method] = buildOperation(op)
	}

	spec := defaultOpenAPI(info)
	spec["paths"] = paths
	if len(servers) > 0 {
		outServers := make([]map[string]any, 0, len(servers))
		for _, s := range servers {
			outServers = append(outServers, map[string]any{
				"url":         s.URL,
				"description": s.Description,
			})
		}
		spec["servers"] = outServers
	}
	return json.Marshal(spec)
}

func defaultOpenAPI(info Info) map[string]any {
	if strings.TrimSpace(info.Title) == "" {
		info.Title = "API"
	}
	if strings.TrimSpace(info.Version) == "" {
		info.Version = "0.0.0"
	}
	return map[string]any{
		"openapi": "3.0.0",
		"info": map[string]any{
			"title":       info.Title,
			"description": info.Description,
			"version":     info.Version,
		},
		"paths": map[string]any{},
	}
}

func buildOperation(op Operation) map[string]any {
	out := map[string]any{}
	if op.Summary != "" {
		out["summary"] = op.Summary
	}
	if op.Description != "" {
		out["description"] = op.Description
	}
	if len(op.Tags) > 0 {
		out["tags"] = append([]string(nil), op.Tags...)
	}
	if op.Deprecated {
		out["deprecated"] = true
	}
	if op.RequestBody != nil {
		out["requestBody"] = map[string]any{
			"description": op.RequestBody.Description,
			"required":    op.RequestBody.Required,
			"content":     contentObject(op.RequestBody.ContentTypes),
		}
	}
	responses := map[string]any{}
	if len(op.Responses) == 0 {
		responses[strconvStatus(http.StatusOK)] = map[string]any{
			"description": http.StatusText(http.StatusOK),
		}
	} else {
		for code, resp := range op.Responses {
			desc := resp.Description
			if desc == "" {
				desc = http.StatusText(code)
			}
			entry := map[string]any{
				"description": desc,
			}
			if len(resp.ContentTypes) > 0 {
				entry["content"] = contentObject(resp.ContentTypes)
			}
			responses[strconvStatus(code)] = entry
		}
	}
	out["responses"] = responses
	return out
}

func contentObject(contentTypes []string) map[string]any {
	if len(contentTypes) == 0 {
		contentTypes = []string{"application/json"}
	}
	out := map[string]any{}
	for _, ct := range contentTypes {
		ct = strings.TrimSpace(ct)
		if ct == "" {
			continue
		}
		out[ct] = map[string]any{
			"schema": map[string]any{
				"type": "object",
			},
		}
	}
	return out
}

func strconvStatus(code int) string {
	if code <= 0 {
		code = http.StatusOK
	}
	return strconv.Itoa(code)
}
