package contracts

import (
	"context"
	"net/http"
	"time"
)

// DatabasePool defines the database capability shared by contrib adapters.
type DatabasePool interface {
	Ping(ctx context.Context) error
	Close()
	Acquire(ctx context.Context) (DatabaseConnection, error)
}

// DatabasePoolSnapshotProvider exposes an optional plain-value pool snapshot.
type DatabasePoolSnapshotProvider interface {
	StatSnapshot() DatabasePoolSnapshot
}

// DatabaseConnection defines an adapter-level database connection.
type DatabaseConnection interface {
	Query(ctx context.Context, sql string, args ...any) (DatabaseRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DatabaseRow
	Exec(ctx context.Context, sql string, args ...any) (DatabaseResult, error)
	Begin(ctx context.Context) (DatabaseTransaction, error)
	Release()
}

// DatabaseTransaction defines an adapter-level transaction.
type DatabaseTransaction interface {
	Query(ctx context.Context, sql string, args ...any) (DatabaseRows, error)
	QueryRow(ctx context.Context, sql string, args ...any) DatabaseRow
	Exec(ctx context.Context, sql string, args ...any) (DatabaseResult, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// DatabaseRows defines an adapter-level query result collection.
type DatabaseRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

// DatabaseRow defines an adapter-level single row result.
type DatabaseRow interface {
	Scan(dest ...any) error
}

// DatabaseResult defines an adapter-level execution result.
type DatabaseResult interface {
	RowsAffected() int64
}

// DatabasePoolSnapshot captures plain-value connection-pool statistics.
type DatabasePoolSnapshot struct {
	AcquireCount         int64
	AcquireDuration      time.Duration
	AcquiredConns        int32
	CanceledAcquireCount int64
	ConstructingConns    int32
	EmptyAcquireCount    int64
	IdleConns            int32
	MaxConns             int32
	NewConnsCount        int64
	TotalConns           int32
}

// SnapshotDatabasePoolStats copies an optional pool snapshot.
func SnapshotDatabasePoolStats(pool DatabasePool) DatabasePoolSnapshot {
	if pool == nil {
		return DatabasePoolSnapshot{}
	}
	if snapshotter, ok := pool.(DatabasePoolSnapshotProvider); ok {
		return snapshotter.StatSnapshot()
	}
	return DatabasePoolSnapshot{}
}

// MethodRouteRegistrar defines the GET-only route registration shape used by adapters.
type MethodRouteRegistrar interface {
	Get(pattern string, h http.HandlerFunc)
}

// MiddlewareChain defines the middleware registration shape used by adapters.
type MiddlewareChain interface {
	Use(middlewares ...func(http.Handler) http.Handler)
}

// HTTPRouter defines the router shape used by contrib composition.
type HTTPRouter interface {
	http.Handler
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
	Mount(pattern string, h http.Handler)
	Use(middlewares ...func(http.Handler) http.Handler)
}

// HTTPMiddleware defines common router middleware capabilities.
type HTTPMiddleware interface {
	RequestID() func(http.Handler) http.Handler
	RealIP() func(http.Handler) http.Handler
	Recoverer() func(http.Handler) http.Handler
}

// Middleware defines the adapter middleware shape.
type Middleware interface {
	Middleware() func(http.Handler) http.Handler
}

// CORSHandler defines the contrib CORS handler shape.
type CORSHandler interface {
	Handler(opts CORSOptions) func(http.Handler) http.Handler
}

// CORSOptions defines contrib CORS configuration.
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

// URLParamExtractor defines the router path-parameter capability used by adapters.
type URLParamExtractor interface {
	URLParam(r *http.Request, key string) string
}

// HTTPClient describes an outbound HTTP client used by contrib adapters.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// EnvVar manages environment variables with typed getters for contrib config.
type EnvVar interface {
	LoadEnvFiles(paths []string) error
	Get(key string) (string, bool)
	GetOr(key, def string) string
	MustGet(key string) string
	GetBoolOr(key string, def bool) bool
	MustGetBool(key string) bool
	GetIntOr(key string, def int) int
	MustGetInt(key string) int
	GetInt64Or(key string, def int64) int64
	MustGetInt64(key string) int64
	GetUintOr(key string, def uint) uint
	MustGetUint(key string) uint
	GetUint64Or(key string, def uint64) uint64
	MustGetUint64(key string) uint64
	GetFloat64Or(key string, def float64) float64
	MustGetFloat64(key string) float64
	MustGetDuration(key string) time.Duration
	GetDurationOr(key string, def time.Duration) time.Duration
	Bind(dst any) error
	MustBind(dst any)
	BindWithPrefix(dst any, prefix string) error
	MustBindWithPrefix(dst any, prefix string)
	DumpRedacted() map[string]string
}

// Validator defines the adapter validation contract.
type Validator interface {
	Validate(ctx context.Context, value interface{}) error
	ValidateStruct(ctx context.Context, obj interface{}) error
	ValidateField(ctx context.Context, obj interface{}, field string) error
}

// TxManager runs a function within an adapter-owned transaction boundary.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Migrator defines the adapter migration lifecycle contract.
type Migrator interface {
	Up(ctx context.Context, dir string) error
	Down(ctx context.Context, dir string) error
	Status(ctx context.Context, dir string) (string, error)
	Close() error
}
