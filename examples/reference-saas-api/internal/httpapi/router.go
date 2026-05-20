package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/idempotency"
	"github.com/aatuh/api-toolkit/contrib/v3/audit"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics"
	openapimw "github.com/aatuh/api-toolkit/contrib/v3/middleware/openapi"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/aatuh/api-toolkit/v3/endpoints/health"
	corepprof "github.com/aatuh/api-toolkit/v3/endpoints/pprof"
	"github.com/aatuh/api-toolkit/v3/httpx"
	idempotencymw "github.com/aatuh/api-toolkit/v3/middleware/idempotency"
	ratelimitmw "github.com/aatuh/api-toolkit/v3/middleware/ratelimit"
	apitkops "github.com/aatuh/api-toolkit/v3/operations"
	"github.com/aatuh/api-toolkit/v3/ports"

	"example.com/reference-saas-api/internal/app"
	"example.com/reference-saas-api/internal/domain"
)

type Config struct {
	Addr                      string
	AdminAddr                 string
	APIKey                    string
	AdminKey                  string
	DatabaseURL               string
	RedisAddr                 string
	CacheStore                string
	RateLimitStore            string
	RateLimitKeyPrefix        string
	IdempotencyStore          string
	IdempotencyKeyPrefix      string
	APIKeyPepper              string
	WebhookSecretKey          string
	ObjectStore               string
	S3Endpoint                string
	S3Region                  string
	S3Bucket                  string
	S3AccessKeyID             string
	S3SecretAccessKey         string
	OpenAPIRequestValidation  bool
	OpenAPIResponseValidation bool
	AsyncWorkerEnabled        bool
}

