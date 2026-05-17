package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aatuh/api-toolkit/v3/fielderrors"
)

func TestWriteProblemGolden(t *testing.T) {
	rec := httptest.NewRecorder()
	p := Problem{
		Title:  "Bad Request",
		Detail: "invalid input",
	}
	p.With("error_code", "invalid_input")
	p.With("hint", "check payload")
	p.With("status", "ignored")

	WriteProblem(rec, http.StatusBadRequest, p)

	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("expected problem+json content type, got %q", ct)
	}

	got := decodeJSON(t, rec.Body.Bytes())
	want := readGoldenJSON(t, "problem.json")

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected problem body\nwant: %#v\ngot: %#v", want, got)
	}
}

func TestWriteProblemWithFieldErrorsGolden(t *testing.T) {
	rec := httptest.NewRecorder()
	errs := fielderrors.FieldErrors{
		{Field: "name", Code: "required", Message: "name is required"},
		{Field: "age", Code: "min", Message: "age must be >= 18"},
	}
	WriteProblemWithFieldErrors(rec, http.StatusBadRequest, Problem{
		Title:  "Bad Request",
		Detail: "validation failed",
	}, errs)

	got := decodeJSON(t, rec.Body.Bytes())
	want := readGoldenJSON(t, "problem_with_fields.json")

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected field error body\nwant: %#v\ngot: %#v", want, got)
	}
}

func TestProblemFromErrorDoesNotLeak(t *testing.T) {
	p, status := ProblemFromError(errors.New("secret failure"))
	if status != http.StatusInternalServerError {
		t.Fatalf("expected 500 status, got %d", status)
	}
	if p.Detail != "internal server error" {
		t.Fatalf("expected generic detail, got %q", p.Detail)
	}
}

func TestProblemFromErrorTypes(t *testing.T) {
	p, status := ProblemFromError(ErrUnauthorized)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 status, got %d", status)
	}
	if p.Type != DefaultTypeURI(TypeUnauthorized) {
		t.Fatalf("expected unauthorized type, got %q", p.Type)
	}

	p, status = ProblemFromError(fielderrors.FieldErrors{
		{Field: "name", Code: "required", Message: "name is required"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 status, got %d", status)
	}
	if p.Type != DefaultTypeURI(TypeValidation) {
		t.Fatalf("expected validation type, got %q", p.Type)
	}
}

func TestProblemFromErrorStrictRedacts(t *testing.T) {
	err := &HTTPError{
		Status: http.StatusInternalServerError,
		Detail: "db exploded",
		Err:    errors.New("secret"),
	}
	p, status := ProblemFromErrorStrict(err)
	if status != http.StatusInternalServerError {
		t.Fatalf("expected 500 status, got %d", status)
	}
	if p.Detail != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("expected redacted detail, got %q", p.Detail)
	}

	err = &HTTPError{
		Status: http.StatusBadRequest,
		Detail: "invalid payload",
	}
	p, status = ProblemFromErrorStrict(err)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 status, got %d", status)
	}
	if p.Detail != "invalid payload" {
		t.Fatalf("expected detail preserved, got %q", p.Detail)
	}
}

func readGoldenJSON(t *testing.T, name string) any {
	t.Helper()
	path := filepath.Join("testdata", name)
	// #nosec G304 -- testdata file names are controlled in tests.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	return decodeJSON(t, data)
}

func decodeJSON(t *testing.T, data []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return out
}
