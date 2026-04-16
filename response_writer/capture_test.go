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