func ConfigFromEnv() (Config, error) {
	cacheStore := envDefault("CACHE_STORE", "memory")
	if strings.EqualFold(os.Getenv("ENV"), "production") && strings.TrimSpace(os.Getenv("CACHE_STORE")) == "" {
		cacheStore = "redis"
	}
	rateLimitStore := envDefault("RATE_LIMIT_STORE", "memory")
	if strings.EqualFold(os.Getenv("ENV"), "production") && strings.TrimSpace(os.Getenv("RATE_LIMIT_STORE")) == "" {
		rateLimitStore = "redis"
	}
	idempotencyStore := envDefault("IDEMPOTENCY_STORE", "memory")
	if strings.EqualFold(os.Getenv("ENV"), "production") && strings.TrimSpace(os.Getenv("IDEMPOTENCY_STORE")) == "" {
		idempotencyStore = "redis"
	}
	cfg := Config{
		Addr:                      envDefault("API_ADDR", ":8080"),
		AdminAddr:                 strings.TrimSpace(os.Getenv("ADMIN_ADDR")),
		APIKey:                    envDefault("API_KEY", "local-dev-key"),
		AdminKey:                  envDefault("ADMIN_KEY", "local-admin-key"),
		DatabaseURL:               strings.TrimSpace(os.Getenv("DATABASE_URL")),
		RedisAddr:                 strings.TrimSpace(os.Getenv("REDIS_ADDR")),
		CacheStore:                strings.ToLower(strings.TrimSpace(cacheStore)),
		RateLimitStore:            strings.ToLower(strings.TrimSpace(rateLimitStore)),
		RateLimitKeyPrefix:        envDefault("RATE_LIMIT_KEY_PREFIX", "ratelimit:"),
		IdempotencyStore:          strings.ToLower(strings.TrimSpace(idempotencyStore)),
		IdempotencyKeyPrefix:      envDefault("IDEMPOTENCY_KEY_PREFIX", "idempotency:"),
		APIKeyPepper:              strings.TrimSpace(os.Getenv("API_KEY_PEPPER")),
		WebhookSecretKey:          strings.TrimSpace(os.Getenv("WEBHOOK_SECRET_KEY")),
		ObjectStore:               strings.ToLower(envDefault("OBJECT_STORE", "memory")),
		S3Endpoint:                strings.TrimSpace(os.Getenv("S3_ENDPOINT")),
		S3Region:                  envDefault("S3_REGION", "us-east-1"),
		S3Bucket:                  strings.TrimSpace(os.Getenv("S3_BUCKET")),
		S3AccessKeyID:             strings.TrimSpace(os.Getenv("S3_ACCESS_KEY_ID")),
		S3SecretAccessKey:         strings.TrimSpace(os.Getenv("S3_SECRET_ACCESS_KEY")),
		OpenAPIRequestValidation:  envBoolDefault("OPENAPI_REQUEST_VALIDATION", true),
		OpenAPIResponseValidation: envBoolDefault("OPENAPI_RESPONSE_VALIDATION", defaultOpenAPIResponseValidation()),
		AsyncWorkerEnabled:        envBoolDefault("ASYNC_WORKER_ENABLED", true),
	}
	if cfg.CacheStore != "memory" && cfg.CacheStore != "redis" {
		return Config{}, errors.New("CACHE_STORE must be memory or redis")
	}
	if cfg.RateLimitStore != "memory" && cfg.RateLimitStore != "redis" {
		return Config{}, errors.New("RATE_LIMIT_STORE must be memory or redis")
	}
	if cfg.IdempotencyStore != "memory" && cfg.IdempotencyStore != "redis" {
		return Config{}, errors.New("IDEMPOTENCY_STORE must be memory or redis")
	}
	if cfg.ObjectStore != "memory" && cfg.ObjectStore != "s3" {
		return Config{}, errors.New("OBJECT_STORE must be memory or s3")
	}
	if cfg.ObjectStore == "s3" && (cfg.S3Endpoint == "" || cfg.S3Bucket == "" || cfg.S3AccessKeyID == "" || cfg.S3SecretAccessKey == "") {
		return Config{}, errors.New("S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY_ID, and S3_SECRET_ACCESS_KEY are required when OBJECT_STORE=s3")
	}
	if strings.EqualFold(os.Getenv("ENV"), "production") {
		var missing []string
		if cfg.DatabaseURL == "" {
			missing = append(missing, "DATABASE_URL")
		}
		if cfg.RedisAddr == "" {
			missing = append(missing, "REDIS_ADDR")
		}
		if cfg.APIKeyPepper == "" {
			missing = append(missing, "API_KEY_PEPPER")
		}
		if cfg.WebhookSecretKey == "" {
			missing = append(missing, "WEBHOOK_SECRET_KEY")
		}
		if cfg.CacheStore != "redis" {
			missing = append(missing, "CACHE_STORE=redis")
		}
		if cfg.RateLimitStore != "redis" {
			missing = append(missing, "RATE_LIMIT_STORE=redis")
		}
		if cfg.IdempotencyStore != "redis" {
			missing = append(missing, "IDEMPOTENCY_STORE=redis")
		}
		if cfg.APIKey == "" || cfg.APIKey == "local-dev-key" {
			missing = append(missing, "API_KEY")
		}

		if cfg.AdminKey == "" || cfg.AdminKey == "local-admin-key" {
			missing = append(missing, "ADMIN_KEY")
		}
		if len(missing) > 0 {
			return Config{}, errors.New("production configuration missing: " + strings.Join(missing, ", "))
		}
	}
	return cfg, nil
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBoolDefault(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	switch value {
	case "":
		return fallback
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func defaultOpenAPIResponseValidation() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ENV"))) {
	case "development", "dev", "local", "test":
		return true
	default:
		return false
	}
}

type RouterConfig struct {
	Widgets  *app.WidgetService
	Tenancy  *app.TenancyService
	APIKeys  *app.APIKeyService
	Async    *app.AsyncService
	Audit    *app.AuditService
	Webhooks *app.WebhookService
	Objects  *app.ObjectService
	Cache    *app.CacheService

	// api-toolkit:router-config-fields
	Metrics           *metricsmw.Middleware
	MetricsHandler    http.Handler
	OpenAPIValidation *openapimw.Middleware
	RateLimit         *ratelimitmw.Middleware
	Idempotency       *idempotencymw.Middleware
	Readiness         HealthChecker
	APIKey            string
	AdminKey          string
}

type HealthChecker interface {
	Check(context.Context) error
}

type HealthCheckFunc func(context.Context) error

func (f HealthCheckFunc) Check(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

func CombineHealthChecks(checkers ...HealthChecker) HealthChecker {
	return HealthCheckFunc(func(ctx context.Context) error {
		for _, checker := range checkers {
			if checker == nil {
				continue
			}
			if err := checker.Check(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

type apiKeyPrincipal struct {
	Key domain.APIKey
}

type apiKeyPrincipalContextKey struct{}

func withAPIKeyPrincipal(ctx context.Context, key domain.APIKey) context.Context {
	return context.WithValue(ctx, apiKeyPrincipalContextKey{}, apiKeyPrincipal{Key: key})
}

func apiKeyPrincipalFromContext(ctx context.Context) (apiKeyPrincipal, bool) {
	if ctx == nil {
		return apiKeyPrincipal{}, false
	}
	principal, ok := ctx.Value(apiKeyPrincipalContextKey{}).(apiKeyPrincipal)
	if !ok || strings.TrimSpace(principal.Key.ID) == "" {
		return apiKeyPrincipal{}, false
	}
	return principal, true
}

func (p apiKeyPrincipal) ActorID() string {
	return strings.TrimSpace(p.Key.ID)
}

func (p apiKeyPrincipal) TenantID() string {
	return strings.TrimSpace(p.Key.OrganizationID)
}

func (p apiKeyPrincipal) HasScope(required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return true
	}
	for _, scope := range p.Key.Scopes {
		scope = strings.TrimSpace(scope)
		if scope == "*" || strings.EqualFold(scope, required) {
			return true
		}
	}
	return false
}

func NewRouter(cfg RouterConfig) http.Handler {
	cfg = cfg.withDefaults()
	router := &serveMuxRouter{mux: http.NewServeMux()}
	router.Get("/livez", handleLive)
	router.Get("/readyz", cfg.handleReady)
	if err := RegisterRoutes(router, cfg); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "router registration failed"})
		})
	}
	return cfg.metrics(router)
}

func RegisterRoutes(r ports.HTTPRouter, cfg RouterConfig) error {
	if r == nil {
		return errors.New("router is required")
	}
	cfg = cfg.withDefaults()
	if cfg.OpenAPIValidation != nil {
		r.Use(cfg.OpenAPIValidation.Middleware())
	}
	r.Get("/docs/openapi.json", handleOpenAPI)
	r.Get("/organizations", cfg.protect("organizations:read", http.HandlerFunc(cfg.handleListOrganizations)).ServeHTTP)
	r.Post("/organizations", cfg.protect("organizations:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateOrganization))).ServeHTTP)
	r.Get("/organizations/{organization_id}/members", cfg.protect("members:read", http.HandlerFunc(cfg.handleListMembers)).ServeHTTP)
	r.Post("/organizations/{organization_id}/invitations", cfg.protect("invitations:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateInvitation))).ServeHTTP)
	r.Get("/organizations/{organization_id}/api-keys", cfg.protect("api-keys:read", http.HandlerFunc(cfg.handleListAPIKeys)).ServeHTTP)
	r.Post("/organizations/{organization_id}/api-keys", cfg.protect("api-keys:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateAPIKey))).ServeHTTP)
	r.Delete("/organizations/{organization_id}/api-keys/{api_key_id}", cfg.protect("api-keys:write", cfg.idempotent(http.HandlerFunc(cfg.handleRevokeAPIKey))).ServeHTTP)
	r.Get("/webhook-events", cfg.protect("webhooks:read", http.HandlerFunc(cfg.handleListWebhookEvents)).ServeHTTP)
	r.Get("/organizations/{organization_id}/webhook-endpoints", cfg.protect("webhooks:read", http.HandlerFunc(cfg.handleListWebhookEndpoints)).ServeHTTP)
	r.Post("/organizations/{organization_id}/webhook-endpoints", cfg.protect("webhooks:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateWebhookEndpoint))).ServeHTTP)
	r.Get("/organizations/{organization_id}/webhook-deliveries", cfg.protect("webhooks:read", http.HandlerFunc(cfg.handleListWebhookDeliveries)).ServeHTTP)
	r.Post("/organizations/{organization_id}/webhook-deliveries/{delivery_id}/replay", cfg.protect("webhooks:write", cfg.idempotent(http.HandlerFunc(cfg.handleReplayWebhookDelivery))).ServeHTTP)
	r.Get("/organizations/{organization_id}/objects", cfg.protect("objects:read", http.HandlerFunc(cfg.handleListObjects)).ServeHTTP)
	r.Post("/organizations/{organization_id}/objects", cfg.protect("objects:write", cfg.idempotent(http.HandlerFunc(cfg.handlePutObject))).ServeHTTP)
	r.Get("/organizations/{organization_id}/objects/{object_key}", cfg.protect("objects:read", http.HandlerFunc(cfg.handleGetObject)).ServeHTTP)
	r.Delete("/organizations/{organization_id}/objects/{object_key}", cfg.protect("objects:write", cfg.idempotent(http.HandlerFunc(cfg.handleDeleteObject))).ServeHTTP)

	r.Post("/invitations/{id}/accept", cfg.protect("invitations:accept", cfg.idempotent(http.HandlerFunc(cfg.handleAcceptInvitation))).ServeHTTP)
	r.Get("/operations/{id}", cfg.protect("operations:read", http.HandlerFunc(cfg.handleGetOperation)).ServeHTTP)
	r.Get("/widgets", cfg.protect("", http.HandlerFunc(cfg.handleListWidgets)).ServeHTTP)
	r.Post("/widgets", cfg.protect("widgets:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateWidget))).ServeHTTP)
	r.Post("/widgets/imports", cfg.protect("widgets:write", cfg.idempotent(http.HandlerFunc(cfg.handleCreateWidgetImport))).ServeHTTP)
	registerPatch(r, "/widgets/{id}", cfg.protect("widgets:write", cfg.idempotent(http.HandlerFunc(cfg.handleUpdateWidget))).ServeHTTP)
	r.Delete("/widgets/{id}", cfg.protect("widgets:write", cfg.idempotent(http.HandlerFunc(cfg.handleDeleteWidget))).ServeHTTP)
	// api-toolkit:router-register-routes
	return nil
}

func NewAdminRouter(cfg RouterConfig) http.Handler {
	cfg = cfg.withDefaults()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/detailed", cfg.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		if err := cfg.checkReady(r); err != nil {
			httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.Problem{Title: http.StatusText(http.StatusServiceUnavailable), Detail: "service is not ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))
	mux.Handle("GET /metrics", http.HandlerFunc(cfg.requireAdmin(func(w http.ResponseWriter, r *http.Request) {
		cfg.MetricsHandler.ServeHTTP(w, r)
	})))
	_ = corepprof.RegisterAdminRoutes(serveMuxGetRouter{mux: mux}, cfg.requireAdminHandler)
	// api-toolkit:admin-router-register-routes
	return mux
}

type serveMuxGetRouter struct {
	mux *http.ServeMux
}

func (r serveMuxGetRouter) Get(pattern string, h http.HandlerFunc) {
	if r.mux == nil {
		return
	}
	r.mux.HandleFunc("GET "+pattern, h)
}

type serveMuxRouter struct {
	mux         *http.ServeMux
	middlewares []func(http.Handler) http.Handler
}

func (r *serveMuxRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r == nil || r.mux == nil {
		http.NotFound(w, req)
		return
	}
	var handler http.Handler = r.mux
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		if r.middlewares[i] != nil {
			handler = r.middlewares[i](handler)
		}
	}
	handler.ServeHTTP(w, req)
}

func (r *serveMuxRouter) Use(middlewares ...func(http.Handler) http.Handler) {
	r.middlewares = append(r.middlewares, middlewares...)
}

func (r *serveMuxRouter) Get(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodGet, pattern, h)
}

func (r *serveMuxRouter) Post(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodPost, pattern, h)
}

func (r *serveMuxRouter) Put(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodPut, pattern, h)
}

func (r *serveMuxRouter) Patch(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodPatch, pattern, h)
}

func (r *serveMuxRouter) Delete(pattern string, h http.HandlerFunc) {
	r.handle(http.MethodDelete, pattern, h)
}

func (r *serveMuxRouter) Mount(pattern string, h http.Handler) {
	if r == nil || r.mux == nil || h == nil {
		return
	}
	r.mux.Handle(pattern, h)
}

func (r *serveMuxRouter) handle(method, pattern string, h http.HandlerFunc) {
	if r == nil || r.mux == nil || h == nil {
		return
	}
	r.mux.HandleFunc(method+" "+pattern, h)
}

type patchRouter interface {
	Patch(pattern string, h http.HandlerFunc)
}

func registerPatch(r ports.HTTPRouter, pattern string, h http.HandlerFunc) {
	if pr, ok := r.(patchRouter); ok {
		pr.Patch(pattern, h)
		return
	}
	r.Mount(pattern, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPatch {
			http.NotFound(w, req)
			return
		}
		h(w, req)
	}))
}

func (cfg RouterConfig) withDefaults() RouterConfig {
	if cfg.Widgets == nil {
		cfg.Widgets = app.NewWidgetService()
	}
	if cfg.Tenancy == nil {
		cfg.Tenancy = app.NewTenancyService()
	}
	if cfg.APIKeys == nil {
		cfg.APIKeys = app.NewAPIKeyService(os.Getenv("API_KEY_PEPPER"), cfg.Tenancy)
	}
	if cfg.Async == nil {
		cfg.Async = app.NewAsyncService(cfg.Widgets)
	}
	if cfg.Audit == nil {
		cfg.Audit = app.NewAuditService()
	}
	if cfg.Webhooks == nil {
		cfg.Webhooks = app.NewWebhookService(cfg.Tenancy)
	}
	if cfg.Objects == nil {
		cfg.Objects = app.NewObjectService(cfg.Tenancy)
	}
	if cfg.Cache == nil {
		cfg.Cache = app.NewCacheService(nil)
	}

	// api-toolkit:router-default-services
	if cfg.MetricsHandler == nil {
		cfg.MetricsHandler = metricsmw.PrometheusHandler()
	}
	if cfg.RateLimit == nil {
		middleware, err := NewRateLimitMiddleware(nil)
		if err == nil {
			cfg.RateLimit = middleware
		}
	}
	if cfg.Idempotency == nil {
		middleware, err := NewIdempotencyMiddleware(idempotency.NewMemoryStore())
		if err == nil {
			cfg.Idempotency = middleware
		}
	}
	if cfg.APIKey == "" {
		cfg.APIKey = "local-dev-key"
	}
	if cfg.AdminKey == "" {
		cfg.AdminKey = "local-admin-key"
	}
	return cfg
}

func NewMetricsMiddleware(recorder metricsmw.MetricsRecorder) (*metricsmw.Middleware, error) {
	return metricsmw.New(metricsmw.Options{Recorder: recorder})
}

func NewRateLimitMiddleware(limiter ports.RateLimiter) (*ratelimitmw.Middleware, error) {
	return ratelimitmw.New(ratelimitmw.Options{
		Capacity:     20,
		RefillRate:   10,
		RetryAfter:   time.Second,
		Limiter:      limiter,
		Key:          fullRateLimitKey,
		HeaderConfig: ratelimitmw.DefaultHeaderConfig(),
	})
}

func NewIdempotencyMiddleware(store ports.IdempotencyStore) (*idempotencymw.Middleware, error) {
	return idempotencymw.New(idempotencymw.Options{
		Store:          store,
		StorageKeyFunc: fullIdempotencyStorageKey,
		HashFunc:       fullIdempotencyRequestHash,
		RequireKey:     true,
		ShouldStore: func(status int) bool {
			return status >= http.StatusOK && status < http.StatusBadRequest
		},
	})
}

func (cfg RouterConfig) metrics(next http.Handler) http.Handler {
	if cfg.Metrics == nil {
		return next
	}
	return cfg.Metrics.Handler(next)
}

func (cfg RouterConfig) rateLimited(next http.Handler) http.Handler {
	if cfg.RateLimit == nil {
		return next
	}
	return cfg.RateLimit.Handler(next)
}

func (cfg RouterConfig) idempotent(next http.Handler) http.Handler {
	if cfg.Idempotency == nil {
		return next
	}
	return cfg.Idempotency.Handler(next)
}

func fullRateLimitKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	actorID, tenantID := idempotencyScope(r)
	h := sha256.New()
	h.Write([]byte("saas-api-full:rate-limit-key:v1"))
	h.Write([]byte{0})
	h.Write([]byte(tenantID))
	h.Write([]byte{0})
	h.Write([]byte(actorID))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToUpper(r.Method)))
	h.Write([]byte{0})
	if r.URL != nil {
		h.Write([]byte(r.URL.Path))
	}
	return "atk:v1:" + hex.EncodeToString(h.Sum(nil))
}

func fullIdempotencyStorageKey(r *http.Request, clientKey string) string {
	clientKey = strings.TrimSpace(clientKey)
	if r == nil || clientKey == "" {
		return ""
	}
	actorID, tenantID := idempotencyScope(r)
	h := sha256.New()
	h.Write([]byte("saas-api-full:idempotency-storage-key:v1"))
	h.Write([]byte{0})
	h.Write([]byte(tenantID))
	h.Write([]byte{0})
	h.Write([]byte(actorID))
	h.Write([]byte{0})
	h.Write([]byte(clientKey))
	return "atk:v1:" + hex.EncodeToString(h.Sum(nil))
}

func fullIdempotencyRequestHash(r *http.Request, body []byte) (string, error) {
	if r == nil {
		return "", errors.New("request is nil")
	}
	actorID, tenantID := idempotencyScope(r)
	h := sha256.New()
	h.Write([]byte(actorID))
	h.Write([]byte{0})
	h.Write([]byte(tenantID))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToUpper(r.Method)))
	h.Write([]byte{0})
	if r.URL != nil {
		h.Write([]byte(r.URL.Path))
		h.Write([]byte{0})
		h.Write([]byte(r.URL.Query().Encode()))
	}
	h.Write([]byte{0})
	h.Write([]byte(r.Header.Get("Content-Type")))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func idempotencyScope(r *http.Request) (string, string) {
	if r == nil {
		return "", ""
	}
	actorID := strings.TrimSpace(os.Getenv("API_ACTOR_ID"))
	if actorID == "" && !strings.EqualFold(os.Getenv("ENV"), "production") {
		actorID = strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	}
	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if organizationID := strings.TrimSpace(r.PathValue("organization_id")); organizationID != "" {
		tenantID = organizationID
	}
	if tenantID == "" {
		tenantID = strings.TrimSpace(r.PathValue("id"))
	}
	if principal, ok := apiKeyPrincipalFromContext(r.Context()); ok {
		if actorID = principal.ActorID(); actorID == "" {
			actorID = strings.TrimSpace(os.Getenv("API_ACTOR_ID"))
		}
		if principalTenant := principal.TenantID(); principalTenant != "" && tenantID == "" {
			tenantID = principalTenant
		}
	}

	return actorID, tenantID
}

func NewHealthHandler(readiness HealthChecker) *health.Handler {
	manager := health.NewManagerWithConfig(ports.HealthCheckConfig{
		Timeout:         5 * time.Second,
		CacheDuration:   5 * time.Second,
		EnableCaching:   true,
		EnableDetailed:  true,
		LivenessChecks:  []string{"basic"},
		ReadinessChecks: []string{"basic", "dependencies"},
	})
	manager.RegisterChecker(health.NewBasicChecker())
	manager.RegisterChecker(health.NewCustomChecker("dependencies", func(ctx context.Context) (ports.HealthStatus, string, interface{}) {
		if readiness == nil {
			return ports.HealthStatusHealthy, "dependencies ready", nil
		}
		if err := readiness.Check(ctx); err != nil {
			return ports.HealthStatusUnhealthy, "dependencies unavailable", nil
		}
		return ports.HealthStatusHealthy, "dependencies ready", nil
	}))
	return health.NewHandler(manager)
}

func NewOpenAPIValidationMiddleware(cfg Config) (*openapimw.Middleware, error) {
	if !cfg.OpenAPIRequestValidation {
		return nil, nil
	}
	doc, err := OpenAPIDocument()
	if err != nil {
		return nil, err
	}
	spec, err := openapi3.NewLoader().LoadFromData(doc)
	if err != nil {
		return nil, err
	}
	opts := []openapimw.Option{
		openapimw.WithIgnoreNotFound(true),
		openapimw.WithFilterOptions(openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		}),
	}
	if cfg.OpenAPIResponseValidation {
		opts = append(opts, openapimw.WithResponseValidation(openapimw.ResponseValidationOptions{
			Enabled:      true,
			MaxBodyBytes: 1 << 20,
			ShouldValidate: func(r *http.Request) bool {
				if r == nil || r.URL == nil {
					return false
				}
				path := r.URL.Path
				return !strings.HasPrefix(path, "/docs") &&
					path != "/livez" &&
					path != "/readyz" &&
					path != "/health" &&
					path != "/healthz" &&
					path != "/health/detailed" &&
					path != "/metrics" &&
					!strings.HasPrefix(path, "/debug/pprof/")
			},
		}))
	}
	return openapimw.New(spec, opts...)
}

func handleLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (cfg RouterConfig) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := cfg.checkReady(r); err != nil {
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.Problem{Title: http.StatusText(http.StatusServiceUnavailable), Detail: "service is not ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (cfg RouterConfig) checkReady(r *http.Request) error {
	if cfg.Readiness == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	return cfg.Readiness.Check(ctx)
}

func handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	doc, err := OpenAPIDocument()
	if err != nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "openapi document unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(doc)
}

func (cfg RouterConfig) recordAudit(r *http.Request, tenantID, actorID, action, resourceType, resourceID string, metadata map[string]string) {
	if cfg.Audit == nil {
		return
	}
	_ = cfg.Audit.Record(r.Context(), audit.Event{
		TenantID: strings.TrimSpace(tenantID),
		Actor: audit.Actor{
			Type: "user",
			ID:   strings.TrimSpace(actorID),
		},
		Action: strings.TrimSpace(action),
		Resource: audit.Resource{
			Type: strings.TrimSpace(resourceType),
			ID:   strings.TrimSpace(resourceID),
		},
		Result:    audit.ResultSuccess,
		RequestID: requestID(r),
		Metadata:  metadata,
	})
}

func requestID(r *http.Request) string {
	for _, name := range []string{"X-Request-ID", "X-Correlation-ID"} {
		if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func (cfg RouterConfig) handleListOrganizations(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	orgs, err := cfg.Tenancy.ListOrganizations(r.Context(), actorID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(orgs))
	for _, org := range orgs {
		items = append(items, org.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (cfg RouterConfig) handleCreateOrganization(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeOrganizationRequest(w, r)
	if !ok {
		return
	}
	org, _, err := cfg.Tenancy.CreateOrganization(r.Context(), actorID, req.Name)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, org.ID, actorID, "organization.create", "organization", org.ID, nil)
	writeJSON(w, http.StatusCreated, org.Public())
}

func (cfg RouterConfig) handleListMembers(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	members, err := cfg.Tenancy.ListMembers(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(members))
	for _, member := range members {
		items = append(items, member.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (cfg RouterConfig) handleCreateInvitation(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeInvitationRequest(w, r)
	if !ok {
		return
	}
	invitation, token, err := cfg.Tenancy.InviteMember(r.Context(), actorID, organizationID, req.Email, req.Role)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "invitation.create", "invitation", invitation.ID, map[string]string{"role": string(invitation.Role)})
	writeJSON(w, http.StatusCreated, map[string]any{"invitation": invitation.Public(), "token": token})
}

func (cfg RouterConfig) handleListAPIKeys(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	keys, err := cfg.APIKeys.List(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		items = append(items, key.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (cfg RouterConfig) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeAPIKeyCreateRequest(w, r)
	if !ok {
		return
	}
	key, secret, err := cfg.APIKeys.Create(r.Context(), actorID, organizationID, req.Name, req.Scopes, req.ExpiresAt)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "api_key.create", "api_key", key.ID, map[string]string{
		"scope_count": strconv.Itoa(len(key.Scopes)),
		"expires":     strconv.FormatBool(key.ExpiresAt != nil),
	})
	writeJSON(w, http.StatusCreated, map[string]any{"api_key": key.Public(), "secret": secret})
}

func (cfg RouterConfig) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	keyID := strings.TrimSpace(r.PathValue("api_key_id"))
	if keyID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "api key id is required"})
		return
	}
	if err := cfg.APIKeys.Revoke(r.Context(), actorID, organizationID, keyID); err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "api_key.revoke", "api_key", keyID, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (cfg RouterConfig) handleListWebhookEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := cfg.authenticateActor(w, r); !ok {
		return
	}
	eventTypes, hit, err := cfg.Cache.WebhookEventTypes(r.Context(), cfg.Webhooks.EventTypes)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if hit {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	writeJSON(w, http.StatusOK, map[string]any{"event_types": eventTypes})
}

func (cfg RouterConfig) handleListWebhookEndpoints(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	endpoints, err := cfg.Webhooks.ListEndpointsForActor(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": endpoints})
}

func (cfg RouterConfig) handleCreateWebhookEndpoint(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeWebhookEndpointRequest(w, r)
	if !ok {
		return
	}
	created, err := cfg.Webhooks.CreateEndpoint(r.Context(), actorID, organizationID, req.URL, req.Events)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "webhook_endpoint.create", "webhook_endpoint", created.Endpoint.ID, map[string]string{"event_count": strconv.Itoa(len(created.Endpoint.Events))})
	writeJSON(w, http.StatusCreated, map[string]any{"endpoint": created.Endpoint, "secret": created.Secret})
}

func (cfg RouterConfig) handleListWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	deliveries, err := cfg.Webhooks.ListDeliveriesForActor(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": deliveries})
}

func (cfg RouterConfig) handleReplayWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	if !decodeWebhookReplayRequest(w, r) {
		return
	}
	deliveryID := strings.TrimSpace(r.PathValue("delivery_id"))
	if deliveryID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "delivery id is required"})
		return
	}
	delivery, err := cfg.Webhooks.ReplayDeliveryForActor(r.Context(), actorID, organizationID, deliveryID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "webhook_delivery.replay", "webhook_delivery", delivery.ID, map[string]string{"event_type": delivery.EventType})
	writeJSON(w, http.StatusAccepted, delivery)
}

