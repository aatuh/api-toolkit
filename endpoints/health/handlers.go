package health

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/v4/httpx"
	"github.com/aatuh/api-toolkit/v4/specs"
)

// Handler provides HTTP handlers for health endpoints.
type Handler struct {
	manager ManagerContract
}

// NewHandler creates a new health handler.
func NewHandler(manager ManagerContract) *Handler {
	if manager == nil {
		manager = New()
	}
	return &Handler{manager: manager}
}

// NewBasicHandler builds a handler with just the basic checker.
func NewBasicHandler() *Handler {
	manager := New()
	manager.RegisterChecker(NewBasicChecker())
	return NewHandler(manager)
}

// NewDefaultHandler builds a handler with standard checkers.
func NewDefaultHandler(pool DatabasePool) *Handler {
	config := Config{
		Timeout:         5 * time.Second,
		CacheDuration:   5 * time.Second,
		EnableCaching:   true,
		EnableDetailed:  false,
		LivenessChecks:  []string{"basic", "memory"},
		ReadinessChecks: []string{"basic", "memory"},
	}
	checkers := []Checker{
		NewBasicChecker(),
		NewMemoryChecker(1024),
	}
	if pool != nil {
		config.ReadinessChecks = append(config.ReadinessChecks, "database")
		checkers = append(checkers, NewDatabaseChecker(pool))
	}
	manager := NewWithConfig(config)
	manager.RegisterCheckers(checkers...)
	return NewHandler(manager)
}

// LivenessHandler handles liveness checks.
// @Summary Liveness probe
// @Description Returns the liveness status of the application
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Application is alive"
// @Failure 503 {object} map[string]interface{} "Application is not alive"
// @Router /livez [get]
func (h *Handler) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result := h.manager.GetLiveness(ctx)

	statusCode := http.StatusOK
	if result.Status == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    result.Status,
		"timestamp": result.Timestamp,
		"message":   result.Message,
	}

	httpx.WriteJSONChecked(w, statusCode, response)
}

// ReadinessHandler handles readiness checks.
// @Summary Readiness probe
// @Description Returns the readiness status of the application
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Application is ready"
// @Failure 503 {object} map[string]interface{} "Application is not ready"
// @Router /readyz [get]
func (h *Handler) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result := h.manager.GetReadiness(ctx)

	statusCode := http.StatusOK
	if result.Status == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    result.Status,
		"timestamp": result.Timestamp,
		"message":   result.Message,
	}

	httpx.WriteJSONChecked(w, statusCode, response)
}

// HealthHandler handles basic health checks.
// @Summary Health check
// @Description Returns the basic health status of the application
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Application is healthy"
// @Failure 503 {object} map[string]interface{} "Application is unhealthy"
// @Router /healthz [get]
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	response := h.manager.GetHealth(ctx)

	statusCode := http.StatusOK
	if response.Status == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	httpx.WriteJSONChecked(w, statusCode, response)
}

// DetailedHealthHandler handles detailed health checks.
// @Summary Detailed health check
// @Description Returns detailed health information including individual checks
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} DetailedHealthResponse "Detailed health information"
// @Failure 503 {object} DetailedHealthResponse "Application is unhealthy"
// @Router /health/detailed [get]
func (h *Handler) DetailedHealthHandler(w http.ResponseWriter, r *http.Request) {
	if !h.detailedHealthEnabled() {
		httpx.WriteProblemChecked(w, http.StatusNotFound, httpx.Problem{
			Type:   httpx.DefaultTypeURI(httpx.TypeNotFound),
			Title:  http.StatusText(http.StatusNotFound),
			Detail: "detailed health is disabled",
		})
		return
	}

	ctx := r.Context()
	response := h.manager.GetDetailedHealth(ctx)

	statusCode := http.StatusOK
	if response.Status == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	httpx.WriteJSONChecked(w, statusCode, response)
}

// RegisterRoutes registers all enabled health endpoints on the given router.
// When detailed health is enabled, prefer registering public probes separately
// and mounting DetailedHealthHandler with RegisterAdminDetailedHealthRoute.
func (h *Handler) RegisterRoutes(router interface {
	Get(pattern string, h http.HandlerFunc)
}) {
	if router == nil {
		return
	}
	h.RegisterRoutesTo(healthRouteRegistrar{register: router.Get})
}

// RegisterRoutesTo registers all enabled health endpoints on the given registrar.
// When detailed health is enabled, prefer registering public probes separately
// and mounting DetailedHealthHandler with RegisterAdminDetailedHealthRoute.
func (h *Handler) RegisterRoutesTo(router RouteRegistrar) {
	h.RegisterPublicRoutesTo(router)
	if h == nil || router == nil {
		return
	}
	if h.detailedHealthEnabled() {
		router.Get(specs.HealthDetailed, h.DetailedHealthHandler)
	}
}

