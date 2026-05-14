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
	Content      map[string]MediaType
}

// Response describes response content metadata.
type Response struct {
	Description  string
	ContentTypes []string
	Content      map[string]MediaType
	Ref          string
}

// MediaType describes OpenAPI media type metadata.
type MediaType struct {
	Schema    map[string]any
	SchemaRef string
	Example   any
	Examples  map[string]any
}

// Example describes a reusable OpenAPI example object.
type Example struct {
	Summary       string `json:"summary,omitempty"`
	Description   string `json:"description,omitempty"`
	Value         any    `json:"value,omitempty"`
	ExternalValue string `json:"externalValue,omitempty"`
}

// Parameter describes an OpenAPI operation parameter.
type Parameter struct {
	Name        string
	In          string
	Description string
	Required    bool
	Schema      map[string]any
}

// SecurityRequirement describes an OpenAPI security requirement.
type SecurityRequirement struct {
	Name   string
	Scopes []string
}

// SecurityScheme describes a reusable OpenAPI security scheme.
type SecurityScheme struct {
	Type             string
	Description      string
	Name             string
	In               string
	Scheme           string
	BearerFormat     string
	OpenIDConnectURL string
	Flows            map[string]any
}

// Components describes reusable OpenAPI components.
type Components struct {
	Schemas         map[string]map[string]any
	Responses       map[string]Response
	SecuritySchemes map[string]SecurityScheme
}

// Operation describes an API operation for registry usage.
type Operation struct {
	OperationID string
	Method      string
	Path        string
	Summary     string
	Description string
	Tags        []string
	Deprecated  bool
	Sunset      string
	Parameters  []Parameter
	Security    []SecurityRequirement
	Scopes      []string
	RequestBody *RequestBody
	Responses   map[int]Response
	Extensions  map[string]any
}

// Registry collects operations and produces a minimal OpenAPI document.
type Registry struct {
	mu             sync.RWMutex
	info           Info
	openAPIVersion string
	servers        []Server
	security       []SecurityRequirement
	ops            []Operation
	components     Components
}

// OpenAPIVersion identifies the OpenAPI document version emitted by a Registry.
type OpenAPIVersion string

const (
	// OpenAPIVersion30 is the default version emitted by NewRegistry.
	OpenAPIVersion30 OpenAPIVersion = "3.0.0"
	// OpenAPIVersion31 enables OpenAPI 3.1 output for services that opt in.
	OpenAPIVersion31 OpenAPIVersion = "3.1.0"
)

// RegistryOptions configures OpenAPI registry behavior.
type RegistryOptions struct {
	OpenAPIVersion OpenAPIVersion
}

// NewRegistry constructs an OpenAPI registry with the provided info.
func NewRegistry(info Info) *Registry {
	return NewRegistryWithOptions(info, RegistryOptions{})
}

// NewRegistryWithOptions constructs an OpenAPI registry with explicit options.
func NewRegistryWithOptions(info Info, opts RegistryOptions) *Registry {
	return &Registry{info: info, openAPIVersion: normalizeOpenAPIVersion(opts.OpenAPIVersion)}
}

// SetServers replaces the server list.
func (r *Registry) SetServers(servers []Server) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = append([]Server(nil), servers...)
}

// SetSecurity replaces top-level OpenAPI security requirements.
func (r *Registry) SetSecurity(requirements []SecurityRequirement) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.security = cloneSecurityRequirements(requirements)
}

// SetComponents replaces reusable OpenAPI components.
func (r *Registry) SetComponents(components Components) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.components = cloneComponents(components)
}

// RegisterSchema registers or replaces a reusable schema component.
func (r *Registry) RegisterSchema(name string, schema map[string]any) {
	if r == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" || len(schema) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.components.Schemas == nil {
		r.components.Schemas = map[string]map[string]any{}
	}
	r.components.Schemas[name] = cloneMap(schema)
}

// RegisterSecurityScheme registers or replaces a reusable security scheme.
func (r *Registry) RegisterSecurityScheme(name string, scheme SecurityScheme) {
	if r == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.components.SecuritySchemes == nil {
		r.components.SecuritySchemes = map[string]SecurityScheme{}
	}
	r.components.SecuritySchemes[name] = scheme
}

