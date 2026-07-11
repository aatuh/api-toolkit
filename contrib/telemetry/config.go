package telemetry

import (
	"strconv"
	"strings"

	"github.com/aatuh/api-toolkit/contrib/v4/config"
)

// TraceConfig holds tracing configuration inputs.
type TraceConfig struct {
	Enabled     bool
	ServiceName string
	Endpoint    string
	Protocol    string
	Environment string
	Sampler     string
	SamplerArg  string
	SampleRatio float64
}

// TraceConfigFromEnv loads tracing configuration from environment variables.
func TraceConfigFromEnv() TraceConfig {
	return LoadTraceConfig(nil)
}

// LoadTraceConfig loads tracing configuration using the shared config loader.
func LoadTraceConfig(loader *config.Loader) TraceConfig {
	if loader == nil {
		loader = config.NewLoader()
	}
	enabled := loader.Bool("OTEL_TRACING_ENABLED", false)
	serviceName := strings.TrimSpace(loader.String("OTEL_SERVICE_NAME", ""))
	if serviceName == "" {
		serviceName = "api"
	}
	endpoint := strings.TrimSpace(loader.String("OTEL_EXPORTER_OTLP_ENDPOINT", ""))
	protocol := strings.TrimSpace(loader.String("OTEL_EXPORTER_OTLP_PROTOCOL", ""))
	environment := strings.TrimSpace(loader.String("APP_ENV", ""))
	if environment == "" {
		environment = strings.TrimSpace(loader.String("ENV", ""))
	}
	sampler := strings.TrimSpace(loader.String("OTEL_TRACES_SAMPLER", ""))
	samplerArg := strings.TrimSpace(loader.String("OTEL_TRACES_SAMPLER_ARG", ""))
	sampleRatio := -1.0
	if sampler == "" {
		if raw := strings.TrimSpace(loader.String("OTEL_SAMPLE_RATIO", "")); raw != "" {
			if ratio, ok := parseRatio(raw); ok {
				sampleRatio = ratio
			}
		}
	}
	return TraceConfig{
		Enabled:     enabled,
		ServiceName: serviceName,
		Endpoint:    endpoint,
		Protocol:    protocol,
		Environment: environment,
		Sampler:     sampler,
		SamplerArg:  samplerArg,
		SampleRatio: sampleRatio,
	}
}

func parseRatio(raw string) (float64, bool) {
	val, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, false
	}
	if val < 0 {
		val = 0
	}
	if val > 1 {
		val = 1
	}
	return val, true
}
