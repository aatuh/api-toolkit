package ports

import (
	"context"
	"net/http"
	"time"
)

// Logger is a tiny façade to avoid vendor lock-in.
type Logger interface {
	Debug(msg string, kv ...any)
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
}

// Clock allows deterministic tests.
type Clock interface {
	Now() time.Time
}

// IDGen generates unique IDs.
type IDGen interface {
	New() string
}

// Validator defines the interface for input validation.
type Validator interface {
	Validate(ctx context.Context, value interface{}) error
	ValidateStruct(ctx context.Context, obj interface{}) error
	ValidateField(ctx context.Context, obj interface{}, field string) error
}

// TxManager runs a function within a transaction boundary.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Migrator is the app-level contract used by main and CLI.
type Migrator interface {
	Up(ctx context.Context, dir string) error
	Down(ctx context.Context, dir string) error
	Status(ctx context.Context, dir string) (string, error)
}

// EnvVar manages environment variables with typed getters.
type EnvVar interface {
	// MustGet returns the value or panics if not present.
	MustGet(key string) string
	// MustGetBool returns the value as a boolean or panics if not present.
	MustGetBool(key string) bool
	// MustGetInt returns the value as an integer or panics if not present.
	MustGetInt(key string) int
	// MustGetInt64 returns the value as an int64 or panics if not present.
	MustGetInt64(key string) int64
	// MustGetUint returns the value as a uint or panics if not present.
	MustGetUint(key string) uint
	// MustGetUint64 returns the value as a uint64 or panics if not present.
	MustGetUint64(key string) uint64
	// MustGetFloat64 returns the value as a float64 or panics if not present.
	MustGetFloat64(key string) float64
	// MustGetDuration returns the value as a duration or panics if not present.
	MustGetDuration(key string) time.Duration
}

// HTTPRouter defines the interface for HTTP routing.
type HTTPRouter interface {
	http.Handler
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
	Mount(pattern string, h http.Handler)
	Use(middlewares ...func(http.Handler) http.Handler)
}

// HTTPMiddleware defines the interface for HTTP middleware.
type HTTPMiddleware interface {
	RequestID() func(http.Handler) http.Handler
	RealIP() func(http.Handler) http.Handler
	Recoverer() func(http.Handler) http.Handler
}

// CORSHandler defines the interface for CORS handling.
type CORSHandler interface {
	Handler(opts CORSOptions) func(http.Handler) http.Handler
}

// CORSOptions defines CORS configuration.
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// URLParamExtractor defines the interface for extracting URL parameters.
type URLParamExtractor interface {
	URLParam(r *http.Request, key string) string
}

// DatabasePool defines the interface for database connection pooling.
type DatabasePool interface {
	Ping(ctx context.Context) error
	Close()
	Acquire(ctx context.Context) (DatabaseConnection, error)
	Stat() DatabaseStats
}