// RegisterPublicRoutesTo registers only public health probe endpoints on the
// given registrar. It never mounts detailed dependency health, even when
// detailed health is enabled on the manager.
func (h *Handler) RegisterPublicRoutesTo(router RouteRegistrar) {
	if h == nil || router == nil {
		return
	}

	router.Get(specs.Livez, h.LivenessHandler)
	router.Get(specs.Readyz, h.ReadinessHandler)
	router.Get(specs.Healthz, h.HealthHandler)
	router.Get(specs.Health, h.HealthHandler)
}

// RegisterAdminDetailedHealthRoute registers only the detailed health route
// behind an explicit authorization or internal-network wrapper. Passing nil
// fails closed so operator-only dependency detail cannot be mounted without
// policy.
func (h *Handler) RegisterAdminDetailedHealthRoute(router RouteRegistrar, requireAdmin func(http.Handler) http.Handler) error {
	if h == nil || router == nil {
		return nil
	}
	if requireAdmin == nil {
		return errors.New("detailed health admin route requires an authorization wrapper")
	}
	if h.detailedHealthEnabled() {
		router.Get(specs.HealthDetailed, requireAdmin(http.HandlerFunc(h.DetailedHealthHandler)).ServeHTTP)
	}
	return nil
}

// RegisterCustomRoutes registers health endpoints with custom paths. When
// DetailedHealth is set, prefer RegisterAdminDetailedHealthRoute for the
// detailed route unless an upstream internal-only policy already protects it.
func (h *Handler) RegisterCustomRoutes(router interface {
	Get(pattern string, h http.HandlerFunc)
}, paths HealthPaths) {
	if router == nil {
		return
	}
	h.RegisterCustomRoutesTo(healthRouteRegistrar{register: router.Get}, paths)
}

// RegisterCustomRoutesTo registers health endpoints with custom paths. When
// DetailedHealth is set, prefer RegisterAdminDetailedHealthRoute for the
// detailed route unless an upstream internal-only policy already protects it.
func (h *Handler) RegisterCustomRoutesTo(router RouteRegistrar, paths HealthPaths) {
	if h == nil || router == nil {
		return
	}

	if paths.Liveness != "" {
		router.Get(paths.Liveness, h.LivenessHandler)
	}
	if paths.Readiness != "" {
		router.Get(paths.Readiness, h.ReadinessHandler)
	}
	if paths.Health != "" {
		router.Get(paths.Health, h.HealthHandler)
	}
	if paths.DetailedHealth != "" && h.detailedHealthEnabled() {
		router.Get(paths.DetailedHealth, h.DetailedHealthHandler)
	}
}

// HealthPaths defines custom paths for health endpoints.
//
//revive:disable-next-line:exported
type HealthPaths struct {
	Liveness       string
	Readiness      string
	Health         string
	DetailedHealth string
}

// DefaultHealthPaths returns the default health endpoint paths.
func DefaultHealthPaths() HealthPaths {
	return HealthPaths{
		Liveness:       specs.Livez,
		Readiness:      specs.Readyz,
		Health:         specs.Health,
		DetailedHealth: specs.HealthDetailed,
	}
}

// Middleware creates a middleware that adds cached or local health information
// to requests without probing dependencies on the request path.
func (h *Handler) Middleware() func(http.Handler) http.Handler {
	if h == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			health := h.cachedOrLocalHealth()
			ctx := context.WithValue(r.Context(), healthStatusKey, health.Status)
			ctx = context.WithValue(ctx, healthTimestampKey, health.Timestamp)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (h *Handler) cachedOrLocalHealth() Response {
	response := Response{
		Status:    StatusUnknown,
		Timestamp: time.Now(),
	}
	if h == nil || h.manager == nil {
		return response
	}
	cached, ok := h.manager.(CachedManager)
	if !ok {
		return response
	}
	snapshot, ok := cached.CachedHealth()
	if !ok {
		return response
	}
	if snapshot.Status == "" {
		snapshot.Status = StatusUnknown
	}
	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}
	return snapshot
}

// HealthStatusFromContext extracts health status from request context.
//
//revive:disable-next-line:exported
func HealthStatusFromContext(ctx context.Context) (Status, bool) {
	status, ok := ctx.Value(healthStatusKey).(Status)
	return status, ok
}

// HealthTimestampFromContext extracts health timestamp from request context.
//
//revive:disable-next-line:exported
func HealthTimestampFromContext(ctx context.Context) (time.Time, bool) {
	timestamp, ok := ctx.Value(healthTimestampKey).(time.Time)
	return timestamp, ok
}

type healthContextKey string

const (
	healthStatusKey    healthContextKey = "health_status"
	healthTimestampKey healthContextKey = "health_timestamp"
)

type healthRouteRegistrar struct {
	register func(string, http.HandlerFunc)
}

func (r healthRouteRegistrar) Get(pattern string, h http.HandlerFunc) {
	r.register(pattern, h)
}

func (h *Handler) detailedHealthEnabled() bool {
	if h == nil {
		return false
	}
	mgr, ok := h.manager.(DetailedManager)
	if !ok {
		return false
	}
	return mgr.DetailedHealthEnabled()
}
