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
