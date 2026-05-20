package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/auditpostgres"
	"github.com/aatuh/api-toolkit/contrib/v3/adapters/idempotency"
	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v3/adapters/pgxpool"
	webhookdeliverypostgres "github.com/aatuh/api-toolkit/contrib/v3/adapters/webhookdeliverypostgres"
	"github.com/aatuh/api-toolkit/contrib/v3/async"
	"github.com/aatuh/api-toolkit/contrib/v3/bootstrap"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v3/middleware/metrics"
	"github.com/aatuh/api-toolkit/contrib/v3/webhookdelivery"

	objectstorage "example.com/reference-saas-api/internal/adapters/objectstore"
	"example.com/reference-saas-api/internal/adapters/postgres"
	rediscache "example.com/reference-saas-api/internal/adapters/redis"
	"example.com/reference-saas-api/internal/app"
	"github.com/aatuh/api-toolkit/v3/endpoints/version"
	"github.com/aatuh/api-toolkit/v3/ports"

	"example.com/reference-saas-api/internal/httpapi"
)

const (
	appVersion  = "dev"
	buildCommit = "unknown"
	buildDate   = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := httpapi.ConfigFromEnv()
	if err != nil {
		return err
	}
	widgets := app.NewWidgetService()
	tenancy := app.NewTenancyService()
	apiKeys := app.NewAPIKeyService(cfg.APIKeyPepper, tenancy)
	auditLog := app.NewAuditService()
	webhookEndpointPolicy := webhookdelivery.EndpointPolicy{AllowInsecureHTTP: !strings.EqualFold(os.Getenv("ENV"), "production")}
	webhooks := app.NewWebhookServiceWithEndpointPolicy(tenancy, webhookEndpointPolicy)
	objects := app.NewObjectService(tenancy)
	cacheService := app.NewCacheService(nil)

	// api-toolkit:main-service-defaults
	var rateLimiter ports.RateLimiter
	idempotencyStore := ports.IdempotencyStore(idempotency.NewMemoryStore())
	var objectMetadata app.ObjectMetadataStore
	var cacheReadiness httpapi.HealthChecker = cacheService
	var readiness httpapi.HealthChecker = httpapi.HealthCheckFunc(func(context.Context) error { return nil })
	var postgresPool ports.DatabasePool
	var webhookStore *postgres.WebhookStore
	if cfg.DatabaseURL != "" {
		dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		pool, err := postgres.Open(dbCtx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			return err
		}
		dbCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
		if err := postgres.CheckRequiredTables(dbCtx, pool, nil); err != nil {
			cancel()
			pool.Close()
			return err
		}
		cancel()
		defer pool.Close()
		postgresPool = &pgxpooladapter.Adapter{Pool: pool}
		tenancy = app.NewTenancyServiceWithStore(postgres.NewTenancyStore(pool))
		widgets = app.NewWidgetServiceWithStore(postgres.NewWidgetStore(pool))

		// api-toolkit:main-postgres-stores
		apiKeys = app.NewAPIKeyServiceWithStore(cfg.APIKeyPepper, tenancy, postgres.NewAPIKeyStore(pool))
		auditLog = app.NewAuditServiceWithRecorder(auditpostgres.New(postgresPool, auditpostgres.Options{}))
		webhookStore, err = postgres.NewWebhookStore(postgresPool, cfg.WebhookSecretKey, webhookEndpointPolicy)
		if err != nil {
			pool.Close()
			return err
		}
		webhooks = app.NewWebhookServiceWithStoreAndEndpointPolicy(tenancy, webhookStore, webhookEndpointPolicy)
		objectMetadata = postgres.NewObjectStore(pool)
		objects = app.NewObjectService(tenancy)
		readiness = postgres.HealthChecker{Pool: pool}
	}
	if strings.EqualFold(cfg.ObjectStore, "s3") {
		blobStore, err := objectstorage.OpenS3BlobStore(objectstorage.S3Config{
			Endpoint:        cfg.S3Endpoint,
			Region:          cfg.S3Region,
			Bucket:          cfg.S3Bucket,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
		})
		if err != nil {
			return err
		}
		objects = app.NewObjectServiceWithStores(tenancy, objectMetadata, blobStore)
	}
	if strings.EqualFold(cfg.CacheStore, "redis") {
		redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		redisCache, err := rediscache.OpenCache(redisCtx, cfg.RedisAddr)
		cancel()
		if err != nil {
			return err
		}
		defer redisCache.Close()
		cacheService = app.NewCacheService(redisCache.Store)
		cacheReadiness = redisCache
	}
	if strings.EqualFold(cfg.RateLimitStore, "redis") {
		redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		redisRateLimit, err := rediscache.OpenRateLimiter(redisCtx, cfg.RedisAddr, cfg.RateLimitKeyPrefix, 20, 10)
		cancel()
		if err != nil {
			return err
		}
		defer redisRateLimit.Close()
		rateLimiter = redisRateLimit.Limiter
	}
	if strings.EqualFold(cfg.IdempotencyStore, "redis") {
		redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		redisIdempotency, err := rediscache.OpenIdempotencyStore(redisCtx, cfg.RedisAddr, cfg.IdempotencyKeyPrefix)
		cancel()
		if err != nil {
			return err
		}
		defer redisIdempotency.Close()
		idempotencyStore = redisIdempotency.Store
	}
	rateLimitMiddleware, err := httpapi.NewRateLimitMiddleware(rateLimiter)
	if err != nil {
		return err
	}
	idempotencyMiddleware, err := httpapi.NewIdempotencyMiddleware(idempotencyStore)
	if err != nil {
		return err
	}
	metricsRecorder, err := metricsmw.NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		return err
	}
	metricsMiddleware, err := httpapi.NewMetricsMiddleware(metricsRecorder)
	if err != nil {
		return err
	}
	readiness = httpapi.CombineHealthChecks(readiness, cacheReadiness)
	asyncJobs := app.NewAsyncService(widgets)
	asyncStore := async.Store(asyncJobs)
	if postgresPool != nil {
		operationStore := postgres.NewWidgetImportOperationStore(postgresPool)
		outbox := postgres.NewWidgetImportOutbox(postgresPool, operationStore)
		asyncJobs = app.NewAsyncServiceWithStores(widgets, operationStore, outbox)
		asyncStore = outbox
	}
	asyncHandler, err := async.NewHandlerMux(async.HandlerRoute{Kind: app.WidgetImportJobKind, Handler: asyncJobs})
	if err != nil {
		return err
	}
	if webhookStore != nil {
		webhookDeliverer, err := webhookdelivery.NewDeliverer(webhookdelivery.DelivererConfig{
			EndpointPolicy: webhookdelivery.EndpointPolicy{AllowInsecureHTTP: !strings.EqualFold(os.Getenv("ENV"), "production")},
			Metrics:        metricsRecorder,
			UserAgent:      "saas-api-full-webhooks/1",
		})
		if err != nil {
			return err
		}
		webhookHandler, err := webhookdelivery.NewHandler(webhookdelivery.HandlerConfig{
			Endpoints: webhookStore,
			Deliverer: webhookDeliverer,
			Attempts:  webhookStore,
		})
		if err != nil {
			return err
		}
		if err := asyncHandler.Register(webhookdeliverypostgres.OutboxEventType, webhookHandler); err != nil {
			return err
		}
	}
	asyncRunner, err := async.New(async.Config{
		Store:        asyncStore,
		Handler:      asyncHandler,
		Logger:       ports.NopLogger{},
		BatchSize:    5,
		Concurrency:  2,
		PollInterval: time.Second,
	})
	if err != nil {
		return err
	}
	backgroundTasks := []bootstrap.BackgroundTask{}
	if cfg.AsyncWorkerEnabled {
		backgroundTasks = append(backgroundTasks, bootstrap.BackgroundTask{
			Name: "async-worker",
			Run:  asyncRunner.Run,
		})
	}

	openAPIValidation, err := httpapi.NewOpenAPIValidationMiddleware(cfg)
	if err != nil {
		return err
	}
	routerConfig := httpapi.RouterConfig{
		Widgets:  widgets,
		Tenancy:  tenancy,
		APIKeys:  apiKeys,
		Async:    asyncJobs,
		Audit:    auditLog,
		Webhooks: webhooks,
		Objects:  objects,
		Cache:    cacheService,

		Metrics:           metricsMiddleware,
		MetricsHandler:    metricsmw.PrometheusHandler(),
		OpenAPIValidation: openAPIValidation,
		RateLimit:         rateLimitMiddleware,
		Idempotency:       idempotencyMiddleware,
		Readiness:         readiness,
		AdminKey:          cfg.AdminKey,
		// api-toolkit:main-router-config
		APIKey: cfg.APIKey,
	}
	bootstrapRouterConfig, err := bootstrap.DefaultRouterConfigFromEnv(nil)
	if err != nil {
		return err
	}
	bootstrapRouterConfig.Metrics = metricsRecorder
	if rateLimiter != nil {
		bootstrapRouterConfig.RateLimit.Limiter = rateLimiter
	}
	router, err := bootstrap.NewDefaultRouterWithConfig(ports.NopLogger{}, bootstrapRouterConfig)
	if err != nil {
		return err
	}
	service, err := bootstrap.NewAPIService(bootstrap.APIServiceConfig{
		Addr:                    cfg.Addr,
		AdminAddr:               cfg.AdminAddr,
		Log:                     ports.NopLogger{},
		Router:                  router,
		MiddlewareOrder:         bootstrap.StrictSaaSAPIMiddlewareOrder(),
		RequiredMiddlewareOrder: bootstrap.StrictSaaSAPIMiddlewareOrder(),
		RegisterRoutes: func(r ports.HTTPRouter) error {
			return httpapi.RegisterRoutes(r, routerConfig)
		},
		BackgroundTasks: backgroundTasks,
		SystemEndpoints: bootstrap.SystemEndpoints{
			Health:  httpapi.NewHealthHandler(readiness),
			Version: version.NewHandler(version.Config{Info: ports.VersionInfo{Version: appVersion, Commit: buildCommit, Date: buildDate}}),
			Metrics: metricsmw.PrometheusHandler(),
			Pprof:   http.DefaultServeMux,
		},
		Admin: bootstrap.SystemEndpointAdminOptions{
			RequireAdmin: httpapi.RequireAdmin(cfg.AdminKey),
			EnablePprof:  true,
		},
	})
	if err != nil {
		return err
	}
	return service.Start(ctx)
}
