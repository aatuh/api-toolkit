package telemetry

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestLoadTraceConfigDefaults(t *testing.T) {
	setTraceEnv(t, map[string]string{})

	cfg := LoadTraceConfig(nil)
	if cfg.Enabled {
		t.Fatal("expected tracing to be disabled by default")
	}
	if cfg.ServiceName != "api" {
		t.Fatalf("service name = %q, want api", cfg.ServiceName)
	}
	if cfg.Endpoint != "" {
		t.Fatalf("endpoint = %q", cfg.Endpoint)
	}
	if cfg.Protocol != "" {
		t.Fatalf("protocol = %q", cfg.Protocol)
	}
	if cfg.Environment != "" {
		t.Fatalf("environment = %q", cfg.Environment)
	}
	if cfg.Sampler != "" {
		t.Fatalf("sampler = %q", cfg.Sampler)
	}
	if cfg.SamplerArg != "" {
		t.Fatalf("sampler arg = %q", cfg.SamplerArg)
	}
	if cfg.SampleRatio != -1 {
		t.Fatalf("sample ratio = %v, want -1", cfg.SampleRatio)
	}
}

func TestLoadTraceConfigPrefersAppEnvAndClampsSampleRatio(t *testing.T) {
	setTraceEnv(t, map[string]string{
		"OTEL_TRACING_ENABLED":        "true",
		"OTEL_SERVICE_NAME":           " billing-api ",
		"OTEL_EXPORTER_OTLP_ENDPOINT": " https://collector:4318 ",
		"OTEL_EXPORTER_OTLP_PROTOCOL": " http/protobuf ",
		"APP_ENV":                     " production ",
		"ENV":                         " staging ",
		"OTEL_SAMPLE_RATIO":           "1.5",
	})

	cfg := LoadTraceConfig(nil)
	if !cfg.Enabled {
		t.Fatal("expected tracing to be enabled")
	}
	if cfg.ServiceName != "billing-api" {
		t.Fatalf("service name = %q", cfg.ServiceName)
	}
	if cfg.Endpoint != "https://collector:4318" {
		t.Fatalf("endpoint = %q", cfg.Endpoint)
	}
	if cfg.Protocol != "http/protobuf" {
		t.Fatalf("protocol = %q", cfg.Protocol)
	}
	if cfg.Environment != "production" {
		t.Fatalf("environment = %q", cfg.Environment)
	}
	if cfg.SampleRatio != 1 {
		t.Fatalf("sample ratio = %v, want 1", cfg.SampleRatio)
	}
}

func TestLoadTraceConfigIgnoresLegacySampleRatioWhenSamplerIsExplicit(t *testing.T) {
	setTraceEnv(t, map[string]string{
		"OTEL_TRACING_ENABLED": "true",
		"OTEL_TRACES_SAMPLER":  "always_off",
		"OTEL_SAMPLE_RATIO":    "0.25",
	})

	cfg := LoadTraceConfig(nil)
	if cfg.Sampler != "always_off" {
		t.Fatalf("sampler = %q", cfg.Sampler)
	}
	if cfg.SampleRatio != -1 {
		t.Fatalf("sample ratio = %v, want -1", cfg.SampleRatio)
	}
}

func TestParseRatio(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		raw    string
		want   float64
		wantOK bool
	}{
		{name: "exact", raw: "0.25", want: 0.25, wantOK: true},
		{name: "negative clamps to zero", raw: "-1", want: 0, wantOK: true},
		{name: "above one clamps", raw: "2", want: 1, wantOK: true},
		{name: "invalid", raw: "not-a-number", want: 0, wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseRatio(tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("ratio = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasScheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		endpoint string
		want     bool
	}{
		{endpoint: "https://collector:4318", want: true},
		{endpoint: "grpc://collector:4317", want: true},
		{endpoint: "collector:4317", want: false},
		{endpoint: "localhost", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.endpoint, func(t *testing.T) {
			t.Parallel()

			if got := hasScheme(tc.endpoint); got != tc.want {
				t.Fatalf("hasScheme(%q) = %v, want %v", tc.endpoint, got, tc.want)
			}
		})
	}
}

func TestGRPCEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		endpoint     string
		wantHost     string
		wantInsecure bool
	}{
		{name: "http endpoint", endpoint: "http://collector:4317", wantHost: "collector:4317", wantInsecure: true},
		{name: "https endpoint", endpoint: "https://collector:4317", wantHost: "collector:4317", wantInsecure: false},
		{name: "grpc endpoint", endpoint: "grpc://collector:4317", wantHost: "collector:4317", wantInsecure: true},
		{name: "no scheme", endpoint: "collector:4317", wantHost: "", wantInsecure: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, insecure := grpcEndpoint(tc.endpoint)
			if host != tc.wantHost || insecure != tc.wantInsecure {
				t.Fatalf("grpcEndpoint(%q) = (%q, %v), want (%q, %v)", tc.endpoint, host, insecure, tc.wantHost, tc.wantInsecure)
			}
		})
	}
}

