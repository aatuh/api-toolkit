package ports

// NopLogger is a no-op logger for safe defaults.
type NopLogger struct{}

// Debug implements Logger.
func (NopLogger) Debug(string, ...any) {}

// Info implements Logger.
func (NopLogger) Info(string, ...any) {}

// Warn implements Logger.
func (NopLogger) Warn(string, ...any) {}

// Error implements Logger.
func (NopLogger) Error(string, ...any) {}
