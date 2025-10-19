package requestlog

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/ports"
)

type Middleware struct {
	Log ports.Logger
}

func New(log ports.Logger) *Middleware { return &Middleware{Log: log} }

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &respWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r)

		m.Log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"bytes", ww.bytes,
			"dur_ms", time.Since(start).Milliseconds(),
			"ip", clientIP(r),
			"ua", r.UserAgent(),
			"rid", requestID(r),
		)
	})
}

type respWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *respWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *respWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func clientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func requestID(r *http.Request) string {
	if v := r.Header.Get("X-Request-ID"); v != "" {
		return v
	}
	return ""
}
