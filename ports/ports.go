package ports

import (
	"context"
	"encoding/json"
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

// Migrator defines an interface for database migrations.
type Migrator interface {
	Up(ctx context.Context, dir string) error
	Down(ctx context.Context, dir string) error
	Status(ctx context.Context, dir string) (string, error)
	Close() error
}

// EnvVar manages environment variables with typed getters.
type EnvVar interface {
	// LoadEnvFiles loads environment variables from specific files.
	LoadEnvFiles(paths []string) error
	// Get returns the value and if it exists.
	Get(key string) (string, bool)
	// GetOr returns the value or default if not present.
	GetOr(key, def string) string
	// MustGet returns the value or panics if not present.
	MustGet(key string) string
	// GetBoolOr returns the value as boolean or default if not present.
	GetBoolOr(key string, def bool) bool
	// MustGetBool returns the value as a boolean or panics if not present.
	MustGetBool(key string) bool
	// GetIntOr returns the value as an integer or default if not present.
	GetIntOr(key string, def int) int
	// MustGetInt returns the value as an integer or panics if not present.
	MustGetInt(key string) int
	// GetInt64Or returns the value as an int64 or default if not present.
	GetInt64Or(key string, def int64) int64
	// MustGetInt64 returns the value as an int64 or panics if not present.
	MustGetInt64(key string) int64
	// GetUintOr returns the value as a uint or default if not present.
	GetUintOr(key string, def uint) uint
	// MustGetUint returns the value as a uint or panics if not present.
	MustGetUint(key string) uint
	// GetUint64Or returns the value as a uint64 or default if not present.
	GetUint64Or(key string, def uint64) uint64
	// MustGetUint64 returns the value as a uint64 or panics if not present.
	MustGetUint64(key string) uint64
	// GetFloat64Or returns the value as a float64 or default if not present.
	GetFloat64Or(key string, def float64) float64
	// MustGetFloat64 returns the value as a float64 or panics if not present.
	MustGetFloat64(key string) float64
	// MustGetDuration returns the value as a duration or panics if not present.
	MustGetDuration(key string) time.Duration
	// GetDurationOr returns the value as a duration or default if not present.
	GetDurationOr(key string, def time.Duration) time.Duration
	// Bind populates a struct from environment variables.
	Bind(dst any) error
	// MustBind panics on binding errors.
	MustBind(dst any)
	// BindWithPrefix binds with a prefix.
	BindWithPrefix(dst any, prefix string) error
	// MustBindWithPrefix panics on binding errors with prefix.
	MustBindWithPrefix(dst any, prefix string)
	// DumpRedacted returns environment with secrets redacted.
	DumpRedacted() map[string]string
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

// CheckoutSessionRequest describes a hosted checkout request (e.g. Stripe Checkout).
type CheckoutSessionRequest struct {
	Amount     int64             // Minor units (cents)
	Currency   string            // ISO currency, e.g. "eur"
	PriceID    string            // Provider-specific price ID (preferred)
	SuccessURL string            // Where to redirect on success
	CancelURL  string            // Where to redirect on cancellation
	Metadata   map[string]string // Arbitrary key/value for reconciliation
	Mode       string            // "payment" | "subscription"
	Locale     string            // Optional locale code, provider-specific
}

// CheckoutSession represents a provider-created hosted checkout session.
type CheckoutSession struct {
	ID  string
	URL string
}

// WebhookEvent is a generic payment provider webhook payload wrapper.
type WebhookEvent struct {
	ID        string
	Type      string
	CreatedAt time.Time
	Payload   json.RawMessage
}

// Price represents a payment provider price.
type Price struct {
	ID         string
	ProductID  string
	Currency   string
	UnitAmount int64
	Nickname   string
	Metadata   map[string]string
	Active     bool
}

// PaymentProvider defines a hosted checkout + webhook contract.
type PaymentProvider interface {
	CreateCheckoutSession(ctx context.Context, req CheckoutSessionRequest) (CheckoutSession, error)
	ParseWebhook(ctx context.Context, payload []byte, sigHeader string) (WebhookEvent, error)
	ListPrices(ctx context.Context) ([]Price, error)
}

// CustomerInput describes a new billing customer.
type CustomerInput struct {
	Name     string
	Email    string
	Phone    string
	Address  *CustomerAddress
	Metadata map[string]string
}

// CustomerAddress describes a customer's billing address.
type CustomerAddress struct {
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
}

// Customer represents a billing customer.
type Customer struct {
	ID string
}

// SetupIntentInput describes a payment method setup intent request.
type SetupIntentInput struct {
	CustomerID string
	Usage      string
	Metadata   map[string]string
}

// SetupIntent represents a setup intent response.
type SetupIntent struct {
	ID              string
	ClientSecret    string
	CustomerID      string
	PaymentMethodID string
}

// PaymentMethod describes a stored payment method.
type PaymentMethod struct {
	ID       string
	Brand    string
	Last4    string
	ExpMonth int
	ExpYear  int
}

// InvoiceItemInput describes a pending invoice item.
type InvoiceItemInput struct {
	CustomerID     string
	Amount         int64
	Currency       string
	Description    string
	TaxBehavior    string
	Metadata       map[string]string
	IdempotencyKey string
}

// InvoiceItem represents a created invoice item.
type InvoiceItem struct {
	ID string
	InvoiceID string
}

// InvoiceItemUpdate describes a patch for an invoice item.
type InvoiceItemUpdate struct {
	TaxBehavior string
}

// InvoiceInput describes a draft invoice creation request.
type InvoiceInput struct {
	CustomerID       string
	AutoAdvance      bool
	AutomaticTax     bool
	CollectionMethod string
	DueDays          *int
	PendingInvoiceItemsBehavior string
	Metadata         map[string]string
	IdempotencyKey   string
}

// Invoice represents a billing invoice.
type Invoice struct {
	ID               string
	Status           string
	Currency         string
	AmountDue        int64
	AmountPaid       int64
	HostedInvoiceURL string
	CreatedAt        time.Time
	DueDate          *time.Time
	FinalizedAt      *time.Time
	PaidAt           *time.Time
}

// BillingPortalSessionInput describes a customer portal session request.
type BillingPortalSessionInput struct {
	CustomerID string
	ReturnURL  string
	Locale     string
}

// BillingPortalSession represents a customer portal session.
type BillingPortalSession struct {
	ID  string
	URL string
}

// BillingProvider defines billing-related operations for hosted invoicing.
type BillingProvider interface {
	CreateCustomer(ctx context.Context, in CustomerInput) (Customer, error)
	UpdateCustomer(ctx context.Context, customerID string, in CustomerInput) error
	CreateSetupIntent(ctx context.Context, in SetupIntentInput) (SetupIntent, error)
	SetCustomerDefaultPaymentMethod(ctx context.Context, customerID, paymentMethodID string) error
	RetrievePaymentMethod(ctx context.Context, paymentMethodID string) (PaymentMethod, error)
	CreateInvoiceItem(ctx context.Context, in InvoiceItemInput) (InvoiceItem, error)
	RetrieveInvoiceItem(ctx context.Context, invoiceItemID string) (InvoiceItem, error)
	UpdateInvoiceItem(ctx context.Context, invoiceItemID string, in InvoiceItemUpdate) (InvoiceItem, error)
	CreateInvoice(ctx context.Context, in InvoiceInput) (Invoice, error)
	FinalizeInvoice(ctx context.Context, invoiceID string) (Invoice, error)
	PayInvoice(ctx context.Context, invoiceID string) (Invoice, error)
	RetrieveInvoice(ctx context.Context, invoiceID string) (Invoice, error)
	CreateBillingPortalSession(ctx context.Context, in BillingPortalSessionInput) (BillingPortalSession, error)
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

// VersionInfo describes application build information.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Middleware defines the interface for middlewares.
type Middleware interface {
	Middleware() func(http.Handler) http.Handler
}