func (cfg RouterConfig) handleListObjects(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	objects, err := cfg.Objects.List(r.Context(), actorID, organizationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(objects))
	for _, object := range objects {
		items = append(items, object.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (cfg RouterConfig) handlePutObject(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeObjectPutRequest(w, r)
	if !ok {
		return
	}
	object, err := cfg.Objects.Put(r.Context(), actorID, organizationID, req.Key, req.ContentType, req.Data)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "object.put", "object", object.Key, map[string]string{
		"content_type": object.ContentType,
		"size":         strconv.FormatInt(object.Size, 10),
	})
	writeJSON(w, http.StatusCreated, object.Public())
}

func (cfg RouterConfig) handleGetObject(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.PathValue("object_key"))
	object, data, found, err := cfg.Objects.Get(r.Context(), actorID, organizationID, key)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !found {
		writeAppError(w, app.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": object.Public(), "content_base64": base64.StdEncoding.EncodeToString(data)})
}

func (cfg RouterConfig) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	organizationID, ok := cfg.authenticateOrganizationTenant(w, r)
	if !ok {
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	key := strings.TrimSpace(r.PathValue("object_key"))
	if err := cfg.Objects.Delete(r.Context(), actorID, organizationID, key); err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, organizationID, actorID, "object.delete", "object", key, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (cfg RouterConfig) handleAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	invitationID := strings.TrimSpace(r.PathValue("id"))
	if invitationID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invitation id is required"})
		return
	}
	if _, ok := requireHeader(w, r, "Idempotency-Key"); !ok {
		return
	}
	req, ok := decodeAcceptInvitationRequest(w, r)
	if !ok {
		return
	}
	member, err := cfg.Tenancy.AcceptInvitation(r.Context(), invitationID, req.Token, actorID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	cfg.recordAudit(r, member.OrganizationID, actorID, "invitation.accept", "membership", member.UserID, nil)
	writeJSON(w, http.StatusOK, member.Public())
}

func (cfg RouterConfig) handleGetOperation(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	operationID := strings.TrimSpace(r.PathValue("id"))
	if operationID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "operation id is required"})
		return
	}
	operation, found, err := cfg.Async.GetOperation(r.Context(), tenantID, operationID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !found {
		writeAppError(w, app.ErrNotFound)
		return
	}
	apitkops.WriteOperation(w, http.StatusOK, operation)
}

func (cfg RouterConfig) handleListWidgets(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	widgets, err := cfg.Widgets.List(r.Context(), tenantID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(widgets))
	for _, widget := range widgets {
		items = append(items, widget.Public())
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}

func (cfg RouterConfig) handleCreateWidget(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	idempotencyKey, ok := requireHeader(w, r, "Idempotency-Key")
	if !ok {
		return
	}
	req, ok := decodeWidgetRequest(w, r)
	if !ok {
		return
	}
	widget, replayed, err := cfg.Widgets.Create(r.Context(), tenantID, req.Name, idempotencyKey)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("ETag", widget.ETag())
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	if !replayed {
		if _, err := cfg.Webhooks.DispatchEvent(r.Context(), tenantID, "widget.created", map[string]any{"id": widget.ID, "tenant_id": tenantID, "version": widget.Version}); err != nil {
			writeAppError(w, err)
			return
		}
	}
	cfg.recordAudit(r, tenantID, actorID, "widget.create", "widget", widget.ID, map[string]string{"replayed": strconv.FormatBool(replayed)})
	writeJSON(w, status, widget.Public())
}

func (cfg RouterConfig) handleCreateWidgetImport(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	idempotencyKey, ok := requireHeader(w, r, "Idempotency-Key")
	if !ok {
		return
	}
	req, ok := decodeWidgetImportRequest(w, r)
	if !ok {
		return
	}
	operation, replayed, err := cfg.Async.StartWidgetImport(r.Context(), tenantID, idempotencyKey, req.Items)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if replayed {
		w.Header().Set("Idempotent-Replay", "true")
	}
	cfg.recordAudit(r, tenantID, actorID, "widget_import.create", "operation", operation.ID, map[string]string{
		"item_count": strconv.Itoa(len(req.Items)),
		"replayed":   strconv.FormatBool(replayed),
	})
	apitkops.WriteAccepted(w, apitkops.AcceptedConfig{
		ID:         operation.ID,
		Location:   "/operations/" + operation.ID,
		RetryAfter: time.Second,
	})
}

func (cfg RouterConfig) handleUpdateWidget(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	idempotencyKey, ok := requireHeader(w, r, "Idempotency-Key")
	if !ok {
		return
	}
	ifMatch, ok := requireHeader(w, r, "If-Match")
	if !ok {
		return
	}
	req, ok := decodeWidgetRequest(w, r)
	if !ok {
		return
	}
	widget, replayed, err := cfg.Widgets.Update(r.Context(), tenantID, r.PathValue("id"), req.Name, ifMatch, idempotencyKey)
	if err != nil {
		writeAppError(w, err)
		return
	}
	w.Header().Set("ETag", widget.ETag())
	if !replayed {
		if _, err := cfg.Webhooks.DispatchEvent(r.Context(), tenantID, "widget.updated", map[string]any{"id": widget.ID, "tenant_id": tenantID, "version": widget.Version}); err != nil {
			writeAppError(w, err)
			return
		}
	}
	cfg.recordAudit(r, tenantID, actorID, "widget.update", "widget", widget.ID, nil)
	writeJSON(w, http.StatusOK, widget.Public())
}

func (cfg RouterConfig) handleDeleteWidget(w http.ResponseWriter, r *http.Request) {
	actorID, ok := cfg.authenticateActor(w, r)
	if !ok {
		return
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return
	}
	idempotencyKey, ok := requireHeader(w, r, "Idempotency-Key")
	if !ok {
		return
	}
	replayed, err := cfg.Widgets.Delete(r.Context(), tenantID, r.PathValue("id"), idempotencyKey)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if !replayed {
		if _, err := cfg.Webhooks.DispatchEvent(r.Context(), tenantID, "widget.deleted", map[string]any{"id": strings.TrimSpace(r.PathValue("id")), "tenant_id": tenantID}); err != nil {
			writeAppError(w, err)
			return
		}
	}
	cfg.recordAudit(r, tenantID, actorID, "widget.delete", "widget", r.PathValue("id"), nil)
	w.WriteHeader(http.StatusNoContent)
}

type widgetRequest struct {
	Name string
}

type widgetImportRequest struct {
	Items []app.WidgetImportItem
}

type organizationRequest struct {
	Name string
}

type invitationRequest struct {
	Email string
	Role  domain.Role
}

type acceptInvitationRequest struct {
	Token string
}

type apiKeyCreateRequest struct {
	Name      string
	Scopes    []string
	ExpiresAt *time.Time
}

type webhookEndpointRequest struct {
	URL    string
	Events []string
}

type objectPutRequest struct {
	Key         string
	ContentType string
	Data        []byte
}

func decodeOrganizationRequest(w http.ResponseWriter, r *http.Request) (organizationRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]string
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return organizationRequest{}, false
	}
	name := strings.TrimSpace(raw["name"])
	if name == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "name is required"})
		return organizationRequest{}, false
	}
	return organizationRequest{Name: name}, true
}