// RegisterResponse registers or replaces a reusable response component.
func (r *Registry) RegisterResponse(name string, response Response) {
	if r == nil {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.components.Responses == nil {
		r.components.Responses = map[string]Response{}
	}
	r.components.Responses[name] = response
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

// Operations returns the registered operations in deterministic OpenAPI order.
func (r *Registry) Operations() []Operation {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	ops := append([]Operation(nil), r.ops...)
	r.mu.RUnlock()
	sortOperations(ops)
	return ops
}

// OpenAPI returns a JSON-encoded OpenAPI document.
func (r *Registry) OpenAPI() ([]byte, error) {
	if r == nil {
		return json.Marshal(defaultOpenAPI(Info{}, string(OpenAPIVersion30)))
	}
	r.mu.RLock()
	info := r.info
	openAPIVersion := r.openAPIVersion
	servers := append([]Server(nil), r.servers...)
	security := cloneSecurityRequirements(r.security)
	ops := append([]Operation(nil), r.ops...)
	components := cloneComponents(r.components)
	r.mu.RUnlock()

	sortOperations(ops)

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

	spec := defaultOpenAPI(info, openAPIVersion)
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
	if outSecurity := securityObjects(security); len(outSecurity) > 0 {
		spec["security"] = outSecurity
	}
	if outComponents := componentsObject(components); len(outComponents) > 0 {
		spec["components"] = outComponents
	}
	return json.Marshal(spec)
}

func defaultOpenAPI(info Info, version string) map[string]any {
	if strings.TrimSpace(info.Title) == "" {
		info.Title = "API"
	}
	if strings.TrimSpace(info.Version) == "" {
		info.Version = "0.0.0"
	}
	if version == "" {
		version = string(OpenAPIVersion30)
	}
	return map[string]any{
		"openapi": version,
		"info": map[string]any{
			"title":       info.Title,
			"description": info.Description,
			"version":     info.Version,
		},
		"paths": map[string]any{},
	}
}

func normalizeOpenAPIVersion(version OpenAPIVersion) string {
	switch version {
	case OpenAPIVersion31:
		return string(OpenAPIVersion31)
	case OpenAPIVersion30:
		return string(OpenAPIVersion30)
	default:
		return string(OpenAPIVersion30)
	}
}

func buildOperation(op Operation) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(op.OperationID) != "" {
		out["operationId"] = strings.TrimSpace(op.OperationID)
	}
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
	if strings.TrimSpace(op.Sunset) != "" {
		out["x-sunset"] = strings.TrimSpace(op.Sunset)
	}
	if len(op.Scopes) > 0 {
		out["x-scopes"] = sortedStrings(op.Scopes)
	}
	if len(op.Parameters) > 0 {
		out["parameters"] = parameterObjects(op.Parameters)
	}
	if len(op.Security) > 0 {
		out["security"] = securityObjects(op.Security)
	}
	if op.RequestBody != nil {
		out["requestBody"] = map[string]any{
			"description": op.RequestBody.Description,
			"required":    op.RequestBody.Required,
			"content":     contentObject(op.RequestBody.ContentTypes, op.RequestBody.Content),
		}
	}
	responses := map[string]any{}
	if len(op.Responses) == 0 {
		responses[strconvStatus(http.StatusOK)] = map[string]any{
			"description": http.StatusText(http.StatusOK),
		}
	} else {
		for code, resp := range op.Responses {
			responses[strconvStatus(code)] = responseObject(resp, http.StatusText(code))
		}
	}
	out["responses"] = responses
	for key, value := range op.Extensions {
		if strings.HasPrefix(key, "x-") {
			out[key] = value
		}
	}
	return out
}

func sortOperations(ops []Operation) {
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Path == ops[j].Path {
			return strings.ToUpper(ops[i].Method) < strings.ToUpper(ops[j].Method)
		}
		return ops[i].Path < ops[j].Path
	})
}

func parameterObjects(parameters []Parameter) []map[string]any {
	params := append([]Parameter(nil), parameters...)
	sort.SliceStable(params, func(i, j int) bool {
		if params[i].In == params[j].In {
			return params[i].Name < params[j].Name
		}
		return params[i].In < params[j].In
	})
	out := make([]map[string]any, 0, len(params))
	for _, param := range params {
		name := strings.TrimSpace(param.Name)
		in := strings.TrimSpace(param.In)
		if name == "" || in == "" {
			continue
		}
		schema := param.Schema
		if len(schema) == 0 {
			schema = map[string]any{"type": "string"}
		}
		entry := map[string]any{
			"name":     name,
			"in":       in,
			"required": param.Required,
			"schema":   schema,
		}
		if param.Description != "" {
			entry["description"] = param.Description
		}
		out = append(out, entry)
	}
	return out
}

func securityObjects(requirements []SecurityRequirement) []map[string][]string {
	reqs := append([]SecurityRequirement(nil), requirements...)
	sort.SliceStable(reqs, func(i, j int) bool {
		return reqs[i].Name < reqs[j].Name
	})
	out := make([]map[string][]string, 0, len(reqs))
	for _, req := range reqs {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			continue
		}
		out = append(out, map[string][]string{name: sortedStrings(req.Scopes)})
	}
	return out
}

