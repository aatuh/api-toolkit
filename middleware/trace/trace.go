package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/aatuh/api-toolkit/v2/httpx/identity"
	"github.com/aatuh/api-toolkit/v2/ports"
)

// W3C Trace Context (traceparent) format:
// version(2)-trace-id(32)-parent-id(16)-flags(2)
// https://www.w3.org/TR/trace-context/

const (
	headerTraceParent = "traceparent"
	headerTraceState  = "tracestate"
	headerTraceID     = "X-Trace-ID"
	headerRequestID   = "X-Request-ID"
)

type ctxKey string

const (
	ctxTraceID ctxKey = "trace.trace_id"
	ctxSpanID  ctxKey = "trace.span_id"
)

// Options controls middleware behaviour.
type Options struct {
	// TrustIncoming strictly validates client-provided traceparent and uses it
	// if valid. When false, the middleware always generates a fresh trace ID.
	TrustIncoming bool
	// SampledFlag defaults to 00 (not sampled). Set to 01 to enable sampling bit.
	SampledFlag byte
	// TraceIDGen overrides the trace id generator.
	TraceIDGen ports.IDGen
	// SpanIDGen overrides the span id generator.
	SpanIDGen ports.IDGen
}

// Middleware attaches trace/span IDs to request context and sets response header.
type Middleware struct {
	opts Options
}

// New constructs a Middleware with sane defaults.
func New(opts Options) (*Middleware, error) {
	return &Middleware{opts: normalizeOptions(opts)}, nil
}

// Middleware implements ports.Middleware by producing the handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	opts := m.opts
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var traceID string

			var traceFlags byte
			var traceState string

			if opts.TrustIncoming {
				if tp := r.Header.Get(headerTraceParent); tp != "" {
					if tid, _, flags, ok := parseTraceParent(tp); ok {
						traceID = tid
						traceFlags = flags
						traceState = strings.TrimSpace(r.Header.Get(headerTraceState))
					}
				}
			}

			if traceID == "" || !isValidTraceID(traceID) {
				traceID = generateTraceID(opts.TraceIDGen)
			}
			// Always create a new span id for this server span.
			spanID := generateSpanID(opts.SpanIDGen)

			// Put into context
			r = r.WithContext(withTrace(r.Context(), traceID, spanID))

			flags := opts.SampledFlag
			if opts.TrustIncoming && traceID != "" {
				flags = traceFlags
			}

			// Best-effort echo of correlation headers for clients and downstreams.
			if w.Header().Get(headerRequestID) == "" {
				if requestID := identity.RequestID(r); requestID != "" {
					w.Header().Set(headerRequestID, requestID)
				}
			}
			w.Header().Set(headerTraceID, traceID)
			w.Header().Set(headerTraceParent, formatTraceParent(traceID, spanID, flags))
			if traceState != "" {
				w.Header().Set(headerTraceState, traceState)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Use preserves the old helper-style API.
// Deprecated: use New(opts).Middleware() instead.
func Use(opts Options) (func(http.Handler) http.Handler, error) {
	mw, err := New(opts)
	if err != nil {
		return nil, err
	}
	return mw.Middleware(), nil
}

// GetTraceID returns the hex-encoded 16-byte trace id if present.
func GetTraceID(r *http.Request) string {
	v, _ := r.Context().Value(ctxTraceID).(string)
	return v
}

// GetSpanID returns the hex-encoded 8-byte span id if present.
func GetSpanID(r *http.Request) string {
	v, _ := r.Context().Value(ctxSpanID).(string)
	return v
}

func withTrace(ctx context.Context, traceID, spanID string) context.Context {
	ctx = context.WithValue(ctx, ctxTraceID, traceID)
	ctx = context.WithValue(ctx, ctxSpanID, spanID)
	return ctx
}

func parseTraceParent(s string) (traceID, parentID string, flags byte, ok bool) {
	// Strict, fast checks to avoid DoS from huge headers
	if len(s) < 55 || len(s) > 200 { // minimal canonical length ~55
		return "", "", 0, false
	}
	parts := strings.Split(s, "-")
	if len(parts) < 4 {
		return "", "", 0, false
	}
	ver, tid, pid, fl := parts[0], parts[1], parts[2], parts[3]
	if len(ver) != 2 || ver != "00" { // accept only version 00
		return "", "", 0, false
	}
	if !isValidTraceID(tid) || !isValidSpanID(pid) || len(fl) != 2 || !isLowerHex(fl) {
		return "", "", 0, false
	}
	val, err := strconv.ParseUint(fl, 16, 8)
	if err != nil {
		return "", "", 0, false
	}
	return tid, pid, byte(val), true
}

func formatTraceParent(traceID, spanID string, flags byte) string {
	flag := hex.EncodeToString([]byte{flags})
	return "00-" + traceID + "-" + spanID + "-" + flag
}

func newTraceID() string { // 16 bytes => 32 hex
	var b [16]byte
	_, _ = rand.Read(b[:])
	// Must not be all zeros; very unlikely, but guard anyway.
	if allZero(b[:]) {
		b[0] = 1
	}
	return hex.EncodeToString(b[:])
}

func newSpanID() string { // 8 bytes => 16 hex
	var b [8]byte
	_, _ = rand.Read(b[:])
	if allZero(b[:]) {
		b[0] = 1
	}
	return hex.EncodeToString(b[:])
}

func generateTraceID(gen ports.IDGen) string {
	if gen != nil {
		if id := normalizeHexID(gen.New()); isValidTraceID(id) {
			return id
		}
	}
	return newTraceID()
}

func generateSpanID(gen ports.IDGen) string {
	if gen != nil {
		if id := normalizeHexID(gen.New()); isValidSpanID(id) {
			return id
		}
	}
	return newSpanID()
}

func normalizeHexID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func isValidTraceID(s string) bool {
	return len(s) == 32 && isLowerHex(s) && !allZeroHex(s)
}

func isValidSpanID(s string) bool {
	return len(s) == 16 && isLowerHex(s) && !allZeroHex(s)
}

func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func allZeroHex(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}

func normalizeOptions(opts Options) Options {
	if opts.SampledFlag != 0x00 && opts.SampledFlag != 0x01 {
		opts.SampledFlag = 0x01
	}
	if opts.TraceIDGen == nil {
		opts.TraceIDGen = traceIDGen{}
	}
	if opts.SpanIDGen == nil {
		opts.SpanIDGen = spanIDGen{}
	}
	return opts
}

type traceIDGen struct{}

func (traceIDGen) New() string { return newTraceID() }

type spanIDGen struct{}

func (spanIDGen) New() string { return newSpanID() }