func decodeInvitationRequest(w http.ResponseWriter, r *http.Request) (invitationRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]string
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return invitationRequest{}, false
	}
	email := strings.TrimSpace(raw["email"])
	role := domain.Role(strings.TrimSpace(raw["role"]))
	if email == "" || !strings.Contains(email, "@") || !role.Valid() || role == domain.RoleOwner {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "valid email and non-owner role are required"})
		return invitationRequest{}, false
	}
	return invitationRequest{Email: email, Role: role}, true
}

func decodeAcceptInvitationRequest(w http.ResponseWriter, r *http.Request) (acceptInvitationRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]string
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return acceptInvitationRequest{}, false
	}
	token := strings.TrimSpace(raw["token"])
	if token == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "token is required"})
		return acceptInvitationRequest{}, false
	}
	return acceptInvitationRequest{Token: token}, true
}

func decodeAPIKeyCreateRequest(w http.ResponseWriter, r *http.Request) (apiKeyCreateRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw struct {
		Name      string   `json:"name"`
		Scopes    []string `json:"scopes"`
		ExpiresAt string   `json:"expires_at"`
	}
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return apiKeyCreateRequest{}, false
	}
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "name is required"})
		return apiKeyCreateRequest{}, false
	}
	scopes := make([]string, 0, len(raw.Scopes))
	for _, scope := range raw.Scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	if len(scopes) == 0 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "at least one scope is required"})
		return apiKeyCreateRequest{}, false
	}
	var expiresAt *time.Time
	if strings.TrimSpace(raw.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw.ExpiresAt))
		if err != nil {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "expires_at must be RFC3339"})
			return apiKeyCreateRequest{}, false
		}
		parsed = parsed.UTC()
		expiresAt = &parsed
	}
	return apiKeyCreateRequest{Name: name, Scopes: scopes, ExpiresAt: expiresAt}, true
}

