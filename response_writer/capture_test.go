package response_writer

import (
	"io"
	"net/http"
	"testing"
)

func TestCaptureDoesNotExposeOptionalResponseWriterInterfaces(t *testing.T) {
	capture := NewCapture()

	if _, ok := any(capture).(http.Flusher); ok {
		t.Fatal("capture should not implement http.Flusher")
	}
	if _, ok := any(capture).(http.Hijacker); ok {
		t.Fatal("capture should not implement http.Hijacker")
	}
	if _, ok := any(capture).(http.Pusher); ok {
		t.Fatal("capture should not implement http.Pusher")
	}
	if _, ok := any(capture).(io.ReaderFrom); ok {
		t.Fatal("capture should not implement io.ReaderFrom")
	}
}

func TestLimitedCaptureMarksOverflowWithoutGrowingBuffer(t *testing.T) {
	capture := NewLimitedCapture(4)

	n, err := capture.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if n != 6 {
		t.Fatalf("expected full write count, got %d", n)
	}
	if !capture.Overflowed() {
		t.Fatal("expected capture to report overflow")
	}
	if got := string(capture.Body()); got != "abcd" {
		t.Fatalf("expected body to stop at limit, got %q", got)
	}
}

func TestCaptureIgnoresRepeatedFinalWriteHeader(t *testing.T) {
	capture := NewCapture()

	capture.WriteHeader(http.StatusAccepted)
	capture.WriteHeader(http.StatusInternalServerError)

	if got := capture.Status(); got != http.StatusAccepted {
		t.Fatalf("status = %d", got)
	}
}

func TestCaptureBodyReturnsImmutableCopy(t *testing.T) {
	capture := NewCapture()
	if _, err := capture.Write([]byte("abc")); err != nil {
		t.Fatalf("write: %v", err)
	}

	body := capture.Body()
	body[0] = 'z'

	if got := string(capture.Body()); got != "abc" {
		t.Fatalf("capture body changed to %q", got)
	}
}
