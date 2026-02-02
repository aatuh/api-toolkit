package openapi

import (
	"bytes"
	"net/http"
)

type responseCapture struct {
	header      http.Header
	status      int
	wroteHeader bool
	body        bytes.Buffer
	maxBytes    int64
	tooLarge    bool
}

func newResponseCapture(maxBodyBytes int64) *responseCapture {
	return &responseCapture{
		header:   make(http.Header),
		status:   http.StatusOK,
		maxBytes: maxBodyBytes,
	}
}

func (c *responseCapture) Header() http.Header {
	return c.header
}

func (c *responseCapture) WriteHeader(code int) {
	c.status = code
	c.wroteHeader = true
}

func (c *responseCapture) Write(b []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	if c.maxBytes > 0 {
		remaining := c.maxBytes - int64(c.body.Len())
		if remaining <= 0 {
			c.tooLarge = true
			return len(b), nil
		}
		if int64(len(b)) > remaining {
			c.tooLarge = true
			_, _ = c.body.Write(b[:remaining])
			return len(b), nil
		}
	}
	return c.body.Write(b)
}

func (c *responseCapture) Status() int {
	return c.status
}

func (c *responseCapture) Body() []byte {
	return c.body.Bytes()
}

func (c *responseCapture) TooLarge() bool {
	return c.tooLarge
}

func (c *responseCapture) WriteTo(w http.ResponseWriter) {
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