func decodeWebhookEndpointRequest(w http.ResponseWriter, r *http.Request) (webhookEndpointRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
	}
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return webhookEndpointRequest{}, false
	}
	targetURL := strings.TrimSpace(raw.URL)
	if targetURL == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "url is required"})
		return webhookEndpointRequest{}, false
	}
	events := make([]string, 0, len(raw.Events))
	for _, eventType := range raw.Events {
		eventType = strings.TrimSpace(eventType)
		if eventType != "" {
			events = append(events, eventType)
		}
	}
	if len(events) == 0 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "at least one event is required"})
		return webhookEndpointRequest{}, false
	}
	return webhookEndpointRequest{URL: targetURL, Events: events}, true
}

func decodeWebhookReplayRequest(w http.ResponseWriter, r *http.Request) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return false
	}
	if len(raw) != 0 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "replay body must be empty"})
		return false
	}
	return true
}

func decodeObjectPutRequest(w http.ResponseWriter, r *http.Request) (objectPutRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	var raw struct {
		Key           string `json:"key"`
		ContentType   string `json:"content_type"`
		ContentBase64 string `json:"content_base64"`
	}
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return objectPutRequest{}, false
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw.ContentBase64))
	if err != nil || len(data) == 0 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "content_base64 is required"})
		return objectPutRequest{}, false
	}
	return objectPutRequest{Key: strings.TrimSpace(raw.Key), ContentType: strings.TrimSpace(raw.ContentType), Data: data}, true
}

