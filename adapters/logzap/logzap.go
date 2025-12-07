package logzap

import (
	"github.com/aatuh/api-toolkit/ports"
	"go.uber.org/zap"
)

// ZapLogger adapts zap to the shared.Logger interface.
type ZapLogger struct{ s *zap.SugaredLogger }

func New(z *zap.Logger) ports.Logger { return &ZapLogger{s: z.Sugar()} }

// NewProduction creates a production logger.
func NewProduction() ports.Logger {
	z, _ := zap.NewProduction()
	return &ZapLogger{s: z.Sugar()}
}

func (l *ZapLogger) Debug(msg string, kv ...any) { l.s.Debugw(msg, kv...) }
func (l *ZapLogger) Info(msg string, kv ...any)  { l.s.Infow(msg, kv...) }
func (l *ZapLogger) Warn(msg string, kv ...any)  { l.s.Warnw(msg, kv...) }
func (l *ZapLogger) Error(msg string, kv ...any) { l.s.Errorw(msg, kv...) }
