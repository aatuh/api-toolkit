package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/httpx"
	"github.com/aatuh/api-toolkit/ports"
	"github.com/aatuh/api-toolkit/response_writer"
)

// KeyFunc extracts an idempotency key from the request.
type KeyFunc func(*http.Request) string

// HashFunc computes a request hash used to detect key reuse with different payloads.
type HashFunc func(*http.Request, []byte) (string, error)

// Options configures the idempotency middleware.
type Options struct {
	Store               ports.IdempotencyStore
	HeaderName          string
	KeyFunc             KeyFunc
	HashFunc            HashFunc
	TTL                 time.Duration
	InFlightTTL         time.Duration
	MaxBodyBytes        int64
	ShouldHandle        func(*http.Request) bool
	ShouldStore         func(status int) bool
	ResponseHeaderAllow []string
	ResponseHeaderDeny  []string
	ReplayHeaderName    string
	FailOpen            bool
	OnError             func(error)
}

// Middleware enforces Idempotency-Key semantics.
type Middleware struct {
	opts Options
}

// New constructs an idempotency middleware.
func New(opts Options) *Middleware {
	if opts.HeaderName == "" {
		opts.HeaderName = "Idempotency-Key"
	}
	if opts.TTL <= 0 {
		opts.TTL = 24 * time.Hour
	}
	if opts.InFlightTTL <= 0 {
		opts.InFlightTTL = 2 * time.Minute
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = 1 << 20
	}
	if opts.HashFunc == nil {
		opts.HashFunc = DefaultHash
	}
	if opts.KeyFunc == nil {
		header := opts.HeaderName
		opts.KeyFunc = func(r *http.Request) string {
			if r == nil {
				return ""
			}
			return strings.TrimSpace(r.Header.Get(header))
		}
	}
	if opts.ShouldHandle == nil {
		opts.ShouldHandle = defaultMethodFilter(http.MethodPost, http.MethodPut, http.MethodPatch)
	}
	if opts.ShouldStore == nil {
		opts.ShouldStore = func(_ int) bool { return true }
	}
	if opts.ReplayHeaderName == "" {
		opts.ReplayHeaderName = "Idempotency-Replayed"
	}
	return &Middleware{opts: opts}
}

// Middleware implements ports.Middleware via Handler adapter.
func (m *Middleware) Middleware() func(http.Handler) http.Handler {
	if m == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler { return m.Handler(next) }
}

// Handler wraps the next handler with idempotency logic.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.opts.Store == nil || !m.opts.ShouldHandle(r) {
			next.ServeHTTP(w, r)
			return
		}
		key := m.opts.KeyFunc(r)
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		body, err := readBody(r, m.opts.MaxBodyBytes)
		if err != nil {
			writeBodyError(w, err)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}

		hash, err := m.opts.HashFunc(r, body)
		if err != nil {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
				Title:  http.StatusText(http.StatusBadRequest),
				Detail: "invalid idempotent request",
			})
			return
		}

		ctx := r.Context()
		record, found, err := m.opts.Store.Get(ctx, key)
		if err != nil {
			if m.opts.OnError != nil {
				m.opts.OnError(err)
			}
			if m.opts.FailOpen {
				next.ServeHTTP(w, r)
				return
			}
			httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.Problem{
				Title:  http.StatusText(http.StatusServiceUnavailable),
				Detail: "idempotency store unavailable",
			})
			return
		}

		if found {
			if record.RequestHash != "" && record.RequestHash != hash {
				writeConflict(w)
				return
			}
			switch record.State {
			case ports.IdempotencyStateCompleted:
				writeReplay(w, key, record, m.opts.ReplayHeaderName)
				return
			case ports.IdempotencyStateInFlight:
				writeInFlight(w, m.opts.InFlightTTL)
				return
			}
		}

		inFlight := ports.IdempotencyRecord{
			State:       ports.IdempotencyStateInFlight,
			RequestHash: hash,
			CreatedAt:   time.Now(),
		}
		reserved, err := m.opts.Store.TryBegin(ctx, key, inFlight, m.opts.InFlightTTL)
		if err != nil {
			if m.opts.OnError != nil {
				m.opts.OnError(err)
			}
			if m.opts.FailOpen {
				next.ServeHTTP(w, r)
				return
			}
			httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.Problem{
				Title:  http.StatusText(http.StatusServiceUnavailable),
				Detail: "idempotency reservation failed",
			})
			return
		}
		if !reserved {
			record, found, err := m.opts.Store.Get(ctx, key)
			if err != nil {
				if m.opts.OnError != nil {
					m.opts.OnError(err)
				}
				if m.opts.FailOpen {
					next.ServeHTTP(w, r)
					return
				}
				httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.Problem{
					Title:  http.StatusText(http.StatusServiceUnavailable),
					Detail: "idempotency lookup failed",
				})
				return
			}
			if found {
				if record.RequestHash != "" && record.RequestHash != hash {
					writeConflict(w)
					return
				}
				switch record.State {
				case ports.IdempotencyStateCompleted:
					writeReplay(w, key, record, m.opts.ReplayHeaderName)
					return
				case ports.IdempotencyStateInFlight:
					writeInFlight(w, m.opts.InFlightTTL)
					return
				}
			}
			// Fallback to processing if we can't observe a stored record.
		}

		capture := response_writer.NewCapture()
		next.ServeHTTP(capture, r)

		record = ports.IdempotencyRecord{
			State:       ports.IdempotencyStateCompleted,
			RequestHash: hash,
			Status:      capture.Status(),
			Header:      filterHeaders(capture.Header(), m.opts.ResponseHeaderAllow, m.opts.ResponseHeaderDeny),
			Body:        append([]byte(nil), capture.Body()...),
			CreatedAt:   time.Now(),
		}
		if m.opts.ShouldStore(capture.Status()) {
			if err := m.opts.Store.Save(ctx, key, record, m.opts.TTL); err != nil && m.opts.OnError != nil {
				m.opts.OnError(err)
			}
		}
		capture.WriteTo(w)
	})
}

