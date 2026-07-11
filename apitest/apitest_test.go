package apitest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aatuh/api-toolkit/v4/fielderrors"
	"github.com/aatuh/api-toolkit/v4/httpx"
)

func TestProblemAndValidationAssertions(t *testing.T) {
	recorder := httptest.NewRecorder()
	problem := httpx.WithFieldErrors(httpx.Problem{Title: "Bad Request"}, fielderrors.FieldErrors{{Field: "name", Code: "required", Message: "name is required"}})
	httpx.WriteProblem(recorder, http.StatusBadRequest, problem)
	AssertProblem(t, recorder, http.StatusBadRequest)
	AssertValidationFields(t, recorder, "name")
}

func TestJSONHeaderAndOperationAssertions(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Location", "/operations/1")
	recorder.WriteHeader(http.StatusAccepted)
	AssertOperationAccepted(t, recorder, "/operations/1")

	recorder = httptest.NewRecorder()
	recorder.Header().Set("ETag", `"abc"`)
	_, _ = recorder.WriteString(`{"ok":true}`)
	AssertETag(t, recorder, `"abc"`)
	AssertJSON(t, recorder, map[string]bool{"ok": true})
}
