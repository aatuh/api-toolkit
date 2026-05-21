package requestlog

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
)

var errRequestLogHijacked = errors.New("requestlog hijacked")

type optionalResponseWriter struct {
	header       http.Header
	statuses     []int
	body         bytes.Buffer
	flushed      bool
	pushedTarget string
	readFrom     bool
	hijacked     bool
}

func newOptionalResponseWriter() *optionalResponseWriter {
	return &optionalResponseWriter{header: make(http.Header)}
}

func (w *optionalResponseWriter) Header() http.Header {
	return w.header
}

func (w *optionalResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *optionalResponseWriter) WriteHeader(status int) {
	w.statuses = append(w.statuses, status)
}

func (w *optionalResponseWriter) Flush() {
	w.flushed = true
}

func (w *optionalResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	return nil, nil, errRequestLogHijacked
}

func (w *optionalResponseWriter) Push(target string, _ *http.PushOptions) error {
	w.pushedTarget = target
	return nil
}

func (w *optionalResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	w.readFrom = true
	return w.body.ReadFrom(r)
}

func TestResponseRecorderTracksInformationalAndFinalStatuses(t *testing.T) {
	base := newOptionalResponseWriter()
	rec := wrapResponseWriter(base)

	rec.WriteHeader(http.StatusEarlyHints)
	if got := rec.Status(); got != http.StatusEarlyHints {
		t.Fatalf("informational status = %d, want %d", got, http.StatusEarlyHints)
	}
	if rec.Committed() {
		t.Fatal("informational status should not commit the recorder")
	}

	rec.WriteHeader(http.StatusCreated)
	rec.WriteHeader(http.StatusInternalServerError)
	if got := rec.Status(); got != http.StatusCreated {
		t.Fatalf("final status = %d, want %d", got, http.StatusCreated)
	}
	if !rec.Committed() {
		t.Fatal("final status should commit the recorder")
	}
	if len(base.statuses) != 2 {
		t.Fatalf("underlying statuses = %#v, want informational and first final", base.statuses)
	}
}

func TestResponseRecorderWriteReadFromAndOptionalInterfaces(t *testing.T) {
	base := newOptionalResponseWriter()
	rec := wrapResponseWriter(base)

	n, err := rec.Write([]byte("abc"))
	if err != nil || n != 3 {
		t.Fatalf("write n=%d err=%v", n, err)
	}
	if rec.Status() != http.StatusOK || rec.BytesWritten() != 3 || !rec.Committed() {
		t.Fatalf("write status=%d bytes=%d committed=%v", rec.Status(), rec.BytesWritten(), rec.Committed())
	}

	rec = wrapResponseWriter(newOptionalResponseWriter())
	n64, err := rec.ReadFrom(bytes.NewBufferString("defg"))
	if err != nil || n64 != 4 {
		t.Fatalf("readfrom n=%d err=%v", n64, err)
	}
	base = rec.Unwrap().(*optionalResponseWriter)
	if !base.readFrom || rec.BytesWritten() != 4 || rec.Status() != http.StatusOK {
		t.Fatalf("readfrom forwarded=%v bytes=%d status=%d", base.readFrom, rec.BytesWritten(), rec.Status())
	}

	rec.Flush()
	if !base.flushed {
		t.Fatal("expected flush to forward")
	}
	if err := rec.Push("/asset.js", nil); err != nil {
		t.Fatalf("push: %v", err)
	}
	if base.pushedTarget != "/asset.js" {
		t.Fatalf("pushed target = %q", base.pushedTarget)
	}
	if _, _, err := rec.Hijack(); !errors.Is(err, errRequestLogHijacked) || !base.hijacked {
		t.Fatalf("hijack err=%v forwarded=%v", err, base.hijacked)
	}
}

func TestResponseRecorderNilAndUnsupportedOptionalInterfaces(t *testing.T) {
	var nilRecorder *responseRecorder
	if nilRecorder.Status() != 0 || nilRecorder.BytesWritten() != 0 || nilRecorder.Committed() || nilRecorder.Unwrap() != nil {
		t.Fatal("nil recorder should expose zero values")
	}
	nilRecorder.WriteHeader(http.StatusAccepted)
	nilRecorder.Flush()
	if _, err := nilRecorder.Write([]byte("x")); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("nil write error = %v", err)
	}
	if _, _, err := nilRecorder.Hijack(); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("nil hijack error = %v", err)
	}
	if err := nilRecorder.Push("/", nil); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("nil push error = %v", err)
	}
	if _, err := nilRecorder.ReadFrom(bytes.NewBufferString("x")); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("nil readfrom error = %v", err)
	}

	rec := wrapResponseWriter(&basicResponseWriter{header: make(http.Header)})
	if _, _, err := rec.Hijack(); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("unsupported hijack error = %v", err)
	}
	if err := rec.Push("/", nil); !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("unsupported push error = %v", err)
	}
}

type basicResponseWriter struct {
	header http.Header
	body   bytes.Buffer
}

func (w *basicResponseWriter) Header() http.Header {
	return w.header
}

func (w *basicResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *basicResponseWriter) WriteHeader(int) {}
