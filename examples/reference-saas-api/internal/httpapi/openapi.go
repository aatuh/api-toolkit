package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aatuh/api-toolkit/v3/routepolicy"
	"github.com/aatuh/api-toolkit/v3/specs"
)

func OpenAPIDocument() ([]byte, error) {
	registry := specs.NewRegistryWithOptions(specs.Info{
		Title:       "Full SaaS API",
		Description: "Generated api-toolkit full SaaS/API profile.",
		Version:     "dev",
	}, specs.RegistryOptions{
		OpenAPIVersion: specs.OpenAPIVersion31,
	})
	registry.RegisterSecurityScheme("ApiKeyAuth", specs.SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"})
	registry.SetSecurity([]specs.SecurityRequirement{{Name: "ApiKeyAuth"}})

	registerSchemas(registry)
	specs.RegisterProblemCatalog(registry, nil)
	for _, operation := range operations() {
		registry.Register(operation)
	}
	doc, err := registry.OpenAPI()
	if err != nil {
		return nil, fmt.Errorf("render openapi: %w", err)
	}
	return normalizeJSON(doc)
}

func registerSchemas(registry *specs.Registry) {
	registry.RegisterSchema("Widget", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "name", "version"},
		"properties": map[string]any{
			"id":        map[string]any{"type": "string"},
			"tenant_id": map[string]any{"type": "string"},
			"name":      map[string]any{"type": "string"},
			"version":   map[string]any{"type": "integer", "format": "int64"},
		},
	})
	registry.RegisterSchema("WidgetCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"name"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
		},
	})
	registry.RegisterSchema("WidgetList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items":       map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Widget"}},
			"next_cursor": map[string]any{"type": "string", "nullable": true},
		},
	})
	registry.RegisterSchema("WidgetImportItem", map[string]any{
		"type":                 "object",
		"required":             []string{"name"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
		},
	})
	registry.RegisterSchema("WidgetImportRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"items"},
		"additionalProperties": false,
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/WidgetImportItem"}, "minItems": 1, "maxItems": 100},
		},
	})
	registry.RegisterSchema("WidgetImportResult", map[string]any{
		"type":     "object",
		"required": []string{"created", "widget_ids"},
		"properties": map[string]any{
			"created":    map[string]any{"type": "integer", "minimum": 0},
			"widget_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	})
	registry.RegisterSchema("OperationAccepted", map[string]any{
		"type":     "object",
		"required": []string{"state"},
		"properties": map[string]any{
			"id":       map[string]any{"type": "string"},
			"state":    map[string]any{"type": "string", "enum": []string{"pending"}},
			"location": map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("WidgetImportOperation", map[string]any{
		"type":     "object",
		"required": []string{"id", "state"},
		"properties": map[string]any{
			"id":      map[string]any{"type": "string"},
			"state":   map[string]any{"type": "string", "enum": []string{"pending", "running", "succeeded", "failed", "canceled"}},
			"result":  map[string]any{"$ref": "#/components/schemas/WidgetImportResult", "nullable": true},
			"problem": map[string]any{"$ref": "#/components/schemas/Problem", "nullable": true},
		},
	})
	registry.RegisterSchema("Organization", map[string]any{
		"type":     "object",
		"required": []string{"id", "name", "created_at", "updated_at"},
		"properties": map[string]any{
			"id":         map[string]any{"type": "string"},
			"name":       map[string]any{"type": "string"},
			"created_at": map[string]any{"type": "string", "format": "date-time"},
			"updated_at": map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("OrganizationCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"name"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "minLength": 1, "maxLength": 160},
		},
	})
	registry.RegisterSchema("OrganizationList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Organization"}},
		},
	})
	registry.RegisterSchema("Membership", map[string]any{
		"type":     "object",
		"required": []string{"organization_id", "user_id", "role", "created_at"},
		"properties": map[string]any{
			"organization_id": map[string]any{"type": "string"},
			"user_id":         map[string]any{"type": "string"},
			"role":            map[string]any{"type": "string", "enum": []string{"owner", "admin", "member", "viewer"}},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("MembershipList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Membership"}},
		},
	})
	registry.RegisterSchema("Invitation", map[string]any{
		"type":     "object",
		"required": []string{"id", "organization_id", "email", "role", "token_prefix", "expires_at", "created_at"},
		"properties": map[string]any{
			"id":              map[string]any{"type": "string"},
			"organization_id": map[string]any{"type": "string"},
			"email":           map[string]any{"type": "string", "format": "email"},
			"role":            map[string]any{"type": "string", "enum": []string{"owner", "admin", "member", "viewer"}},
			"token_prefix":    map[string]any{"type": "string"},
			"expires_at":      map[string]any{"type": "string", "format": "date-time"},
			"accepted_at":     map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("InvitationCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"email", "role"},
		"additionalProperties": false,
		"properties": map[string]any{
			"email": map[string]any{"type": "string", "format": "email"},
			"role":  map[string]any{"type": "string", "enum": []string{"admin", "member", "viewer"}},
		},
	})
	registry.RegisterSchema("InvitationCreated", map[string]any{
		"type":     "object",
		"required": []string{"invitation", "token"},
		"properties": map[string]any{
			"invitation": map[string]any{"$ref": "#/components/schemas/Invitation"},
			"token":      map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("InvitationAcceptRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"token"},
		"additionalProperties": false,
		"properties": map[string]any{
			"token": map[string]any{"type": "string", "minLength": 1},
		},
	})
	registry.RegisterSchema("APIKey", map[string]any{
		"type":     "object",
		"required": []string{"id", "organization_id", "name", "prefix", "scopes", "created_at"},
		"properties": map[string]any{
			"id":              map[string]any{"type": "string"},
			"organization_id": map[string]any{"type": "string"},
			"name":            map[string]any{"type": "string"},
			"prefix":          map[string]any{"type": "string"},
			"scopes":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"expires_at":      map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"last_used_at":    map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"revoked_at":      map[string]any{"type": "string", "format": "date-time", "nullable": true},
			"created_at":      map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("APIKeyCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"name", "scopes"},
		"additionalProperties": false,
		"properties": map[string]any{
			"name":       map[string]any{"type": "string", "minLength": 1, "maxLength": 120},
			"scopes":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
			"expires_at": map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("APIKeyCreated", map[string]any{
		"type":     "object",
		"required": []string{"api_key", "secret"},
		"properties": map[string]any{
			"api_key": map[string]any{"$ref": "#/components/schemas/APIKey"},
			"secret":  map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("APIKeyList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/APIKey"}},
		},
	})
	registry.RegisterSchema("WebhookEventCatalog", map[string]any{
		"type":     "object",
		"required": []string{"event_types"},
		"properties": map[string]any{
			"event_types": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []string{
				"widget.created",
				"widget.updated",
				"widget.deleted",
				"widget.import.completed",
				// api-toolkit:openapi-webhook-event-types
			}}},
		},
	})
	registry.RegisterSchema("Object", map[string]any{
		"type":     "object",
		"required": []string{"tenant_id", "key", "content_type", "size", "created_at", "updated_at"},
		"properties": map[string]any{
			"tenant_id":    map[string]any{"type": "string"},
			"key":          map[string]any{"type": "string"},
			"content_type": map[string]any{"type": "string"},
			"size":         map[string]any{"type": "integer", "minimum": 0},
			"created_at":   map[string]any{"type": "string", "format": "date-time"},
			"updated_at":   map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("ObjectPutRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"key", "content_type", "content_base64"},
		"additionalProperties": false,
		"properties": map[string]any{
			"key":            map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
			"content_type":   map[string]any{"type": "string", "enum": []string{"application/json", "application/pdf", "image/jpeg", "image/png", "text/plain"}},
			"content_base64": map[string]any{"type": "string", "format": "byte"},
		},
	})
	registry.RegisterSchema("ObjectRead", map[string]any{
		"type":     "object",
		"required": []string{"object", "content_base64"},
		"properties": map[string]any{
			"object":         map[string]any{"$ref": "#/components/schemas/Object"},
			"content_base64": map[string]any{"type": "string", "format": "byte"},
		},
	})
	registry.RegisterSchema("ObjectList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Object"}},
		},
	})
	registry.RegisterSchema("WebhookEndpoint", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "url", "events", "created_at"},
		"properties": map[string]any{
			"id":         map[string]any{"type": "string"},
			"tenant_id":  map[string]any{"type": "string"},
			"url":        map[string]any{"type": "string", "format": "uri"},
			"events":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"disabled":   map[string]any{"type": "boolean"},
			"created_at": map[string]any{"type": "string", "format": "date-time"},
			"updated_at": map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("WebhookEndpointCreateRequest", map[string]any{
		"type":                 "object",
		"required":             []string{"url", "events"},
		"additionalProperties": false,
		"properties": map[string]any{
			"url":    map[string]any{"type": "string", "format": "uri"},
			"events": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
		},
	})
	registry.RegisterSchema("WebhookEndpointCreated", map[string]any{
		"type":     "object",
		"required": []string{"endpoint", "secret"},
		"properties": map[string]any{
			"endpoint": map[string]any{"$ref": "#/components/schemas/WebhookEndpoint"},
			"secret":   map[string]any{"type": "string"},
		},
	})
	registry.RegisterSchema("WebhookEndpointList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/WebhookEndpoint"}},
		},
	})
	registry.RegisterSchema("WebhookDelivery", map[string]any{
		"type":     "object",
		"required": []string{"id", "tenant_id", "endpoint_id", "event_id", "event_type", "url", "state", "attempt", "next_at", "created_at", "updated_at"},
		"properties": map[string]any{
			"id":               map[string]any{"type": "string"},
			"tenant_id":        map[string]any{"type": "string"},
			"endpoint_id":      map[string]any{"type": "string"},
			"event_id":         map[string]any{"type": "string"},
			"event_type":       map[string]any{"type": "string"},
			"url":              map[string]any{"type": "string", "format": "uri"},
			"state":            map[string]any{"type": "string", "enum": []string{"pending", "leased", "succeeded", "failed", "dead_letter"}},
			"attempt":          map[string]any{"type": "integer", "minimum": 0},
			"next_at":          map[string]any{"type": "string", "format": "date-time"},
			"last_status_code": map[string]any{"type": "integer", "nullable": true},
			"last_error":       map[string]any{"type": "string", "nullable": true},
			"created_at":       map[string]any{"type": "string", "format": "date-time"},
			"updated_at":       map[string]any{"type": "string", "format": "date-time"},
		},
	})
	registry.RegisterSchema("WebhookDeliveryList", map[string]any{
		"type":     "object",
		"required": []string{"items"},
		"properties": map[string]any{
			"items": map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/WebhookDelivery"}},
		},
	})
	registry.RegisterSchema("WebhookReplayRequest", map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           map[string]any{},
	})

	// api-toolkit:openapi-schemas
}

func operations() []specs.Operation {
	auth := func(scopes ...string) []specs.SecurityRequirement {
		return []specs.SecurityRequirement{{Name: "ApiKeyAuth", Scopes: scopes}}
	}
	problemStatuses := []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusPreconditionFailed, http.StatusTooManyRequests}
	jsonBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WidgetCreateRequest"},
		},
	}
	widgetResponse := specs.Response{
		Description: "Widget",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Widget"},
		},
	}
	widgetImportBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WidgetImportRequest"},
		},
	}
	operationAcceptedResponse := specs.Response{
		Description: "Operation accepted",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/OperationAccepted"},
		},
	}
	operationResponse := specs.Response{
		Description: "Operation",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WidgetImportOperation"},
		},
	}
	organizationCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/OrganizationCreateRequest"},
		},
	}
	invitationCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/InvitationCreateRequest"},
		},
	}
	invitationAcceptBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/InvitationAcceptRequest"},
		},
	}
	organizationResponse := specs.Response{
		Description: "Organization",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Organization"},
		},
	}
	membershipResponse := specs.Response{
		Description: "Membership",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Membership"},
		},
	}
	invitationCreatedResponse := specs.Response{
		Description: "Invitation created",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/InvitationCreated"},
		},
	}
	apiKeyCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/APIKeyCreateRequest"},
		},
	}
	apiKeyCreatedResponse := specs.Response{
		Description: "API key created",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/APIKeyCreated"},
		},
	}
	webhookEventCatalogResponse := specs.Response{
		Description: "Webhook event catalog",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEventCatalog"},
		},
	}
	webhookEndpointCreateBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEndpointCreateRequest"},
		},
	}
	webhookEndpointCreatedResponse := specs.Response{
		Description: "Webhook endpoint created",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEndpointCreated"},
		},
	}
	webhookEndpointListResponse := specs.Response{
		Description: "Webhook endpoint list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookEndpointList"},
		},
	}
	webhookDeliveryListResponse := specs.Response{
		Description: "Webhook delivery list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookDeliveryList"},
		},
	}
	webhookDeliveryResponse := specs.Response{
		Description: "Webhook delivery",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookDelivery"},
		},
	}
	webhookReplayBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/WebhookReplayRequest"},
		},
	}
	objectPutBody := &specs.RequestBody{
		Required: true,
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/ObjectPutRequest"},
		},
	}
	objectResponse := specs.Response{
		Description: "Object",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/Object"},
		},
	}
	objectReadResponse := specs.Response{
		Description: "Object content",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/ObjectRead"},
		},
	}
	objectListResponse := specs.Response{
		Description: "Object list",
		Content: map[string]specs.MediaType{
			"application/json": {SchemaRef: "#/components/schemas/ObjectList"},
		},
	}

	// api-toolkit:openapi-operation-variables
	operations := []specs.Operation{
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getLiveness",
			Method:      http.MethodGet,
			Path:        "/livez",
			Summary:     "Liveness",
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Live"}},
		}),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getReadiness",
			Method:      http.MethodGet,
			Path:        "/readyz",
			Summary:     "Readiness",
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Ready"}},
		}),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getOpenAPI",
			Method:      http.MethodGet,
			Path:        "/docs/openapi.json",
			Summary:     "OpenAPI document",
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "OpenAPI document"}},
		}),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getDetailedHealth",
			Method:      http.MethodGet,
			Path:        "/health/detailed",
			Summary:     "Detailed health",
			Security:    auth("admin:read"),
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Detailed health"}},
		}, routepolicy.WithAdminPolicy("admin"), routepolicy.WithProblemResponses(http.StatusUnauthorized, http.StatusForbidden)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getMetrics",
			Method:      http.MethodGet,
			Path:        "/metrics",
			Summary:     "Metrics",
			Security:    auth("admin:read"),
			Responses:   map[int]specs.Response{http.StatusOK: {Description: "Metrics"}},
		}, routepolicy.WithAdminPolicy("admin"), routepolicy.WithProblemResponses(http.StatusUnauthorized, http.StatusForbidden)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizations",
			Method:      http.MethodGet,
			Path:        "/organizations",
			Summary:     "List organizations",
			Security:    auth("organizations:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "Organization list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/OrganizationList"},
					},
				},
			},
		}, routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganization",
			Method:      http.MethodPost,
			Path:        "/organizations",
			Summary:     "Create organization",
			Parameters: []specs.Parameter{
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("organizations:write"),
			RequestBody: organizationCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: organizationResponse},
		}, routepolicy.WithTenantRequired("actor"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationMembers",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/members",
			Summary:     "List organization members",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security: auth("members:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "Membership list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/MembershipList"},
					},
				},
			},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganizationInvitation",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/invitations",
			Summary:     "Create organization invitation",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("invitations:write"),
			RequestBody: invitationCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: invitationCreatedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationAPIKeys",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/api-keys",
			Summary:     "List organization API keys",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security: auth("api-keys:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "API key list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/APIKeyList"},
					},
				},
			},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganizationAPIKey",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/api-keys",
			Summary:     "Create organization API key",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("api-keys:write"),
			RequestBody: apiKeyCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: apiKeyCreatedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "revokeOrganizationAPIKey",
			Method:      http.MethodDelete,
			Path:        "/organizations/{organization_id}/api-keys/{api_key_id}",
			Summary:     "Revoke organization API key",
			Parameters: []specs.Parameter{
				{Name: "api_key_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("api-keys:write"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Revoked"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listWebhookEvents",
			Method:      http.MethodGet,
			Path:        "/webhook-events",
			Summary:     "List webhook event types",
			Security:    auth("webhooks:read"),
			Responses:   map[int]specs.Response{http.StatusOK: webhookEventCatalogResponse},
		}, routepolicy.WithProblemResponses(http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationWebhookEndpoints",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/webhook-endpoints",
			Summary:     "List organization webhook endpoints",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("webhooks:read"),
			Responses: map[int]specs.Response{http.StatusOK: webhookEndpointListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createOrganizationWebhookEndpoint",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/webhook-endpoints",
			Summary:     "Create organization webhook endpoint",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("webhooks:write"),
			RequestBody: webhookEndpointCreateBody,
			Responses:   map[int]specs.Response{http.StatusCreated: webhookEndpointCreatedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationWebhookDeliveries",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/webhook-deliveries",
			Summary:     "List organization webhook deliveries",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("webhooks:read"),
			Responses: map[int]specs.Response{http.StatusOK: webhookDeliveryListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "replayOrganizationWebhookDelivery",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/webhook-deliveries/{delivery_id}/replay",
			Summary:     "Replay organization webhook delivery",
			Parameters: []specs.Parameter{
				{Name: "delivery_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("webhooks:write"),
			RequestBody: webhookReplayBody,
			Responses:   map[int]specs.Response{http.StatusAccepted: webhookDeliveryResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listOrganizationObjects",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/objects",
			Summary:     "List organization objects",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("objects:read"),
			Responses: map[int]specs.Response{http.StatusOK: objectListResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "putOrganizationObject",
			Method:      http.MethodPost,
			Path:        "/organizations/{organization_id}/objects",
			Summary:     "Put organization object",
			Parameters: []specs.Parameter{
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("objects:write"),
			RequestBody: objectPutBody,
			Responses:   map[int]specs.Response{http.StatusCreated: objectResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getOrganizationObject",
			Method:      http.MethodGet,
			Path:        "/organizations/{organization_id}/objects/{object_key}",
			Summary:     "Get organization object",
			Parameters: []specs.Parameter{
				{Name: "object_key", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("objects:read"),
			Responses: map[int]specs.Response{http.StatusOK: objectReadResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "deleteOrganizationObject",
			Method:      http.MethodDelete,
			Path:        "/organizations/{organization_id}/objects/{object_key}",
			Summary:     "Delete organization object",
			Parameters: []specs.Parameter{
				{Name: "object_key", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "organization_id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("objects:write"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Deleted"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "acceptInvitation",
			Method:      http.MethodPost,
			Path:        "/invitations/{id}/accept",
			Summary:     "Accept invitation",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("invitations:accept"),
			RequestBody: invitationAcceptBody,
			Responses:   map[int]specs.Response{http.StatusOK: membershipResponse},
		}, routepolicy.WithTenantRequired("invitation"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "getOperation",
			Method:      http.MethodGet,
			Path:        "/operations/{id}",
			Summary:     "Get operation",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("operations:read"),
			Responses: map[int]specs.Response{http.StatusOK: operationResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "listWidgets",
			Method:      http.MethodGet,
			Path:        "/widgets",
			Summary:     "List widgets",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "cursor", In: "query", Required: false, Schema: map[string]any{"type": "string"}},
				{Name: "limit", In: "query", Required: false, Schema: map[string]any{"type": "integer", "minimum": 1, "maximum": 100}},
			},
			Security: auth("widgets:read"),
			Responses: map[int]specs.Response{
				http.StatusOK: {
					Description: "Widget list",
					Content: map[string]specs.MediaType{
						"application/json": {SchemaRef: "#/components/schemas/WidgetList"},
					},
				},
			},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithProblemResponses(http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createWidget",
			Method:      http.MethodPost,
			Path:        "/widgets",
			Summary:     "Create widget",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("widgets:write"),
			RequestBody: jsonBody,
			Responses:   map[int]specs.Response{http.StatusCreated: widgetResponse, http.StatusOK: widgetResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "createWidgetImport",
			Method:      http.MethodPost,
			Path:        "/widgets/imports",
			Summary:     "Create widget import",
			Parameters: []specs.Parameter{
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("widgets:write"),
			RequestBody: widgetImportBody,
			Responses:   map[int]specs.Response{http.StatusAccepted: operationAcceptedResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "updateWidget",
			Method:      http.MethodPatch,
			Path:        "/widgets/{id}",
			Summary:     "Update widget",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "If-Match", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:    auth("widgets:write"),
			RequestBody: jsonBody,
			Responses:   map[int]specs.Response{http.StatusOK: widgetResponse},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		routepolicy.ApplyMetadata(specs.Operation{
			OperationID: "deleteWidget",
			Method:      http.MethodDelete,
			Path:        "/widgets/{id}",
			Summary:     "Delete widget",
			Parameters: []specs.Parameter{
				{Name: "id", In: "path", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "X-Tenant-ID", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
				{Name: "Idempotency-Key", In: "header", Required: true, Schema: map[string]any{"type": "string"}},
			},
			Security:  auth("widgets:write"),
			Responses: map[int]specs.Response{http.StatusNoContent: {Description: "Deleted"}},
		}, routepolicy.WithTenantRequired("header"), routepolicy.WithIdempotencyRequired(), routepolicy.WithRateLimit("write-standard"), routepolicy.WithProblemResponses(problemStatuses...)),
		// api-toolkit:openapi-operations
	}
	return operations
}

func normalizeJSON(data []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
