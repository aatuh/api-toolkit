package httpx

import (
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestWriteJSONCheckedEncodingFailureDoesNotCommit(t *testing.T) {
	w := &checkedResponseWriter{}

	err := WriteJSONChecked(w, http.StatusOK, math.Inf(1))

	assertWriteFailure(t, err, ResponseWriteStageEncode, nil)
	if w.status != 0 {
		t.Fatalf("status = %d, want no committed response", w.status)
	}
	if w.writes != 0 {
		t.Fatalf("writes = %d, want 0", w.writes)
	}
}

func TestWriteProblemCheckedEncodingFailureDoesNotCommit(t *testing.T) {
	w := &checkedResponseWriter{}

	err := WriteProblemChecked(w, http.StatusBadRequest, Problem{
		Ext: map[string]any{"invalid": func() {}},
	})

	assertWriteFailure(t, err, ResponseWriteStageEncode, nil)
	if w.status != 0 {
		t.Fatalf("status = %d, want no committed response", w.status)
	}
	if w.writes != 0 {
		t.Fatalf("writes = %d, want 0", w.writes)
	}
}

func TestWriteJSONCheckedReturnsHeaderFailure(t *testing.T) {
	headerErr := errors.New("header writer failed")
	w := &checkedResponseWriter{headerErr: headerErr}

	err := WriteJSONChecked(w, http.StatusCreated, map[string]string{"status": "created"})

	assertWriteFailure(t, err, ResponseWriteStageHeader, headerErr)
	if w.writes != 0 {
		t.Fatalf("writes = %d, want 0", w.writes)
	}
}

func TestWriteJSONCheckedReturnsBodyFailureWithoutLeakingResponseData(t *testing.T) {
	bodyErr := errors.New("token=super-secret")
	w := &checkedResponseWriter{writeErr: bodyErr}

	err := WriteJSONChecked(w, http.StatusCreated, map[string]string{"status": "created"})

	assertWriteFailure(t, err, ResponseWriteStageBody, bodyErr)
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("write error leaked response or transport data: %q", err)
	}
	if w.writes != 1 {
		t.Fatalf("writes = %d, want 1", w.writes)
	}
}

func TestWriteJSONCheckedReturnsShortBodyWrite(t *testing.T) {
	w := &checkedResponseWriter{shortWrite: true}

	err := WriteJSONChecked(w, http.StatusOK, map[string]string{"status": "ok"})

	assertWriteFailure(t, err, ResponseWriteStageBody, io.ErrShortWrite)
}

func TestWriteJSONDoesNotFallbackAfterCommittedBodyFailure(t *testing.T) {
	w := &checkedResponseWriter{writeErr: errors.New("connection closed")}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	if w.writes != 1 {
		t.Fatalf("writes = %d, want exactly one write", w.writes)
	}
}

func TestWriteProblemDoesNotFallbackAfterCommittedBodyFailure(t *testing.T) {
	w := &checkedResponseWriter{writeErr: errors.New("connection closed")}

	WriteProblem(w, http.StatusBadRequest, Problem{Detail: "invalid input"})

	if w.writes != 1 {
		t.Fatalf("writes = %d, want exactly one write", w.writes)
	}
}

func assertWriteFailure(t *testing.T, err error, wantStage ResponseWriteStage, wantCause error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected write error")
	}
	var writeErr *ResponseWriteError
	if !errors.As(err, &writeErr) {
		t.Fatalf("error type = %T, want *ResponseWriteError", err)
	}
	if writeErr.Stage != wantStage {
		t.Fatalf("stage = %q, want %q", writeErr.Stage, wantStage)
	}
	if wantCause != nil && !errors.Is(err, wantCause) {
		t.Fatalf("error does not preserve cause %v: %v", wantCause, err)
	}
}

type checkedResponseWriter struct {
	header     http.Header
	status     int
	writes     int
	headerErr  error
	writeErr   error
	shortWrite bool
}

func (w *checkedResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *checkedResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *checkedResponseWriter) WriteHeaderError(status int) error {
	if w.headerErr != nil {
		return w.headerErr
	}
	w.status = status
	return nil
}

func (w *checkedResponseWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	if w.shortWrite {
		return len(p) - 1, nil
	}
	return len(p), nil
}
