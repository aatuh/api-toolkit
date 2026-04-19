package logzap

import (
	"bytes"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewWrapsZapLogger(t *testing.T) {
	var buf bytes.Buffer
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&buf),
		zap.DebugLevel,
	)

	logger := New(zap.New(core))
	logger.Info("hello", "key", "value")
	logger.Error("boom", "count", 2)

	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) {
		t.Fatalf("output missing hello message: %s", out)
	}
	if !strings.Contains(out, `"key":"value"`) {
		t.Fatalf("output missing structured field: %s", out)
	}
	if !strings.Contains(out, `"msg":"boom"`) {
		t.Fatalf("output missing error message: %s", out)
	}
	if !strings.Contains(out, `"count":2`) {
		t.Fatalf("output missing numeric field: %s", out)
	}
}