// DefaultHash returns a stable SHA-256 hash of the method, path, query, content type, and body.
func DefaultHash(r *http.Request, body []byte) (string, error) {
	if r == nil {
		return "", errors.New("request is nil")
	}
	h := sha256.New()
	_, _ = io.WriteString(h, strings.ToUpper(r.Method))
	h.Write([]byte{0})
	_, _ = io.WriteString(h, r.URL.Path)
	h.Write([]byte{0})
	_, _ = io.WriteString(h, r.URL.RawQuery)
	h.Write([]byte{0})
	_, _ = io.WriteString(h, r.Header.Get("Content-Type"))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func defaultMethodFilter(methods ...string) func(*http.Request) bool {
	allowed := make(map[string]struct{}, len(methods))
	for _, m := range methods {
		allowed[strings.ToUpper(m)] = struct{}{}
	}
	return func(r *http.Request) bool {
		if r == nil {
			return false
		}
		_, ok := allowed[strings.ToUpper(r.Method)]
		return ok
	}
}

var errBodyTooLarge = errors.New("request body too large")

func readBody(r *http.Request, maxBytes int64) ([]byte, error) {
	if r == nil || r.Body == nil {
		return nil, nil
	}
	defer func() {
		_ = r.Body.Close()
	}()
	if maxBytes <= 0 {
		return io.ReadAll(r.Body)
	}
	limited := io.LimitReader(r.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errBodyTooLarge
	}
	return data, nil
}

func writeBodyError(w http.ResponseWriter, err error) {
	if errors.Is(err, errBodyTooLarge) {
		httpx.WriteProblem(w, http.StatusRequestEntityTooLarge, httpx.Problem{
			Title:  http.StatusText(http.StatusRequestEntityTooLarge),
			Detail: "request body too large",
		})
		return
	}
	httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
		Title:  http.StatusText(http.StatusBadRequest),
		Detail: "invalid request body",
	})
}

func writeReplay(w http.ResponseWriter, key string, record ports.IdempotencyRecord, replayHeader string) {
	if replayHeader != "" {
		w.Header().Set(replayHeader, "true")
	}
	if key != "" {
		w.Header().Set("Idempotency-Key", key)
	}
	if record.Header != nil {
		for k, v := range record.Header {
			out := make([]string, len(v))
			copy(out, v)
			w.Header()[k] = out
		}
	}
	status := record.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(record.Body)
}

func writeInFlight(w http.ResponseWriter, ttl time.Duration) {
	if ttl > 0 {
		w.Header().Set("Retry-After", itoa(int(ttl.Seconds())))
	}
	httpx.WriteProblem(w, http.StatusConflict, httpx.Problem{
		Title:  http.StatusText(http.StatusConflict),
		Detail: "idempotency key is already in use",
	})
}

func writeConflict(w http.ResponseWriter) {
	httpx.WriteProblem(w, http.StatusConflict, httpx.Problem{
		Title:  http.StatusText(http.StatusConflict),
		Detail: "idempotency key reuse with different request",
	})
}

func filterHeaders(h http.Header, allow, deny []string) http.Header {
	if h == nil {
		return nil
	}
	clean := make(http.Header, len(h))
	for k, v := range h {
		out := make([]string, len(v))
		copy(out, v)
		clean[k] = out
	}
	clean = applyHeaderDeny(clean, deny)
	clean = applyDefaultDeny(clean)
	if len(allow) > 0 {
		return applyHeaderAllow(clean, allow)
	}
	return clean
}

func applyHeaderAllow(h http.Header, allow []string) http.Header {
	out := make(http.Header, len(allow))
	allowed := make(map[string]struct{}, len(allow))
	for _, key := range allow {
		if key == "" {
			continue
		}
		allowed[http.CanonicalHeaderKey(key)] = struct{}{}
	}
	for k, v := range h {
		if _, ok := allowed[k]; ok {
			out[k] = v
		}
	}
	return out
}

func applyHeaderDeny(h http.Header, deny []string) http.Header {
	if len(deny) == 0 {
		return h
	}
	denied := make(map[string]struct{}, len(deny))
	for _, key := range deny {
		if key == "" {
			continue
		}
		denied[http.CanonicalHeaderKey(key)] = struct{}{}
	}
	for k := range h {
		if _, ok := denied[k]; ok {
			delete(h, k)
		}
	}
	return h
}

func applyDefaultDeny(h http.Header) http.Header {
	// Drop hop-by-hop and size-sensitive headers.
	defaultDeny := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
		"Content-Length",
	}
	return applyHeaderDeny(h, defaultDeny)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var a [12]byte
	i := len(a)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		a[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		a[i] = '-'
	}
	return string(a[i:])
}
