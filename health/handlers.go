package health

import (
	"context"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/response_writer"
	"github.com/aatuh/api-toolkit/specs"
)

// Handler provides HTTP handlers for health endpoints.
type Handler struct {
	manager ports.HealthManager
}

// NewHandler creates a new health handler.
func NewHandler(manager ports.HealthManager) *Handler {
	return &Handler{manager: manager}
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
	if result.Status == ports.HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    result.Status,
		"timestamp": result.Timestamp,
		"message":   result.Message,
	}

	response_writer.WriteJSON(w, statusCode, response)
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
	if result.Status == ports.HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	response := map[string]interface{}{
		"status":    result.Status,
		"timestamp": result.Timestamp,
		"message":   result.Message,
	}

	response_writer.WriteJSON(w, statusCode, response)
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
	if response.Status == ports.HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	response_writer.WriteJSON(w, statusCode, response)
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
	ctx := r.Context()
	response := h.manager.GetDetailedHealth(ctx)

	statusCode := http.StatusOK
	if response.Status == ports.HealthStatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	}

	response_writer.WriteJSON(w, statusCode, response)
}

// RegisterRoutes registers all health endpoints on the given router.
func (h *Handler) RegisterRoutes(router interface {
	Get(pattern string, h http.HandlerFunc)
}) {
	// Standard Kubernetes-style endpoints
	router.Get(specs.Livez, h.LivenessHandler)
	router.Get(specs.Readyz, h.ReadinessHandler)
	router.Get(specs.Healthz, h.HealthHandler)

	// Additional endpoints
	router.Get(specs.Health, h.HealthHandler)
	router.Get(specs.HealthDetailed, h.DetailedHealthHandler)
}

// RegisterCustomRoutes registers health endpoints with custom paths.
func (h *Handler) RegisterCustomRoutes(router interface {
	Get(pattern string, h http.HandlerFunc)
}, paths HealthPaths) {
	if paths.Liveness != "" {
		router.Get(paths.Liveness, h.LivenessHandler)
	}
	if paths.Readiness != "" {
		router.Get(paths.Readiness, h.ReadinessHandler)
	}
	if paths.Health != "" {
		router.Get(paths.Health, h.HealthHandler)
	}
	if paths.DetailedHealth != "" {
		router.Get(paths.DetailedHealth, h.DetailedHealthHandler)
	}
}

// HealthPaths defines custom paths for health endpoints.
type HealthPaths struct {
	Liveness       string
	Readiness      string
	Health         string
	DetailedHealth string
}

// DefaultHealthPaths returns the default health endpoint paths.
func DefaultHealthPaths() HealthPaths {
	return HealthPaths{
		Liveness:       "/live",
		Readiness:      "/ready",
		Health:         "/health",
		DetailedHealth: "/health/detailed",
	}
}

// Middleware creates a middleware that adds health information to requests.
func (h *Handler) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add health status to request context
			ctx := r.Context()
			health := h.manager.GetHealth(ctx)

			// Add to context for use by other handlers
			ctx = context.WithValue(ctx, "health_status", health.Status)
			ctx = context.WithValue(ctx, "health_timestamp", health.Timestamp)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// HealthStatusFromContext extracts health status from request context.
func HealthStatusFromContext(ctx context.Context) (ports.HealthStatus, bool) {
	status, ok := ctx.Value("health_status").(ports.HealthStatus)
	return status, ok
}

// HealthTimestampFromContext extracts health timestamp from request context.
func HealthTimestampFromContext(ctx context.Context) (time.Time, bool) {
	timestamp, ok := ctx.Value("health_timestamp").(time.Time)
	return timestamp, ok
}