// DatabaseConnection defines the interface for individual database connections.
type DatabaseConnection interface {
	Query(ctx context.Context, sql string, args ...any) (DatabaseRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DatabaseRow
	Exec(ctx context.Context, sql string, args ...any) (DatabaseResult, error)
	Begin(ctx context.Context) (DatabaseTransaction, error)
	Release()
}

// DatabaseTransaction defines the interface for database transactions.
type DatabaseTransaction interface {
	Query(ctx context.Context, sql string, args ...any) (DatabaseRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DatabaseRow
	Exec(ctx context.Context, sql string, args ...any) (DatabaseResult, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// DatabaseRows defines the interface for query result rows.
type DatabaseRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// DatabaseRow defines the interface for a single query result row.
type DatabaseRow interface {
	Scan(dest ...any) error
}

// DatabaseResult defines the interface for query execution results.
type DatabaseResult interface {
	RowsAffected() int64
}

// DatabaseStats defines the interface for database pool statistics.
type DatabaseStats interface {
	AcquireCount() int64
	AcquireDuration() time.Duration
	AcquiredConns() int32
	CanceledAcquireCount() int64
	ConstructingConns() int32
	EmptyAcquireCount() int64
	IdleConns() int32
	MaxConns() int32
	NewConnsCount() int64
	TotalConns() int32
}

// HealthChecker defines the interface for individual health checks.
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) HealthResult
}

// HealthResult represents the result of a health check.
type HealthResult struct {
	Status    HealthStatus  `json:"status"`
	Message   string        `json:"message,omitempty"`
	Details   interface{}   `json:"details,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration,omitempty"`
}

// HealthStatus represents the status of a health check.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// HealthManager defines the interface for managing health checks.
type HealthManager interface {
	RegisterChecker(checker HealthChecker)
	RegisterCheckers(checkers ...HealthChecker)
	GetLiveness(ctx context.Context) HealthResult
	GetReadiness(ctx context.Context) HealthResult
	GetHealth(ctx context.Context) HealthResponse
	GetDetailedHealth(ctx context.Context) DetailedHealthResponse
}

// HealthResponse represents the overall health response.
type HealthResponse struct {
	Status    HealthStatus `json:"status"`
	Timestamp time.Time    `json:"timestamp"`
	Message   string       `json:"message,omitempty"`
}

// DetailedHealthResponse represents a detailed health response with individual checks.
type DetailedHealthResponse struct {
	Status    HealthStatus            `json:"status"`
	Timestamp time.Time               `json:"timestamp"`
	Checks    map[string]HealthResult `json:"checks"`
	Summary   HealthSummary           `json:"summary"`
}

// HealthSummary provides a summary of all health checks.
type HealthSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Degraded  int `json:"degraded"`
	Unknown   int `json:"unknown"`
}

// HealthCheckConfig defines configuration for health checks.
type HealthCheckConfig struct {
	Timeout         time.Duration `json:"timeout"`
	CacheDuration   time.Duration `json:"cache_duration"`
	EnableCaching   bool          `json:"enable_caching"`
	EnableDetailed  bool          `json:"enable_detailed"`
	LivenessChecks  []string      `json:"liveness_checks"`
	ReadinessChecks []string      `json:"readiness_checks"`
}

// HealthCheckRegistry defines the interface for registering health checks.
type HealthCheckRegistry interface {
	Register(name string, checker HealthChecker)
	Unregister(name string)
	GetChecker(name string) (HealthChecker, bool)
	ListCheckers() []string
}

// DocsProvider defines the interface for providing documentation content.
type DocsProvider interface {
	GetHTML() (string, error)
	GetOpenAPI() ([]byte, error)
	GetVersion() (string, error)
	GetInfo() DocsInfo
}

// DocsInfo provides information about the API documentation.
type DocsInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Contact     string `json:"contact,omitempty"`
	License     string `json:"license,omitempty"`
}

// DocsManager defines the interface for managing documentation.
type DocsManager interface {
	RegisterProvider(provider DocsProvider)
	GetHTML() (string, error)
	GetOpenAPI() ([]byte, error)
	GetVersion() (string, error)
	GetInfo() DocsInfo
	ServeHTML(w http.ResponseWriter, r *http.Request)
	ServeOpenAPI(w http.ResponseWriter, r *http.Request)
	ServeVersion(w http.ResponseWriter, r *http.Request)
	ServeInfo(w http.ResponseWriter, r *http.Request)
}

// DocsConfig defines configuration for documentation.
type DocsConfig struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	Contact     string    `json:"contact,omitempty"`
	License     string    `json:"license,omitempty"`
	Paths       DocsPaths `json:"paths"`
	EnableHTML  bool      `json:"enable_html"`
	EnableJSON  bool      `json:"enable_json"`
	EnableYAML  bool      `json:"enable_yaml"`
}

// DocsPaths defines the paths for documentation endpoints.
type DocsPaths struct {
	HTML    string `json:"html"`
	OpenAPI string `json:"openapi"`
	Version string `json:"version"`
	Info    string `json:"info"`
}

// DefaultDocsPaths returns the default documentation endpoint paths.
func DefaultDocsPaths() DocsPaths {
	return DocsPaths{
		HTML:    "/docs",
		OpenAPI: "/docs/openapi.json",
		Version: "/docs/version",
		Info:    "/docs/info",
	}
}

// SecurityHandler defines the interface for security middleware.
type SecurityHandler interface {
	Middleware() func(http.Handler) http.Handler
}