func decodeWidgetRequest(w http.ResponseWriter, r *http.Request) (widgetRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw map[string]string
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return widgetRequest{}, false
	}
	name := strings.TrimSpace(raw["name"])
	if name == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "name is required"})
		return widgetRequest{}, false
	}
	return widgetRequest{Name: name}, true
}

func decodeWidgetImportRequest(w http.ResponseWriter, r *http.Request) (widgetImportRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var raw struct {
		Items []app.WidgetImportItem `json:"items"`
	}
	if err := decoder.Decode(&raw); err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "invalid JSON request body"})
		return widgetImportRequest{}, false
	}
	if len(raw.Items) == 0 || len(raw.Items) > 100 {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "items must contain 1 to 100 widgets"})
		return widgetImportRequest{}, false
	}
	for _, item := range raw.Items {
		name := strings.TrimSpace(item.Name)
		if name == "" || len(name) > 120 {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "item names are required"})
			return widgetImportRequest{}, false
		}
	}
	return widgetImportRequest{Items: raw.Items}, true
}

func (cfg RouterConfig) authenticateManagedAPIKey(w http.ResponseWriter, r *http.Request) (apiKeyPrincipal, bool) {
	if cfg.APIKeys == nil {
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "API key service is not configured"})
		return apiKeyPrincipal{}, false
	}
	key, ok, err := cfg.APIKeys.Verify(r.Context(), r.Header.Get("X-API-Key"))
	if err != nil || !ok {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid API key required"})
		return apiKeyPrincipal{}, false
	}
	return apiKeyPrincipal{Key: key}, true
}

