package logzap

import (
	"os"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/aatuh/api-toolkit/ports"
)

// ZapLogger adapts zap to the shared.Logger interface.
type ZapLogger struct{ s *zap.SugaredLogger }

// New wraps a zap.Logger as a ports.Logger.
func New(z *zap.Logger) ports.Logger { return &ZapLogger{s: z.Sugar()} }

// NewProduction creates a production logger (JSON, no colors).
func NewProduction() ports.Logger {
	cfg := zap.NewProductionConfig()
	return &ZapLogger{s: buildSplitLogger(cfg).Sugar()}
}

// NewProductionWithLevel creates a production logger with custom level.
func NewProductionWithLevel(level string) ports.Logger {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(parseLevel(level))
	return &ZapLogger{s: buildSplitLogger(cfg).Sugar()}
}

// NewDevelopment creates a human-friendly logger for local dev (console, colors).
func NewDevelopment(level string) ports.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.Level = zap.NewAtomicLevelAt(parseLevel(level))
	return &ZapLogger{s: buildSplitLogger(cfg).Sugar()}
}

// Debug logs a debug message with structured fields.
func (l *ZapLogger) Debug(msg string, kv ...any) { l.s.Debugw(msg, kv...) }

// Info logs an info message with structured fields.
func (l *ZapLogger) Info(msg string, kv ...any) { l.s.Infow(msg, kv...) }

// Warn logs a warning message with structured fields.
func (l *ZapLogger) Warn(msg string, kv ...any) { l.s.Warnw(msg, kv...) }

// Error logs an error message with structured fields.
func (l *ZapLogger) Error(msg string, kv ...any) { l.s.Errorw(msg, kv...) }

func buildSplitLogger(cfg zap.Config) *zap.Logger {
	encoder := buildEncoder(cfg)
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), lowPriority(cfg)),
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stderr), highPriority(cfg)),
	)
	if cfg.Sampling != nil {
		var samplerOpts []zapcore.SamplerOption
		if cfg.Sampling.Hook != nil {
			samplerOpts = append(samplerOpts, zapcore.SamplerHook(cfg.Sampling.Hook))
		}
		core = zapcore.NewSamplerWithOptions(
			core,
			time.Second,
			cfg.Sampling.Initial,
			cfg.Sampling.Thereafter,
			samplerOpts...,
		)
	}
	return zap.New(core, buildOptions(cfg)...)
}

func buildEncoder(cfg zap.Config) zapcore.Encoder {
	switch strings.ToLower(strings.TrimSpace(cfg.Encoding)) {
	case "console":
		return zapcore.NewConsoleEncoder(cfg.EncoderConfig)
	default:
		return zapcore.NewJSONEncoder(cfg.EncoderConfig)
	}
}

func buildOptions(cfg zap.Config) []zap.Option {
	errSink := zapcore.AddSync(os.Stderr)
	if len(cfg.ErrorOutputPaths) > 0 {
		if sink, _, err := zap.Open(cfg.ErrorOutputPaths...); err == nil {
			errSink = sink
		}
	}

	opts := []zap.Option{zap.ErrorOutput(errSink)}
	if cfg.Development {
		opts = append(opts, zap.Development())
	}
	if !cfg.DisableCaller {
		opts = append(opts, zap.AddCaller())
	}
	stackLevel := zapcore.ErrorLevel
	if cfg.Development {
		stackLevel = zapcore.WarnLevel
	}
	if !cfg.DisableStacktrace {
		opts = append(opts, zap.AddStacktrace(stackLevel))
	}
	if len(cfg.InitialFields) > 0 {
		keys := make([]string, 0, len(cfg.InitialFields))
		for key := range cfg.InitialFields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fields := make([]zap.Field, 0, len(cfg.InitialFields))
		for _, key := range keys {
			fields = append(fields, zap.Any(key, cfg.InitialFields[key]))
		}
		opts = append(opts, zap.Fields(fields...))
	}
	return opts
}

func lowPriority(cfg zap.Config) zapcore.LevelEnabler {
	return zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel && cfg.Level.Enabled(lvl)
	})
}

func highPriority(cfg zap.Config) zapcore.LevelEnabler {
	return zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel && cfg.Level.Enabled(lvl)
	})
}

func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn", "warning":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}