func TestExporterHelpersRequireEndpoint(t *testing.T) {
	t.Parallel()

	if _, err := newHTTPExporter(context.Background(), " "); !errors.Is(err, errEmptyEndpoint) {
		t.Fatalf("newHTTPExporter() error = %v", err)
	}
	if _, err := newGRPCExporter(context.Background(), " "); !errors.Is(err, errEmptyEndpoint) {
		t.Fatalf("newGRPCExporter() error = %v", err)
	}
}

func TestInitTracingUsesNoopProviderWhenDisabled(t *testing.T) {
	resetTracingState()

	shutdown, enabled, err := InitTracing(context.Background(), TraceConfig{Enabled: false, Endpoint: "https://collector:4318"})
	if err != nil {
		t.Fatalf("InitTracing() error = %v", err)
	}
	if enabled {
		t.Fatal("expected tracing disabled")
	}
	if shutdown == nil {
		t.Fatal("expected shutdown func")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error = %v", err)
	}
}

func TestInitTracingFailsWhenEnabledWithoutEndpoint(t *testing.T) {
	resetTracingState()

	shutdown, enabled, err := InitTracing(context.Background(), TraceConfig{Enabled: true, Endpoint: " "})
	if !errors.Is(err, errEmptyEndpoint) {
		t.Fatalf("InitTracing() error = %v, want %v", err, errEmptyEndpoint)
	}
	if enabled {
		t.Fatal("expected tracing disabled")
	}
	if shutdown == nil {
		t.Fatal("expected shutdown func")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error = %v", err)
	}
}

func TestWrapHTTPTransportAndClient(t *testing.T) {
	t.Parallel()

	if got := WrapHTTPTransport(nil); got == nil {
		t.Fatal("expected wrapped default transport")
	}

	client := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   3 * time.Second,
	}
	wrapped := WrapHTTPClient(client)
	if wrapped == client {
		t.Fatal("expected wrapped client copy")
	}
	if wrapped.Timeout != 3*time.Second {
		t.Fatalf("timeout = %s", wrapped.Timeout)
	}
	if wrapped.Transport == nil {
		t.Fatal("expected wrapped transport")
	}
	if wrapped.Transport == client.Transport {
		t.Fatal("expected transport wrapper around base transport")
	}
}

func TestWrapHTTPClientNilUsesSafeTimeout(t *testing.T) {
	t.Parallel()

	wrapped := WrapHTTPClient(nil)
	if wrapped == nil {
		t.Fatal("expected client")
	}
	if wrapped.Timeout != defaultHTTPClientTimeout {
		t.Fatalf("timeout = %s, want %s", wrapped.Timeout, defaultHTTPClientTimeout)
	}
	if wrapped.Transport == nil {
		t.Fatal("expected wrapped transport")
	}
}

func resetTracingState() {
	traceInitOnce = sync.Once{}
	traceInitErr = nil
	traceInitShutdown = func(context.Context) error { return nil }
	traceInitEnabled = false
}

func setTraceEnv(t *testing.T, overrides map[string]string) {
	t.Helper()

	values := map[string]string{
		"OTEL_TRACING_ENABLED":        "false",
		"OTEL_SERVICE_NAME":           "",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "",
		"OTEL_EXPORTER_OTLP_PROTOCOL": "",
		"APP_ENV":                     "",
		"ENV":                         "",
		"OTEL_TRACES_SAMPLER":         "",
		"OTEL_TRACES_SAMPLER_ARG":     "",
		"OTEL_SAMPLE_RATIO":           "",
	}
	for key, value := range overrides {
		values[key] = value
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
}