func (cfg RouterConfig) authenticateActor(w http.ResponseWriter, r *http.Request) (string, bool) {

	if principal, ok := apiKeyPrincipalFromContext(r.Context()); ok {
		if actorID := principal.ActorID(); actorID != "" {
			return actorID, true
		}
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "API key actor required"})
		return "", false
	}
	if !sameSecret(r.Header.Get("X-API-Key"), cfg.APIKey) {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid API key required"})
		return "", false
	}
	actorID := strings.TrimSpace(os.Getenv("API_ACTOR_ID"))
	if actorID == "" && !strings.EqualFold(os.Getenv("ENV"), "production") {
		actorID = strings.TrimSpace(r.Header.Get("X-Actor-ID"))
	}
	if actorID == "" {
		actorID = "local-api-key"
	}
	return actorID, true

}

func (cfg RouterConfig) authenticateOrganizationTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	organizationID := strings.TrimSpace(r.PathValue("organization_id"))
	if organizationID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "organization id is required"})
		return "", false
	}
	tenantID, ok := cfg.authenticateTenant(w, r)
	if !ok {
		return "", false
	}
	if tenantID != organizationID {
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "tenant path mismatch"})
		return "", false
	}
	return organizationID, true
}

func (cfg RouterConfig) authenticateTenant(w http.ResponseWriter, r *http.Request) (string, bool) {

	tenantID, ok := requireHeader(w, r, "X-Tenant-ID")
	if !ok {
		return "", false
	}
	if principal, ok := apiKeyPrincipalFromContext(r.Context()); ok {
		if principal.TenantID() != tenantID {
			httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "tenant credential mismatch"})
			return "", false
		}
		return tenantID, true
	}
	if !sameSecret(r.Header.Get("X-API-Key"), cfg.APIKey) {
		httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "valid API key required"})
		return "", false
	}
	return tenantID, true

}

func (cfg RouterConfig) protect(requiredScope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sameSecret(r.Header.Get("X-API-Key"), cfg.APIKey) {
			cfg.rateLimited(next).ServeHTTP(w, r)
			return
		}
		principal, ok := cfg.authenticateManagedAPIKey(w, r)
		if !ok {
			return
		}
		if !principal.HasScope(requiredScope) {
			httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "required API key scope missing"})
			return
		}
		cfg.rateLimited(next).ServeHTTP(w, r.WithContext(withAPIKeyPrincipal(r.Context(), principal.Key)))
	})

}

func (cfg RouterConfig) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !sameSecret(r.Header.Get("X-Admin-Key"), cfg.AdminKey) {
			httpx.WriteProblem(w, http.StatusUnauthorized, httpx.Problem{Title: http.StatusText(http.StatusUnauthorized), Detail: "admin authentication required"})
			return
		}
		next(w, r)
	}
}

func (cfg RouterConfig) requireAdminHandler(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(cfg.requireAdmin(next.ServeHTTP))
}

func RequireAdmin(adminKey string) func(http.Handler) http.Handler {
	cfg := RouterConfig{AdminKey: adminKey}.withDefaults()
	return cfg.requireAdminHandler
}

func requireHeader(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	value := strings.TrimSpace(r.Header.Get(name))
	if value == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: name + " header is required"})
		return "", false
	}
	return value, true
}

func sameSecret(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func writeAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrValidation):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{Title: http.StatusText(http.StatusBadRequest), Detail: "request validation failed"})
	case errors.Is(err, app.ErrForbidden):
		httpx.WriteProblem(w, http.StatusForbidden, httpx.Problem{Title: http.StatusText(http.StatusForbidden), Detail: "permission denied"})
	case errors.Is(err, app.ErrNotFound):
		httpx.WriteProblem(w, http.StatusNotFound, httpx.Problem{Title: http.StatusText(http.StatusNotFound), Detail: "resource not found"})
	case errors.Is(err, app.ErrPreconditionFailed):
		httpx.WriteProblem(w, http.StatusPreconditionFailed, httpx.Problem{Title: http.StatusText(http.StatusPreconditionFailed), Detail: "If-Match does not match current resource version"})
	default:
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.Problem{Title: http.StatusText(http.StatusInternalServerError), Detail: "request failed"})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
