package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	baseURL     string
	httpClient  *http.Client
	apiKey      string
	bearerToken string
	headers     http.Header
}

type Option func(*Client)

func New(baseURL string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q", baseURL)
	}
	client := &Client{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
		headers:    http.Header{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	if client.httpClient == nil {
		return nil, errors.New("http client is required")
	}
	return client, nil
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

func WithAPIKey(apiKey string) Option {
	return func(client *Client) {
		client.apiKey = strings.TrimSpace(apiKey)
	}
}

func WithBearerToken(token string) Option {
	return func(client *Client) {
		client.bearerToken = strings.TrimSpace(token)
	}
}

func WithHeader(name, value string) Option {
	return func(client *Client) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if client.headers == nil {
			client.headers = http.Header{}
		}
		client.headers.Set(name, value)
	}
}

type RequestOption func(*requestOptions)

type requestOptions struct {
	pathParams map[string]string
	query      url.Values
	headers    http.Header
}

func PathParam(name, value string) RequestOption {
	return func(opts *requestOptions) {
		if opts.pathParams == nil {
			opts.pathParams = map[string]string{}
		}
		opts.pathParams[name] = value
	}
}

func QueryParam(name, value string) RequestOption {
	return func(opts *requestOptions) {
		if opts.query == nil {
			opts.query = url.Values{}
		}
		opts.query.Set(name, value)
	}
}

func Header(name, value string) RequestOption {
	return func(opts *requestOptions) {
		if opts.headers == nil {
			opts.headers = http.Header{}
		}
		opts.headers.Set(name, value)
	}
}

func formatParamValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []string:
		return strings.Join(typed, ",")
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(value)
	}
}

type Problem struct {
	Type     string         `json:"type,omitempty"`
	Title    string         `json:"title,omitempty"`
	Status   int            `json:"status,omitempty"`
	Detail   string         `json:"detail,omitempty"`
	Instance string         `json:"instance,omitempty"`
	Ext      map[string]any `json:"-"`
}

type Error struct {
	Response *http.Response
	Problem  *Problem
	Body     []byte
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	if err.Problem != nil && err.Problem.Title != "" {
		return err.Problem.Title
	}
	if err.Response != nil {
		return err.Response.Status
	}
	return "request failed"
}

func (c *Client) do(ctx context.Context, method, path string, body any, opts ...RequestOption) (*http.Response, error) {
	if c == nil {
		return nil, errors.New("client is nil")
	}
	requestOpts := requestOptions{
		pathParams: map[string]string{},
		query:      url.Values{},
		headers:    http.Header{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&requestOpts)
		}
	}
	expandedPath, err := expandPath(path, requestOpts.pathParams)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(c.baseURL, "/") + expandedPath
	if encodedQuery := requestOpts.query.Encode(); encodedQuery != "" {
		endpoint += "?" + encodedQuery
	}
	var reader io.Reader
	if body != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = &buf
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	copyHeaders(req.Header, c.headers)
	copyHeaders(req.Header, requestOpts.headers)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return resp, decodeError(resp)
	}
	return resp, nil
}

func expandPath(path string, params map[string]string) (string, error) {
	expanded := path
	for name, value := range params {
		expanded = strings.ReplaceAll(expanded, "{"+name+"}", url.PathEscape(value))
	}
	if strings.Contains(expanded, "{") || strings.Contains(expanded, "}") {
		return "", fmt.Errorf("missing path parameter for %s", path)
	}
	return expanded, nil
}

func decodeError(resp *http.Response) error {
	apiErr := &Error{Response: resp}
	if resp == nil || resp.Body == nil {
		return apiErr
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "application/problem+json") {
		return apiErr
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return apiErr
	}
	apiErr.Body = body
	var problem Problem
	if err := json.Unmarshal(body, &problem); err == nil {
		apiErr.Problem = &problem
	}
	return apiErr
}

func DecodeJSONResponse[T any](resp *http.Response) (*T, error) {
	if resp == nil || resp.Body == nil {
		return nil, errors.New("response body is required")
	}
	defer resp.Body.Close()
	var out T
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response body: %w", err)
	}
	return &out, nil
}

