package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// ResponseWriteStage identifies the response operation that failed.
type ResponseWriteStage string

const (
	// ResponseWriteStageEncode reports JSON encoding failure before a response is committed.
	ResponseWriteStageEncode ResponseWriteStage = "encode"
	// ResponseWriteStageHeader reports an opt-in response-writer header failure.
	ResponseWriteStageHeader ResponseWriteStage = "header"
	// ResponseWriteStageBody reports a response body write failure.
	ResponseWriteStageBody ResponseWriteStage = "body"
)

// ResponseWriteError reports a failed response write without embedding response
// data in its error text. The underlying transport or encoder error remains
// available through errors.Is and errors.As.
type ResponseWriteError struct {
	// Stage identifies the failed response operation.
	Stage ResponseWriteStage
	// Err is the underlying encoder, header, or body-write error.
	Err error
}

// Error returns a stage-only message that does not expose response data.
func (e *ResponseWriteError) Error() string {
	if e == nil {
		return ""
	}
	if e.Stage == "" {
		return "http response write failed"
	}
	return "http response " + string(e.Stage) + " failed"
}

// Unwrap returns the underlying response-write error.
func (e *ResponseWriteError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// WriteJSON writes a JSON response with a buffered marshal to avoid partial writes.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	if err := WriteJSONChecked(w, status, v); responseWriteStage(err) == ResponseWriteStageEncode {
		WriteProblem(w, http.StatusInternalServerError, Problem{
			Type:   DefaultTypeURI(TypeInternal),
			Title:  http.StatusText(http.StatusInternalServerError),
			Detail: "failed to encode response",
		})
	}
}

// WriteJSONChecked writes a JSON response and returns an error when encoding,
// header commitment, or body writing fails. It encodes before committing
// headers and never writes a fallback response after a failure.
//
// net/http does not expose header-write errors. Response writers that can
// detect one may implement WriteHeaderError(int) error; ordinary response
// writers use WriteHeader and are considered committed once it returns.
func WriteJSONChecked(w http.ResponseWriter, status int, v any) error {
	return writeJSONChecked(w, status, "application/json", v)
}

func writeJSONChecked(w http.ResponseWriter, status int, contentType string, v any) error {
	if status <= 0 {
		status = http.StatusOK
	}
	body, err := json.Marshal(v)
	if err != nil {
		return &ResponseWriteError{Stage: ResponseWriteStageEncode, Err: err}
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if err := writeHeader(w, status); err != nil {
		return &ResponseWriteError{Stage: ResponseWriteStageHeader, Err: err}
	}
	payload := append(body, '\n')
	n, err := w.Write(payload)
	if err == nil && n != len(payload) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return &ResponseWriteError{Stage: ResponseWriteStageBody, Err: err}
	}
	return nil
}

func writeHeader(w http.ResponseWriter, status int) error {
	if writer, ok := w.(interface {
		WriteHeaderError(int) error
	}); ok {
		return writer.WriteHeaderError(status)
	}
	w.WriteHeader(status)
	return nil
}

func responseWriteStage(err error) ResponseWriteStage {
	var writeErr *ResponseWriteError
	if errors.As(err, &writeErr) {
		return writeErr.Stage
	}
	return ""
}
