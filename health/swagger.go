package health

import "time"

// HealthResponse represents a health check response for Swagger documentation.
type HealthResponse struct {
	Status    string    `json:"status" example:"healthy"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

// DetailedHealthResponse represents a detailed health response for Swagger documentation.
type DetailedHealthResponse struct {
	Status    string                  `json:"status" example:"healthy"`
	Timestamp time.Time               `json:"timestamp"`
	Checks    map[string]HealthResult `json:"checks"`
	Summary   HealthSummary           `json:"summary"`
}

// HealthResult represents a single health check result for Swagger documentation.
type HealthResult struct {
	Status    string      `json:"status" example:"healthy"`
	Message   string      `json:"message,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Duration  int64       `json:"duration,omitempty"` // Duration in nanoseconds
}

// HealthSummary provides a summary of all health checks for Swagger documentation.
type HealthSummary struct {
	Total     int `json:"total" example:"3"`
	Healthy   int `json:"healthy" example:"3"`
	Unhealthy int `json:"unhealthy" example:"0"`
	Degraded  int `json:"degraded" example:"0"`
	Unknown   int `json:"unknown" example:"0"`
}
