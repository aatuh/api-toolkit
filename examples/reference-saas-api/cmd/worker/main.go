package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"example.com/reference-saas-api/internal/adapters/postgres"
	"example.com/reference-saas-api/internal/app"
	pgxpooladapter "github.com/aatuh/api-toolkit/contrib/v4/adapters/pgxpool"
	webhookdeliverypostgres "github.com/aatuh/api-toolkit/contrib/v4/adapters/webhookdeliverypostgres"
	"github.com/aatuh/api-toolkit/contrib/v4/async"
	metricsmw "github.com/aatuh/api-toolkit/contrib/v4/middleware/metrics"
	"github.com/aatuh/api-toolkit/contrib/v4/webhookdelivery"
	"github.com/aatuh/api-toolkit/v4/ports"
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
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required for worker")
	}
	webhookSecretKey := strings.TrimSpace(os.Getenv("WEBHOOK_SECRET_KEY"))
	if webhookSecretKey == "" {
		return errors.New("WEBHOOK_SECRET_KEY is required for worker")
	}
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := postgres.Open(dbCtx, databaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()
	dbCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
	if err := postgres.CheckRequiredTables(dbCtx, pool, nil); err != nil {
		cancel()
		return err
	}
	cancel()

	postgresPool := &pgxpooladapter.Adapter{Pool: pool}
	widgets := app.NewWidgetServiceWithStore(postgres.NewWidgetStore(pool))
	operationStore := postgres.NewWidgetImportOperationStore(postgresPool)
	outbox := postgres.NewWidgetImportOutbox(postgresPool, operationStore)
	asyncJobs := app.NewAsyncServiceWithStores(widgets, operationStore, outbox)
	metricsRecorder, err := metricsmw.NewPrometheusRecorderChecked(nil, nil)
	if err != nil {
		return err
	}
	asyncHandler, err := async.NewHandlerMux(async.HandlerRoute{Kind: app.WidgetImportJobKind, Handler: asyncJobs})
	if err != nil {
		return err
	}
	webhookEndpointPolicy := webhookdelivery.EndpointPolicy{AllowInsecureHTTP: !strings.EqualFold(os.Getenv("ENV"), "production")}
	webhookStore, err := postgres.NewWebhookStore(postgresPool, webhookSecretKey, webhookEndpointPolicy)
	if err != nil {
		return err
	}
	webhookDeliverer, err := webhookdelivery.NewDeliverer(webhookdelivery.DelivererConfig{
		EndpointPolicy: webhookEndpointPolicy,
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
	asyncRunner, err := async.New(async.Config{
		Store:        outbox,
		Handler:      asyncHandler,
		Logger:       ports.NopLogger{},
		BatchSize:    5,
		Concurrency:  2,
		PollInterval: time.Second,
	})
	if err != nil {
		return err
	}
	return asyncRunner.Run(ctx)
}
