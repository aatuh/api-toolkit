package telemetry

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

var (
	traceInitOnce     sync.Once
	traceInitErr      error
	traceInitShutdown = func(context.Context) error { return nil }
	traceInitEnabled  bool
)

var errEmptyEndpoint = errors.New("trace endpoint is empty")

// InitTracing initializes OpenTelemetry tracing from the provided config.
// It is safe to call multiple times; only the first call will take effect.
func InitTracing(ctx context.Context, cfg TraceConfig) (func(context.Context) error, bool, error) {
	traceInitOnce.Do(func() {
		otel.SetTextMapPropagator(
			propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
		)

		if !cfg.Enabled || strings.TrimSpace(cfg.Endpoint) == "" {
			otel.SetTracerProvider(noop.NewTracerProvider())
			traceInitEnabled = false
			return
		}

		exporter, err := newExporter(ctx, cfg)
		if err != nil {
			traceInitErr = err
			otel.SetTracerProvider(noop.NewTracerProvider())
			return
		}

		res, err := buildResource(ctx, cfg)
		if err != nil {
			traceInitErr = err
			otel.SetTracerProvider(noop.NewTracerProvider())
			return
		}

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSampler(buildSampler(cfg)),
			sdktrace.WithBatcher(exporter),
		)

		otel.SetTracerProvider(tp)
		traceInitShutdown = tp.Shutdown
		traceInitEnabled = true
	})

	return traceInitShutdown, traceInitEnabled, traceInitErr
}

func newExporter(ctx context.Context, cfg TraceConfig) (sdktrace.SpanExporter, error) {
	protocol := strings.ToLower(strings.TrimSpace(cfg.Protocol))
	if strings.Contains(protocol, "grpc") {
		return newGRPCExporter(ctx, cfg.Endpoint)
	}
	return newHTTPExporter(ctx, cfg.Endpoint)
}

func newHTTPExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, errEmptyEndpoint
	}
	opts := []otlptracehttp.Option{}
	if hasScheme(endpoint) {
		opts = append(opts, otlptracehttp.WithEndpointURL(endpoint))
		if strings.HasPrefix(endpoint, "http://") {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
	} else {
		opts = append(opts, otlptracehttp.WithEndpoint(endpoint), otlptracehttp.WithInsecure())
	}
	return otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
}

func newGRPCExporter(ctx context.Context, endpoint string) (sdktrace.SpanExporter, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, errEmptyEndpoint
	}
	opts := []otlptracegrpc.Option{}
	if host, insecure := grpcEndpoint(endpoint); host != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(host))
		if insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
	} else {
		opts = append(opts, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	}
	return otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
}

func buildResource(ctx context.Context, cfg TraceConfig) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", cfg.ServiceName),
	}
	if strings.TrimSpace(cfg.Environment) != "" {
		attrs = append(attrs, attribute.String("deployment.environment", cfg.Environment))
	}
	return resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attrs...),
	)
}

func buildSampler(cfg TraceConfig) sdktrace.Sampler {
	sampler := strings.ToLower(strings.TrimSpace(cfg.Sampler))
	switch sampler {
	case "always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "traceidratio":
		return sdktrace.TraceIDRatioBased(ratioFromArg(cfg.SamplerArg, cfg.SampleRatio))
	case "parentbased_traceidratio":
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratioFromArg(cfg.SamplerArg, cfg.SampleRatio)))
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	default:
		if cfg.SampleRatio >= 0 {
			return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))
		}
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	}
}

func ratioFromArg(arg string, fallback float64) float64 {
	if arg != "" {
		if ratio, ok := parseRatio(arg); ok {
			return ratio
		}
	}
	if fallback >= 0 {
		return fallback
	}
	return 1
}

func hasScheme(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && u.Scheme != ""
}

func grpcEndpoint(endpoint string) (host string, insecure bool) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" {
		return "", false
	}
	host = u.Host
	if host == "" {
		host = u.Path
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "grpc":
		return host, true
	case "https", "grpcs":
		return host, false
	default:
		return host, false
	}
}
