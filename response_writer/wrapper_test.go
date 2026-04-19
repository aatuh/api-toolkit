package response_writer

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriterIgnoresRepeatedFinalWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	w := Wrap(rec)

	w.WriteHeader(http.StatusCreated)
	w.WriteHeader(http.StatusInternalServerError)

	if got := w.Status(); got != http.StatusCreated {
		t.Fatalf("status = %d", got)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("recorder code = %d", rec.Code)
	}
}

func TestWriterAllowsInformationalThenFinalStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	w := Wrap(rec)

	w.WriteHeader(http.StatusEarlyHints)
	if w.Committed() {
		t.Fatal("informational status should not count as final commitment")
	}

	w.WriteHeader(http.StatusCreated)

	if got := w.Status(); got != http.StatusCreated {
		t.Fatalf("status = %d", got)
	}
	if !w.Committed() {
		t.Fatal("expected final status to commit the response")
	}
}

func TestWriterFlushCommitsDefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	w := Wrap(rec)

	w.Flush()

	if !w.Committed() {
		t.Fatal("expected flush to commit the response")
	}
	if got := w.Status(); got != http.StatusOK {
		t.Fatalf("status = %d", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("recorder code = %d", rec.Code)
	}
}