func cloneSecurityRequirements(requirements []SecurityRequirement) []SecurityRequirement {
	if len(requirements) == 0 {
		return nil
	}
	out := make([]SecurityRequirement, 0, len(requirements))
	for _, req := range requirements {
		name := strings.TrimSpace(req.Name)
		if name == "" {
			continue
		}
		out = append(out, SecurityRequirement{
			Name:   name,
			Scopes: append([]string(nil), req.Scopes...),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sortedStrings(values []string) []string {
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

func contentObject(contentTypes []string, content map[string]MediaType) map[string]any {
	out := map[string]any{}
	if len(content) > 0 {
		for ct, media := range content {
			ct = strings.TrimSpace(ct)
			if ct == "" {
				continue
			}
			out[ct] = mediaTypeObject(media)
		}
		return out
	}
	if len(contentTypes) == 0 {
		contentTypes = []string{"application/json"}
	}
	for _, ct := range contentTypes {
		ct = strings.TrimSpace(ct)
		if ct == "" {
			continue
		}
		out[ct] = mediaTypeObject(MediaType{})
	}
	return out
}

func strconvStatus(code int) string {
	if code <= 0 {
		code = http.StatusOK
	}
	return strconv.Itoa(code)
}

func mediaTypeObject(media MediaType) map[string]any {
	out := map[string]any{}
	switch {
	case strings.TrimSpace(media.SchemaRef) != "":
		out["schema"] = map[string]any{"$ref": strings.TrimSpace(media.SchemaRef)}
	case len(media.Schema) > 0:
		out["schema"] = cloneAnyMap(media.Schema)
	default:
		out["schema"] = map[string]any{"type": "object"}
	}
	if media.Example != nil {
		out["example"] = media.Example
	}
	if len(media.Examples) > 0 {
		out["examples"] = cloneAnyMap(media.Examples)
	}
	return out
}

func responseObject(response Response, defaultDescription string) map[string]any {
	if strings.TrimSpace(response.Ref) != "" {
		return map[string]any{"$ref": strings.TrimSpace(response.Ref)}
	}
	desc := response.Description
	if desc == "" {
		desc = defaultDescription
	}
	entry := map[string]any{"description": desc}
	if len(response.ContentTypes) > 0 || len(response.Content) > 0 {
		entry["content"] = contentObject(response.ContentTypes, response.Content)
	}
	return entry
}

func componentsObject(components Components) map[string]any {
	out := map[string]any{}
	if len(components.Schemas) > 0 {
		schemas := map[string]any{}
		for name, schema := range components.Schemas {
			name = strings.TrimSpace(name)
			if name != "" && len(schema) > 0 {
				schemas[name] = cloneAnyMap(schema)
			}
		}
		if len(schemas) > 0 {
			out["schemas"] = schemas
		}
	}
	if len(components.Responses) > 0 {
		responses := map[string]any{}
		for name, response := range components.Responses {
			name = strings.TrimSpace(name)
			if name != "" {
				responses[name] = responseObject(response, "Response")
			}
		}
		if len(responses) > 0 {
			out["responses"] = responses
		}
	}
	if len(components.SecuritySchemes) > 0 {
		schemes := map[string]any{}
		for name, scheme := range components.SecuritySchemes {
			name = strings.TrimSpace(name)
			if name != "" {
				schemes[name] = securitySchemeObject(scheme)
			}
		}
		if len(schemes) > 0 {
			out["securitySchemes"] = schemes
		}
	}
	return out
}

func securitySchemeObject(scheme SecurityScheme) map[string]any {
	out := map[string]any{}
	if scheme.Type != "" {
		out["type"] = scheme.Type
	}
	if scheme.Description != "" {
		out["description"] = scheme.Description
	}
	if scheme.Name != "" {
		out["name"] = scheme.Name
	}
	if scheme.In != "" {
		out["in"] = scheme.In
	}
	if scheme.Scheme != "" {
		out["scheme"] = scheme.Scheme
	}
	if scheme.BearerFormat != "" {
		out["bearerFormat"] = scheme.BearerFormat
	}
	if scheme.OpenIDConnectURL != "" {
		out["openIdConnectUrl"] = scheme.OpenIDConnectURL
	}
	if len(scheme.Flows) > 0 {
		out["flows"] = cloneAnyMap(scheme.Flows)
	}
	return out
}

func cloneComponents(components Components) Components {
	out := Components{}
	if len(components.Schemas) > 0 {
		out.Schemas = map[string]map[string]any{}
		for name, schema := range components.Schemas {
			out.Schemas[name] = cloneMap(schema)
		}
	}
	if len(components.Responses) > 0 {
		out.Responses = map[string]Response{}
		for name, response := range components.Responses {
			out.Responses[name] = response
		}
	}
	if len(components.SecuritySchemes) > 0 {
		out.SecuritySchemes = map[string]SecurityScheme{}
		for name, scheme := range components.SecuritySchemes {
			out.SecuritySchemes[name] = scheme
		}
	}
	return out
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	return cloneAnyMap(values)
}

func cloneAnyMap(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
