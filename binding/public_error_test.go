package binding

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/fielderrors"
)

func TestValidationProblemDoesNotExposeUnsafeErrorDetails(t *testing.T) {
	for _, err := range []error{
		errors.New("SELECT * FROM accounts WHERE token='super-secret'"),
		unsafeFieldError{message: "provider token=super-secret"},
	} {
		t.Run(err.Error(), func(t *testing.T) {
			problem := ValidationProblem(err)
			if problem.Detail != "validation failed" {
				t.Fatalf("detail = %q, want generic validation failure", problem.Detail)
			}
			if strings.Contains(problem.Detail, "super-secret") {
				t.Fatalf("detail leaked secret: %q", problem.Detail)
			}
			if _, ok := problem.Ext["validation"]; ok {
				t.Fatalf("unsafe provider fields were exposed: %#v", problem.Ext)
			}
		})
	}
}

func TestWriteValidationProblemDoesNotExposeUnsafeErrorDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteValidationProblem(rec, errors.New("provider token=super-secret"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "super-secret") {
		t.Fatalf("response body leaked secret: %s", body)
	}
	if !strings.Contains(rec.Body.String(), `"validation failed"`) {
		t.Fatalf("response body missing generic detail: %s", rec.Body.String())
	}
}

func TestValidationProblemAllowsExplicitPublicError(t *testing.T) {
	problem := ValidationProblem(publicValidationError("postal code is invalid"))
	if problem.Detail != "postal code is invalid" {
		t.Fatalf("detail = %q", problem.Detail)
	}
}

type unsafeFieldError struct {
	message string
}

func (e unsafeFieldError) Error() string {
	return e.message
}

func (e unsafeFieldError) FieldErrors() fielderrors.FieldErrors {
	return fielderrors.FieldErrors{{
		Field:   "token",
		Code:    "invalid",
		Message: e.message,
	}}
}

type publicValidationError string

func (e publicValidationError) Error() string {
	return "internal: " + string(e)
}

func (e publicValidationError) PublicMessage() string {
	return string(e)
}