func copyHeaders(dst, src http.Header) {
	for name, values := range src {
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

type APIKey struct {
	CreatedAt      string   `json:"created_at"`
	ExpiresAt      *string  `json:"expires_at,omitempty"`
	ID             string   `json:"id"`
	LastUsedAt     *string  `json:"last_used_at,omitempty"`
	Name           string   `json:"name"`
	OrganizationID string   `json:"organization_id"`
	Prefix         string   `json:"prefix"`
	RevokedAt      *string  `json:"revoked_at,omitempty"`
	Scopes         []string `json:"scopes"`
}

type APIKeyCreateRequest struct {
	ExpiresAt *string  `json:"expires_at,omitempty"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
}

type APIKeyCreated struct {
	APIKey APIKey `json:"api_key"`
	Secret string `json:"secret"`
}

type APIKeyList struct {
	Items []APIKey `json:"items"`
}

type Invitation struct {
	AcceptedAt     *string `json:"accepted_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
	Email          string  `json:"email"`
	ExpiresAt      string  `json:"expires_at"`
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	Role           string  `json:"role"`
	TokenPrefix    string  `json:"token_prefix"`
}

type InvitationAcceptRequest struct {
	Token string `json:"token"`
}

type InvitationCreateRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

type InvitationCreated struct {
	Invitation Invitation `json:"invitation"`
	Token      string     `json:"token"`
}

type Membership struct {
	CreatedAt      string `json:"created_at"`
	OrganizationID string `json:"organization_id"`
	Role           string `json:"role"`
	UserID         string `json:"user_id"`
}

type MembershipList struct {
	Items []Membership `json:"items"`
}

type Object struct {
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
	Key         string `json:"key"`
	Size        int    `json:"size"`
	TenantID    string `json:"tenant_id"`
	UpdatedAt   string `json:"updated_at"`
}

type ObjectList struct {
	Items []Object `json:"items"`
}

type ObjectPutRequest struct {
	ContentBase64 string `json:"content_base64"`
	ContentType   string `json:"content_type"`
	Key           string `json:"key"`
}

type ObjectRead struct {
	ContentBase64 string `json:"content_base64"`
	Object        Object `json:"object"`
}

type OperationAccepted struct {
	ID       *string `json:"id,omitempty"`
	Location *string `json:"location,omitempty"`
	State    string  `json:"state"`
}

type Organization struct {
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at"`
}

type OrganizationCreateRequest struct {
	Name string `json:"name"`
}

type OrganizationList struct {
	Items []Organization `json:"items"`
}

type ValidationProblem struct {
	Code             *string          `json:"code,omitempty"`
	Detail           *string          `json:"detail,omitempty"`
	DocumentationURL *string          `json:"documentation_url,omitempty"`
	Errors           []map[string]any `json:"errors,omitempty"`
	Instance         *string          `json:"instance,omitempty"`
	LogLevel         *string          `json:"log_level,omitempty"`
	Retryable        *bool            `json:"retryable,omitempty"`
	Status           *int             `json:"status,omitempty"`
	Title            *string          `json:"title,omitempty"`
	Type             *string          `json:"type,omitempty"`
}

type WebhookDelivery struct {
	Attempt        int     `json:"attempt"`
	CreatedAt      string  `json:"created_at"`
	EndpointID     string  `json:"endpoint_id"`
	EventID        string  `json:"event_id"`
	EventType      string  `json:"event_type"`
	ID             string  `json:"id"`
	LastError      *string `json:"last_error,omitempty"`
	LastStatusCode *int    `json:"last_status_code,omitempty"`
	NextAt         string  `json:"next_at"`
	State          string  `json:"state"`
	TenantID       string  `json:"tenant_id"`
	UpdatedAt      string  `json:"updated_at"`
	URL            string  `json:"url"`
}

type WebhookDeliveryList struct {
	Items []WebhookDelivery `json:"items"`
}

type WebhookEndpoint struct {
	CreatedAt string   `json:"created_at"`
	Disabled  *bool    `json:"disabled,omitempty"`
	Events    []string `json:"events"`
	ID        string   `json:"id"`
	TenantID  string   `json:"tenant_id"`
	UpdatedAt *string  `json:"updated_at,omitempty"`
	URL       string   `json:"url"`
}

type WebhookEndpointCreateRequest struct {
	Events []string `json:"events"`
	URL    string   `json:"url"`
}

type WebhookEndpointCreated struct {
	Endpoint WebhookEndpoint `json:"endpoint"`
	Secret   string          `json:"secret"`
}

type WebhookEndpointList struct {
	Items []WebhookEndpoint `json:"items"`
}

type WebhookEventCatalog struct {
	EventTypes []string `json:"event_types"`
}

type WebhookReplayRequest struct {
}

type Widget struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
	Version  int64  `json:"version"`
}

type WidgetCreateRequest struct {
	Name string `json:"name"`
}

type WidgetImportItem struct {
	Name string `json:"name"`
}

type WidgetImportOperation struct {
	ID      string              `json:"id"`
	Problem *Problem            `json:"problem,omitempty"`
	Result  *WidgetImportResult `json:"result,omitempty"`
	State   string              `json:"state"`
}

type WidgetImportRequest struct {
	Items []WidgetImportItem `json:"items"`
}

type WidgetImportResult struct {
	Created   int      `json:"created"`
	WidgetIds []string `json:"widget_ids"`
}

type WidgetList struct {
	Items      []Widget `json:"items"`
	NextCursor *string  `json:"next_cursor,omitempty"`
}

// GetOpenAPI calls GET /docs/openapi.json and decodes the primary JSON response.
// GetOpenAPI calls GET /docs/openapi.json.
func (c *Client) GetOpenAPI(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.GetOpenAPIRaw(ctx, opts...)
}

// GetOpenAPIRaw calls GET /docs/openapi.json and returns the raw HTTP response.
func (c *Client) GetOpenAPIRaw(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.do(ctx, "GET", "/docs/openapi.json", nil, opts...)
}

// GetDetailedHealth calls GET /health/detailed and decodes the primary JSON response.
// GetDetailedHealth calls GET /health/detailed.
func (c *Client) GetDetailedHealth(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.GetDetailedHealthRaw(ctx, opts...)
}

// GetDetailedHealthRaw calls GET /health/detailed and returns the raw HTTP response.
func (c *Client) GetDetailedHealthRaw(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.do(ctx, "GET", "/health/detailed", nil, opts...)
}

type AcceptInvitationParams struct {
	IdempotencyKey string
}

func (params AcceptInvitationParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	return opts
}

// AcceptInvitation calls POST /invitations/{id}/accept and decodes the primary JSON response.
func (c *Client) AcceptInvitation(ctx context.Context, id string, params AcceptInvitationParams, body InvitationAcceptRequest, opts ...RequestOption) (*Membership, *http.Response, error) {
	resp, err := c.AcceptInvitationRaw(ctx, id, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[Membership](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// AcceptInvitationRaw calls POST /invitations/{id}/accept and returns the raw HTTP response.
func (c *Client) AcceptInvitationRaw(ctx context.Context, id string, params AcceptInvitationParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("id", formatParamValue(id)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "POST", "/invitations/{id}/accept", body, opts...)
}

// GetLiveness calls GET /livez and decodes the primary JSON response.
// GetLiveness calls GET /livez.
func (c *Client) GetLiveness(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.GetLivenessRaw(ctx, opts...)
}

// GetLivenessRaw calls GET /livez and returns the raw HTTP response.
func (c *Client) GetLivenessRaw(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.do(ctx, "GET", "/livez", nil, opts...)
}

// GetMetrics calls GET /metrics and decodes the primary JSON response.
// GetMetrics calls GET /metrics.
func (c *Client) GetMetrics(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.GetMetricsRaw(ctx, opts...)
}

// GetMetricsRaw calls GET /metrics and returns the raw HTTP response.
func (c *Client) GetMetricsRaw(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.do(ctx, "GET", "/metrics", nil, opts...)
}

type GetOperationParams struct {
	XTenantID string
}

func (params GetOperationParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// GetOperation calls GET /operations/{id} and decodes the primary JSON response.
func (c *Client) GetOperation(ctx context.Context, id string, params GetOperationParams, opts ...RequestOption) (*WidgetImportOperation, *http.Response, error) {
	resp, err := c.GetOperationRaw(ctx, id, params, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[WidgetImportOperation](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// GetOperationRaw calls GET /operations/{id} and returns the raw HTTP response.
func (c *Client) GetOperationRaw(ctx context.Context, id string, params GetOperationParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("id", formatParamValue(id)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "GET", "/operations/{id}", nil, opts...)
}

// ListOrganizations calls GET /organizations and decodes the primary JSON response.
func (c *Client) ListOrganizations(ctx context.Context, opts ...RequestOption) (*OrganizationList, *http.Response, error) {
	resp, err := c.ListOrganizationsRaw(ctx, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[OrganizationList](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// ListOrganizationsRaw calls GET /organizations and returns the raw HTTP response.
func (c *Client) ListOrganizationsRaw(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.do(ctx, "GET", "/organizations", nil, opts...)
}

type CreateOrganizationParams struct {
	IdempotencyKey string
}

func (params CreateOrganizationParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	return opts
}

// CreateOrganization calls POST /organizations and decodes the primary JSON response.
func (c *Client) CreateOrganization(ctx context.Context, params CreateOrganizationParams, body OrganizationCreateRequest, opts ...RequestOption) (*Organization, *http.Response, error) {
	resp, err := c.CreateOrganizationRaw(ctx, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[Organization](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// CreateOrganizationRaw calls POST /organizations and returns the raw HTTP response.
func (c *Client) CreateOrganizationRaw(ctx context.Context, params CreateOrganizationParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "POST", "/organizations", body, opts...)
}

type ListOrganizationAPIKeysParams struct {
	XTenantID string
}

func (params ListOrganizationAPIKeysParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// ListOrganizationAPIKeys calls GET /organizations/{organization_id}/api-keys and decodes the primary JSON response.
func (c *Client) ListOrganizationAPIKeys(ctx context.Context, organizationID string, params ListOrganizationAPIKeysParams, opts ...RequestOption) (*APIKeyList, *http.Response, error) {
	resp, err := c.ListOrganizationAPIKeysRaw(ctx, organizationID, params, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[APIKeyList](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// ListOrganizationAPIKeysRaw calls GET /organizations/{organization_id}/api-keys and returns the raw HTTP response.
func (c *Client) ListOrganizationAPIKeysRaw(ctx context.Context, organizationID string, params ListOrganizationAPIKeysParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "GET", "/organizations/{organization_id}/api-keys", nil, opts...)
}

type CreateOrganizationAPIKeyParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params CreateOrganizationAPIKeyParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// CreateOrganizationAPIKey calls POST /organizations/{organization_id}/api-keys and decodes the primary JSON response.
func (c *Client) CreateOrganizationAPIKey(ctx context.Context, organizationID string, params CreateOrganizationAPIKeyParams, body APIKeyCreateRequest, opts ...RequestOption) (*APIKeyCreated, *http.Response, error) {
	resp, err := c.CreateOrganizationAPIKeyRaw(ctx, organizationID, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[APIKeyCreated](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// CreateOrganizationAPIKeyRaw calls POST /organizations/{organization_id}/api-keys and returns the raw HTTP response.
func (c *Client) CreateOrganizationAPIKeyRaw(ctx context.Context, organizationID string, params CreateOrganizationAPIKeyParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "POST", "/organizations/{organization_id}/api-keys", body, opts...)
}

type RevokeOrganizationAPIKeyParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params RevokeOrganizationAPIKeyParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// RevokeOrganizationAPIKey calls DELETE /organizations/{organization_id}/api-keys/{api_key_id} and decodes the primary JSON response.
// RevokeOrganizationAPIKey calls DELETE /organizations/{organization_id}/api-keys/{api_key_id}.
func (c *Client) RevokeOrganizationAPIKey(ctx context.Context, apiKeyID string, organizationID string, params RevokeOrganizationAPIKeyParams, opts ...RequestOption) (*http.Response, error) {
	return c.RevokeOrganizationAPIKeyRaw(ctx, apiKeyID, organizationID, params, opts...)
}

// RevokeOrganizationAPIKeyRaw calls DELETE /organizations/{organization_id}/api-keys/{api_key_id} and returns the raw HTTP response.
func (c *Client) RevokeOrganizationAPIKeyRaw(ctx context.Context, apiKeyID string, organizationID string, params RevokeOrganizationAPIKeyParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("api_key_id", formatParamValue(apiKeyID)),
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "DELETE", "/organizations/{organization_id}/api-keys/{api_key_id}", nil, opts...)
}

type CreateOrganizationInvitationParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params CreateOrganizationInvitationParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// CreateOrganizationInvitation calls POST /organizations/{organization_id}/invitations and decodes the primary JSON response.
func (c *Client) CreateOrganizationInvitation(ctx context.Context, organizationID string, params CreateOrganizationInvitationParams, body InvitationCreateRequest, opts ...RequestOption) (*InvitationCreated, *http.Response, error) {
	resp, err := c.CreateOrganizationInvitationRaw(ctx, organizationID, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[InvitationCreated](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// CreateOrganizationInvitationRaw calls POST /organizations/{organization_id}/invitations and returns the raw HTTP response.
func (c *Client) CreateOrganizationInvitationRaw(ctx context.Context, organizationID string, params CreateOrganizationInvitationParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "POST", "/organizations/{organization_id}/invitations", body, opts...)
}

type ListOrganizationMembersParams struct {
	XTenantID string
}

func (params ListOrganizationMembersParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// ListOrganizationMembers calls GET /organizations/{organization_id}/members and decodes the primary JSON response.
func (c *Client) ListOrganizationMembers(ctx context.Context, organizationID string, params ListOrganizationMembersParams, opts ...RequestOption) (*MembershipList, *http.Response, error) {
	resp, err := c.ListOrganizationMembersRaw(ctx, organizationID, params, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[MembershipList](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// ListOrganizationMembersRaw calls GET /organizations/{organization_id}/members and returns the raw HTTP response.
func (c *Client) ListOrganizationMembersRaw(ctx context.Context, organizationID string, params ListOrganizationMembersParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "GET", "/organizations/{organization_id}/members", nil, opts...)
}

type ListOrganizationObjectsParams struct {
	XTenantID string
}

func (params ListOrganizationObjectsParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// ListOrganizationObjects calls GET /organizations/{organization_id}/objects and decodes the primary JSON response.
func (c *Client) ListOrganizationObjects(ctx context.Context, organizationID string, params ListOrganizationObjectsParams, opts ...RequestOption) (*ObjectList, *http.Response, error) {
	resp, err := c.ListOrganizationObjectsRaw(ctx, organizationID, params, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[ObjectList](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// ListOrganizationObjectsRaw calls GET /organizations/{organization_id}/objects and returns the raw HTTP response.
func (c *Client) ListOrganizationObjectsRaw(ctx context.Context, organizationID string, params ListOrganizationObjectsParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "GET", "/organizations/{organization_id}/objects", nil, opts...)
}

type PutOrganizationObjectParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params PutOrganizationObjectParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// PutOrganizationObject calls POST /organizations/{organization_id}/objects and decodes the primary JSON response.
func (c *Client) PutOrganizationObject(ctx context.Context, organizationID string, params PutOrganizationObjectParams, body ObjectPutRequest, opts ...RequestOption) (*Object, *http.Response, error) {
	resp, err := c.PutOrganizationObjectRaw(ctx, organizationID, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[Object](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// PutOrganizationObjectRaw calls POST /organizations/{organization_id}/objects and returns the raw HTTP response.
func (c *Client) PutOrganizationObjectRaw(ctx context.Context, organizationID string, params PutOrganizationObjectParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "POST", "/organizations/{organization_id}/objects", body, opts...)
}

type DeleteOrganizationObjectParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params DeleteOrganizationObjectParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// DeleteOrganizationObject calls DELETE /organizations/{organization_id}/objects/{object_key} and decodes the primary JSON response.
// DeleteOrganizationObject calls DELETE /organizations/{organization_id}/objects/{object_key}.
func (c *Client) DeleteOrganizationObject(ctx context.Context, objectKey string, organizationID string, params DeleteOrganizationObjectParams, opts ...RequestOption) (*http.Response, error) {
	return c.DeleteOrganizationObjectRaw(ctx, objectKey, organizationID, params, opts...)
}

// DeleteOrganizationObjectRaw calls DELETE /organizations/{organization_id}/objects/{object_key} and returns the raw HTTP response.
func (c *Client) DeleteOrganizationObjectRaw(ctx context.Context, objectKey string, organizationID string, params DeleteOrganizationObjectParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("object_key", formatParamValue(objectKey)),
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "DELETE", "/organizations/{organization_id}/objects/{object_key}", nil, opts...)
}

type GetOrganizationObjectParams struct {
	XTenantID string
}

func (params GetOrganizationObjectParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// GetOrganizationObject calls GET /organizations/{organization_id}/objects/{object_key} and decodes the primary JSON response.
func (c *Client) GetOrganizationObject(ctx context.Context, objectKey string, organizationID string, params GetOrganizationObjectParams, opts ...RequestOption) (*ObjectRead, *http.Response, error) {
	resp, err := c.GetOrganizationObjectRaw(ctx, objectKey, organizationID, params, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[ObjectRead](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// GetOrganizationObjectRaw calls GET /organizations/{organization_id}/objects/{object_key} and returns the raw HTTP response.
func (c *Client) GetOrganizationObjectRaw(ctx context.Context, objectKey string, organizationID string, params GetOrganizationObjectParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("object_key", formatParamValue(objectKey)),
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "GET", "/organizations/{organization_id}/objects/{object_key}", nil, opts...)
}

type ListOrganizationWebhookDeliveriesParams struct {
	XTenantID string
}

func (params ListOrganizationWebhookDeliveriesParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// ListOrganizationWebhookDeliveries calls GET /organizations/{organization_id}/webhook-deliveries and decodes the primary JSON response.
func (c *Client) ListOrganizationWebhookDeliveries(ctx context.Context, organizationID string, params ListOrganizationWebhookDeliveriesParams, opts ...RequestOption) (*WebhookDeliveryList, *http.Response, error) {
	resp, err := c.ListOrganizationWebhookDeliveriesRaw(ctx, organizationID, params, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[WebhookDeliveryList](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// ListOrganizationWebhookDeliveriesRaw calls GET /organizations/{organization_id}/webhook-deliveries and returns the raw HTTP response.
func (c *Client) ListOrganizationWebhookDeliveriesRaw(ctx context.Context, organizationID string, params ListOrganizationWebhookDeliveriesParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "GET", "/organizations/{organization_id}/webhook-deliveries", nil, opts...)
}

type ReplayOrganizationWebhookDeliveryParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params ReplayOrganizationWebhookDeliveryParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// ReplayOrganizationWebhookDelivery calls POST /organizations/{organization_id}/webhook-deliveries/{delivery_id}/replay and decodes the primary JSON response.
func (c *Client) ReplayOrganizationWebhookDelivery(ctx context.Context, deliveryID string, organizationID string, params ReplayOrganizationWebhookDeliveryParams, body WebhookReplayRequest, opts ...RequestOption) (*WebhookDelivery, *http.Response, error) {
	resp, err := c.ReplayOrganizationWebhookDeliveryRaw(ctx, deliveryID, organizationID, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[WebhookDelivery](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// ReplayOrganizationWebhookDeliveryRaw calls POST /organizations/{organization_id}/webhook-deliveries/{delivery_id}/replay and returns the raw HTTP response.
func (c *Client) ReplayOrganizationWebhookDeliveryRaw(ctx context.Context, deliveryID string, organizationID string, params ReplayOrganizationWebhookDeliveryParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("delivery_id", formatParamValue(deliveryID)),
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "POST", "/organizations/{organization_id}/webhook-deliveries/{delivery_id}/replay", body, opts...)
}

type ListOrganizationWebhookEndpointsParams struct {
	XTenantID string
}

func (params ListOrganizationWebhookEndpointsParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// ListOrganizationWebhookEndpoints calls GET /organizations/{organization_id}/webhook-endpoints and decodes the primary JSON response.
func (c *Client) ListOrganizationWebhookEndpoints(ctx context.Context, organizationID string, params ListOrganizationWebhookEndpointsParams, opts ...RequestOption) (*WebhookEndpointList, *http.Response, error) {
	resp, err := c.ListOrganizationWebhookEndpointsRaw(ctx, organizationID, params, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[WebhookEndpointList](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// ListOrganizationWebhookEndpointsRaw calls GET /organizations/{organization_id}/webhook-endpoints and returns the raw HTTP response.
func (c *Client) ListOrganizationWebhookEndpointsRaw(ctx context.Context, organizationID string, params ListOrganizationWebhookEndpointsParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "GET", "/organizations/{organization_id}/webhook-endpoints", nil, opts...)
}

type CreateOrganizationWebhookEndpointParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params CreateOrganizationWebhookEndpointParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// CreateOrganizationWebhookEndpoint calls POST /organizations/{organization_id}/webhook-endpoints and decodes the primary JSON response.
func (c *Client) CreateOrganizationWebhookEndpoint(ctx context.Context, organizationID string, params CreateOrganizationWebhookEndpointParams, body WebhookEndpointCreateRequest, opts ...RequestOption) (*WebhookEndpointCreated, *http.Response, error) {
	resp, err := c.CreateOrganizationWebhookEndpointRaw(ctx, organizationID, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[WebhookEndpointCreated](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// CreateOrganizationWebhookEndpointRaw calls POST /organizations/{organization_id}/webhook-endpoints and returns the raw HTTP response.
func (c *Client) CreateOrganizationWebhookEndpointRaw(ctx context.Context, organizationID string, params CreateOrganizationWebhookEndpointParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("organization_id", formatParamValue(organizationID)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "POST", "/organizations/{organization_id}/webhook-endpoints", body, opts...)
}

// GetReadiness calls GET /readyz and decodes the primary JSON response.
// GetReadiness calls GET /readyz.
func (c *Client) GetReadiness(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.GetReadinessRaw(ctx, opts...)
}

// GetReadinessRaw calls GET /readyz and returns the raw HTTP response.
func (c *Client) GetReadinessRaw(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.do(ctx, "GET", "/readyz", nil, opts...)
}

// ListWebhookEvents calls GET /webhook-events and decodes the primary JSON response.
func (c *Client) ListWebhookEvents(ctx context.Context, opts ...RequestOption) (*WebhookEventCatalog, *http.Response, error) {
	resp, err := c.ListWebhookEventsRaw(ctx, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[WebhookEventCatalog](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// ListWebhookEventsRaw calls GET /webhook-events and returns the raw HTTP response.
func (c *Client) ListWebhookEventsRaw(ctx context.Context, opts ...RequestOption) (*http.Response, error) {
	return c.do(ctx, "GET", "/webhook-events", nil, opts...)
}

type ListWidgetsParams struct {
	XTenantID string
	Cursor    *string
	Limit     *int
}

func (params ListWidgetsParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	if params.Cursor != nil {
		opts = append(opts, QueryParam("cursor", formatParamValue(*params.Cursor)))
	}
	if params.Limit != nil {
		opts = append(opts, QueryParam("limit", formatParamValue(*params.Limit)))
	}
	return opts
}

// ListWidgets calls GET /widgets and decodes the primary JSON response.
func (c *Client) ListWidgets(ctx context.Context, params ListWidgetsParams, opts ...RequestOption) (*WidgetList, *http.Response, error) {
	resp, err := c.ListWidgetsRaw(ctx, params, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[WidgetList](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// ListWidgetsRaw calls GET /widgets and returns the raw HTTP response.
func (c *Client) ListWidgetsRaw(ctx context.Context, params ListWidgetsParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "GET", "/widgets", nil, opts...)
}

type CreateWidgetParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params CreateWidgetParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// CreateWidget calls POST /widgets and decodes the primary JSON response.
func (c *Client) CreateWidget(ctx context.Context, params CreateWidgetParams, body WidgetCreateRequest, opts ...RequestOption) (*Widget, *http.Response, error) {
	resp, err := c.CreateWidgetRaw(ctx, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[Widget](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// CreateWidgetRaw calls POST /widgets and returns the raw HTTP response.
func (c *Client) CreateWidgetRaw(ctx context.Context, params CreateWidgetParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "POST", "/widgets", body, opts...)
}

type CreateWidgetImportParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params CreateWidgetImportParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// CreateWidgetImport calls POST /widgets/imports and decodes the primary JSON response.
func (c *Client) CreateWidgetImport(ctx context.Context, params CreateWidgetImportParams, body WidgetImportRequest, opts ...RequestOption) (*OperationAccepted, *http.Response, error) {
	resp, err := c.CreateWidgetImportRaw(ctx, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[OperationAccepted](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// CreateWidgetImportRaw calls POST /widgets/imports and returns the raw HTTP response.
func (c *Client) CreateWidgetImportRaw(ctx context.Context, params CreateWidgetImportParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "POST", "/widgets/imports", body, opts...)
}

type DeleteWidgetParams struct {
	IdempotencyKey string
	XTenantID      string
}

func (params DeleteWidgetParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// DeleteWidget calls DELETE /widgets/{id} and decodes the primary JSON response.
// DeleteWidget calls DELETE /widgets/{id}.
func (c *Client) DeleteWidget(ctx context.Context, id string, params DeleteWidgetParams, opts ...RequestOption) (*http.Response, error) {
	return c.DeleteWidgetRaw(ctx, id, params, opts...)
}

// DeleteWidgetRaw calls DELETE /widgets/{id} and returns the raw HTTP response.
func (c *Client) DeleteWidgetRaw(ctx context.Context, id string, params DeleteWidgetParams, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("id", formatParamValue(id)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "DELETE", "/widgets/{id}", nil, opts...)
}

type UpdateWidgetParams struct {
	IdempotencyKey string
	IfMatch        string
	XTenantID      string
}

func (params UpdateWidgetParams) requestOptions() []RequestOption {
	var opts []RequestOption
	opts = append(opts, Header("Idempotency-Key", formatParamValue(params.IdempotencyKey)))
	opts = append(opts, Header("If-Match", formatParamValue(params.IfMatch)))
	opts = append(opts, Header("X-Tenant-ID", formatParamValue(params.XTenantID)))
	return opts
}

// UpdateWidget calls PATCH /widgets/{id} and decodes the primary JSON response.
func (c *Client) UpdateWidget(ctx context.Context, id string, params UpdateWidgetParams, body WidgetCreateRequest, opts ...RequestOption) (*Widget, *http.Response, error) {
	resp, err := c.UpdateWidgetRaw(ctx, id, params, body, opts...)
	if err != nil {
		return nil, resp, err
	}
	decoded, err := DecodeJSONResponse[Widget](resp)
	if err != nil {
		return nil, resp, err
	}
	return decoded, resp, nil
}

// UpdateWidgetRaw calls PATCH /widgets/{id} and returns the raw HTTP response.
func (c *Client) UpdateWidgetRaw(ctx context.Context, id string, params UpdateWidgetParams, body any, opts ...RequestOption) (*http.Response, error) {
	requestOpts := []RequestOption{
		PathParam("id", formatParamValue(id)),
	}
	requestOpts = append(requestOpts, params.requestOptions()...)
	opts = append(requestOpts, opts...)
	return c.do(ctx, "PATCH", "/widgets/{id}", body, opts...)
}
