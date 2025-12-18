package logzap

import (
	"strings"

	"github.com/aatuh/api-toolkit/ports"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLogger adapts zap to the shared.Logger interface.
type ZapLogger struct{ s *zap.SugaredLogger }

func New(z *zap.Logger) ports.Logger { return &ZapLogger{s: z.Sugar()} }

// NewProduction creates a production logger (JSON, no colors).
func NewProduction() ports.Logger {
	z, _ := zap.NewProduction()
	return &ZapLogger{s: z.Sugar()}
}

// NewProductionWithLevel creates a production logger with custom level.
func NewProductionWithLevel(level string) ports.Logger {
	cfg := zap.NewProductionConfig()
	if lvl, err := parseLevel(level); err == nil {
		cfg.Level = zap.NewAtomicLevelAt(lvl)
	}
	z, _ := cfg.Build()
	return &ZapLogger{s: z.Sugar()}
}

// NewDevelopment creates a human-friendly logger for local dev (console, colors).
func NewDevelopment(level string) ports.Logger {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	if lvl, err := parseLevel(level); err == nil {
		cfg.Level = zap.NewAtomicLevelAt(lvl)
	}
	z, _ := cfg.Build()
	return &ZapLogger{s: z.Sugar()}
}

func (l *ZapLogger) Debug(msg string, kv ...any) { l.s.Debugw(msg, kv...) }
func (l *ZapLogger) Info(msg string, kv ...any)  { l.s.Infow(msg, kv...) }
func (l *ZapLogger) Warn(msg string, kv ...any)  { l.s.Warnw(msg, kv...) }
func (l *ZapLogger) Error(msg string, kv ...any) { l.s.Errorw(msg, kv...) }

func parseLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zap.DebugLevel, nil
	case "info":
		return zap.InfoLevel, nil
	case "warn", "warning":
		return zap.WarnLevel, nil
	case "error":
		return zap.ErrorLevel, nil
	default:
		return zap.InfoLevel, nil
	}
}
