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
