package idempotency

import (
	"bytes"
	"net/http"
)

// responseCapture buffers response headers and body for idempotency replay.
// It intentionally does not preserve optional http.ResponseWriter interfaces
// such as Flusher, Hijacker, Pusher, or ReaderFrom.
type responseCapture struct {
	header      http.Header
	status      int
	wroteHeader bool
	body        bytes.Buffer
	maxBytes    int64
	overflowed  bool
}

func newLimitedResponseCapture(maxBytes int64) *responseCapture {
	return &responseCapture{
		header:   make(http.Header),
		status:   http.StatusOK,
		maxBytes: maxBytes,
	}
}

func (c *responseCapture) Header() http.Header {
	return c.header
}

func (c *responseCapture) WriteHeader(code int) {
	if c == nil {
		return
	}
	if isInformationalStatus(code) {
		return
	}
	if c.wroteHeader {
		return
	}
	c.status = code
	c.wroteHeader = true
}

func isInformationalStatus(code int) bool {
	return code >= 100 && code < 200
}

func (c *responseCapture) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if c.maxBytes > 0 {
		if c.overflowed {
			return len(b), nil
		}
		remaining := c.maxBytes - int64(c.body.Len())
		if remaining <= 0 {
			c.overflowed = true
			return len(b), nil
		}
		if int64(len(b)) > remaining {
			_, _ = c.body.Write(b[:int(remaining)])
			c.overflowed = true
			return len(b), nil
		}
	}
	return c.body.Write(b)
}

func (c *responseCapture) Status() int {
	return c.status
}

func (c *responseCapture) Body() []byte {
	if c == nil || c.body.Len() == 0 {
		return nil
	}
	out := make([]byte, c.body.Len())
	copy(out, c.body.Bytes())
	return out
}

func (c *responseCapture) Overflowed() bool {
	return c.overflowed
}

func (c *responseCapture) WriteTo(w http.ResponseWriter) {
	if w == nil {
		return
	}
	copyResponseHeader(w.Header(), c.header)
	w.WriteHeader(c.status)
	_, _ = w.Write(c.body.Bytes())
}

func copyResponseHeader(dst, src http.Header) {
	for k, v := range src {
		out := make([]string, len(v))
		copy(out, v)
		dst[k] = out
	}
}
