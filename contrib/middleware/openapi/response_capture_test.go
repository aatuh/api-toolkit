package openapi

import (
	"net/http"
	"testing"
)

func TestResponseCaptureIgnoresRepeatedFinalWriteHeader(t *testing.T) {
	capture := newResponseCapture(0)

	capture.WriteHeader(http.StatusAccepted)
	capture.WriteHeader(http.StatusInternalServerError)

	if got := capture.Status(); got != http.StatusAccepted {
		t.Fatalf("status = %d", got)
	}
}

func TestResponseCaptureNilAndOverflowBranches(t *testing.T) {
	var nilCapture *responseCapture
	nilCapture.WriteHeader(http.StatusOK)

	capture := newResponseCapture(3)
	n, err := capture.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("write error = %v", err)
	}
	if n != len("abcdef") {
		t.Fatalf("write n = %d, want full caller length", n)
	}
	if !capture.TooLarge() {
		t.Fatal("expected capture to mark response too large")
	}
	if body := string(capture.Body()); body != "abc" {
		t.Fatalf("body = %q, want truncated abc", body)
	}

	capture.WriteTo(nil)
}
