//revive:disable:var-naming

package response_writer

import (
	"bytes"
	"net/http"
)

// Capture buffers response headers and body for later replay.
// It intentionally does not preserve optional http.ResponseWriter interfaces
// such as Flusher, Hijacker, Pusher, or ReaderFrom.
type Capture struct {
	header      http.Header
	status      int
	wroteHeader bool
	body        bytes.Buffer
	maxBytes    int64
	overflowed  bool
}

// NewCapture creates a new buffered response writer.
func NewCapture() *Capture {
	return &Capture{
		header: make(http.Header),
		status: http.StatusOK,
	}
}

// NewLimitedCapture creates a buffered response writer with a maximum stored body size.
func NewLimitedCapture(maxBytes int64) *Capture {
	c := NewCapture()
	c.maxBytes = maxBytes
	return c
}

// Header returns the buffered header map.
func (c *Capture) Header() http.Header {
	return c.header
}

// WriteHeader records the status code without writing.
func (c *Capture) WriteHeader(code int) {
	if c == nil {
		return
	}
	if isInformational(code) {
		return
	}
	if c.wroteHeader {
		return
	}
	c.status = code
	c.wroteHeader = true
}

// Write buffers the response body.
func (c *Capture) Write(b []byte) (int, error) {
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

// Status returns the captured status code.
func (c *Capture) Status() int {
	return c.status
}

// BytesWritten returns the number of buffered bytes.
func (c *Capture) BytesWritten() int {
	return c.body.Len()
}

// Body returns a copy of the buffered body.
func (c *Capture) Body() []byte {
	if c == nil || c.body.Len() == 0 {
		return nil
	}
	out := make([]byte, c.body.Len())
	copy(out, c.body.Bytes())
	return out
}

// Overflowed reports whether buffered writes exceeded the configured maximum body size.
func (c *Capture) Overflowed() bool {
	return c.overflowed
}

// WriteTo writes the buffered response to the provided ResponseWriter.
func (c *Capture) WriteTo(w http.ResponseWriter) {
	if w == nil {
		return
	}
	copyHeader(w.Header(), c.header)
	w.WriteHeader(c.status)
	_, _ = w.Write(c.body.Bytes())
}

func copyHeader(dst, src http.Header) {
	for k, v := range src {
		out := make([]string, len(v))
		copy(out, v)
		dst[k] = out
	}
}
