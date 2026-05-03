// Package upload decodes multipart upload requests and maps validation failures to Problem Details.
package upload

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/aatuh/api-toolkit/v2/fielderrors"
	"github.com/aatuh/api-toolkit/v2/httpx"
)

const defaultMaxMemory int64 = 32 << 20

// File describes an uploaded multipart file.
type File struct {
	FieldName   string
	Filename    string
	ContentType string
	Size        int64
	Header      textproto.MIMEHeader
	FileHeader  *multipart.FileHeader
}

// Open opens the uploaded file stream.
func (f File) Open() (multipart.File, error) {
	if f.FileHeader == nil {
		return nil, fmt.Errorf("upload file header is missing")
	}
	return f.FileHeader.Open()
}

// Form describes decoded multipart values and files.
type Form struct {
	Values map[string][]string
	Files  map[string][]File
}

// Config configures multipart decoding.
type Config struct {
	MaxMemory           int64
	MaxRequestBytes     int64
	MaxFileBytes        int64
	RequiredFiles       []string
	AllowedContentTypes []string
}

// AllowedContentTypes returns a normalized content-type allowlist for Config.
func AllowedContentTypes(types ...string) []string {
	out := make([]string, 0, len(types))
	for _, value := range types {
		if trimmed := strings.TrimSpace(strings.ToLower(value)); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// MaxFileBytes returns a file-size limit value for Config.
func MaxFileBytes(n int64) int64 {
	return n
}

// DecodeMultipart decodes a multipart form request.
func DecodeMultipart(r *http.Request, config Config) (Form, error) {
	if r == nil || r.Body == nil {
		return Form{}, fieldError("body", "required", "multipart request body is required")
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type"))), "multipart/form-data") {
		return Form{}, fieldError("content_type", "invalid", "Content-Type must be multipart/form-data")
	}
	if config.MaxRequestBytes > 0 {
		body, err := io.ReadAll(io.LimitReader(r.Body, config.MaxRequestBytes+1))
		if err != nil {
			return Form{}, fieldError("body", "read_failed", "multipart request body could not be read")
		}
		if int64(len(body)) > config.MaxRequestBytes {
			return Form{}, fieldError("body", "too_large", "multipart request exceeds maximum size")
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	maxMemory := config.MaxMemory
	if maxMemory <= 0 {
		maxMemory = defaultMaxMemory
	}
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return Form{}, fieldError("body", "invalid_multipart", "multipart form could not be parsed")
	}
	if r.MultipartForm == nil {
		return Form{}, fieldError("body", "invalid_multipart", "multipart form is required")
	}
	form := Form{Values: cloneValues(r.MultipartForm.Value), Files: map[string][]File{}}
	allowed := allowedSet(config.AllowedContentTypes)
	var errs fielderrors.FieldErrors
	for field, headers := range r.MultipartForm.File {
		for _, header := range headers {
			file := File{FieldName: field, Filename: header.Filename, ContentType: header.Header.Get("Content-Type"), Size: header.Size, Header: header.Header, FileHeader: header}
			if config.MaxFileBytes > 0 && header.Size > config.MaxFileBytes {
				errs = append(errs, fieldError(field, "too_large", field+" exceeds maximum file size")...)
			}
			if len(allowed) > 0 && !allowed[strings.ToLower(strings.TrimSpace(file.ContentType))] {
				errs = append(errs, fieldError(field, "content_type", field+" has an unsupported content type")...)
			}
			form.Files[field] = append(form.Files[field], file)
		}
	}
	for _, field := range config.RequiredFiles {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if len(form.Files[field]) == 0 {
			errs = append(errs, fieldError(field, "required", field+" is required")...)
		}
	}
	if len(errs) > 0 {
		return form, errs
	}
	return form, nil
}

// RequireFile returns the first file for a required field.
func RequireFile(form Form, field string) (File, error) {
	field = strings.TrimSpace(field)
	if field == "" || len(form.Files[field]) == 0 {
		return File{}, fieldError(field, "required", field+" is required")
	}
	return form.Files[field][0], nil
}

// ValidationProblem maps upload validation errors to Problem Details.
func ValidationProblem(err error) httpx.Problem {
	problem := httpx.Problem{Type: httpx.DefaultTypeURI(httpx.TypeValidation), Title: http.StatusText(http.StatusBadRequest), Detail: "validation failed"}
	var provider fielderrors.Provider
	if errors.As(err, &provider) {
		return httpx.WithFieldErrors(problem, provider.FieldErrors())
	}
	if err != nil {
		problem.Detail = err.Error()
	}
	return problem
}

// WriteValidationProblem writes upload validation errors as Problem Details.
func WriteValidationProblem(w http.ResponseWriter, err error) {
	httpx.WriteProblem(w, http.StatusBadRequest, ValidationProblem(err))
}

func fieldError(field, code, message string) fielderrors.FieldErrors {
	return fielderrors.FieldErrors{{Field: field, Code: code, Message: message}}
}

func cloneValues(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for key, values := range in {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func allowedSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		if trimmed := strings.TrimSpace(strings.ToLower(value)); trimmed != "" {
			out[trimmed] = true
		}
	}
	return out
}
