package trace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// W3C Trace Context (traceparent) format:
// version(2)-trace-id(32)-parent-id(16)-flags(2)
// https://www.w3.org/TR/trace-context/

const (
	headerTraceParent = "traceparent"
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
	// SampledFlag defaults to 01 (sampled). Set to 00 to turn off sampling bit.
	SampledFlag byte
}

// Middleware attaches trace/span IDs to request context and sets response header.
func Middleware(opts Options) func(http.Handler) http.Handler {
	if opts.SampledFlag != 0x00 && opts.SampledFlag != 0x01 {
		opts.SampledFlag = 0x01
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var traceID string

			if opts.TrustIncoming {
				if tp := r.Header.Get(headerTraceParent); tp != "" {
					if tid, _, _, ok := parseTraceParent(tp); ok {
						traceID = tid
					}
				}
			}

			if traceID == "" || !isValidTraceID(traceID) {
				traceID = newTraceID()
			}
			// Always create a new span id for this server span.
			spanID := newSpanID()

			// Put into context
			r = r.WithContext(withTrace(r.Context(), traceID, spanID))

			// Best-effort echo of traceparent for clients and downstreams
			w.Header().Set(headerTraceParent, formatTraceParent(traceID, spanID, opts.SampledFlag))

			next.ServeHTTP(w, r)
		})
	}
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
	if fl == "00" {
		flags = 0x00
	} else {
		flags = 0x01
	}
	return tid, pid, flags, true
}

func formatTraceParent(traceID, spanID string, sampled byte) string {
	flag := "00"
	if sampled == 0x01 {
		flag = "01"
	}
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
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
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
