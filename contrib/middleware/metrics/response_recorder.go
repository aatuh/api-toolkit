package metrics

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseRecorder {
	if ww, ok := w.(*responseRecorder); ok {
		return ww
	}
	return &responseRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (w *responseRecorder) Status() int {
	if w == nil {
		return 0
	}
	return w.status
}

func (w *responseRecorder) BytesWritten() int {
	if w == nil {
		return 0
	}
	return w.bytes
}

func (w *responseRecorder) Committed() bool {
	if w == nil {
		return false
	}
	return w.wroteHeader || w.bytes > 0
}

func (w *responseRecorder) Unwrap() http.ResponseWriter {
	if w == nil {
		return nil
	}
	return w.ResponseWriter
}

func (w *responseRecorder) WriteHeader(code int) {
	if w == nil {
		return
	}
	if isInformationalStatus(code) {
		if w.wroteHeader {
			return
		}
		w.status = code
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseRecorder) Write(b []byte) (int, error) {
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

func (w *responseRecorder) Flush() {
	if w == nil {
		return
	}
	if !w.wroteHeader && !isInformationalStatus(w.status) {
		w.WriteHeader(http.StatusOK)
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w == nil {
		return nil, nil, http.ErrNotSupported
	}
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

func (w *responseRecorder) Push(target string, opts *http.PushOptions) error {
	if w == nil {
		return http.ErrNotSupported
	}
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

func (w *responseRecorder) ReadFrom(r io.Reader) (int64, error) {
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

func isInformationalStatus(code int) bool {
	return code >= 100 && code < 200
}
