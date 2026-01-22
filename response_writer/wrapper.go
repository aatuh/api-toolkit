//revive:disable:var-naming

package response_writer

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

// Writer wraps an http.ResponseWriter and tracks status/bytes.
// It forwards optional interfaces (Flusher, Hijacker, Pusher, ReaderFrom).
type Writer struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

// Wrap returns a response writer wrapper that tracks status and bytes.
func Wrap(w http.ResponseWriter) *Writer {
	if ww, ok := w.(*Writer); ok {
		return ww
	}
	return &Writer{ResponseWriter: w, status: http.StatusOK}
}

// Status returns the last written status code.
func (w *Writer) Status() int {
	if w == nil {
		return 0
	}
	return w.status
}

// BytesWritten returns the number of bytes written.
func (w *Writer) BytesWritten() int {
	if w == nil {
		return 0
	}
	return w.bytes
}

// Unwrap returns the underlying http.ResponseWriter.
func (w *Writer) Unwrap() http.ResponseWriter {
	if w == nil {
		return nil
	}
	return w.ResponseWriter
}

// WriteHeader captures the status code and forwards to the underlying writer.
func (w *Writer) WriteHeader(code int) {
	if w == nil {
		return
	}
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

// Write writes the response body and tracks byte count.
func (w *Writer) Write(b []byte) (int, error) {
	if w == nil {
		return 0, http.ErrNotSupported
	}
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Flush forwards the flush if supported.
func (w *Writer) Flush() {
	if w == nil {
		return
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards hijacking if supported.
func (w *Writer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w == nil {
		return nil, nil, http.ErrNotSupported
	}
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push forwards server push if supported.
func (w *Writer) Push(target string, opts *http.PushOptions) error {
	if w == nil {
		return http.ErrNotSupported
	}
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

// ReadFrom forwards streaming reads if supported.
func (w *Writer) ReadFrom(r io.Reader) (int64, error) {
	if w == nil {
		return 0, http.ErrNotSupported
	}
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(r)
		w.bytes += int(n)
		return n, err
	}
	n, err := io.Copy(w.ResponseWriter, r)
	w.bytes += int(n)
	return n, err
}
