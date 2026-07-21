package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"reflect"
	"strings"
	"testing"

	"github.com/aatuh/api-toolkit/v4/fielderrors"
)

func TestDecodeMultipartRejectsInvalidRequests(t *testing.T) {
	if _, err := DecodeMultipart(nil, Config{}); err == nil {
		t.Fatal("expected nil request error")
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", strings.NewReader("body"))
	req.Header.Set("Content-Type", "application/json")
	if _, err := DecodeMultipart(req, Config{}); err == nil {
		t.Fatal("expected content type error")
	}
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", strings.NewReader("too-large"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	if _, err := DecodeMultipart(req, Config{MaxRequestBytes: 3}); err == nil {
		t.Fatal("expected request size error")
	}
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	if _, err := DecodeMultipart(req, Config{}); err == nil {
		t.Fatal("expected malformed multipart error")
	}
}

func TestDecodeMultipartRejectsInvalidMultipartContentTypePrefix(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "multipart/form-dataevil; boundary=x")
	_, err := DecodeMultipart(req, Config{MaxRequestBytes: 128})
	assertUploadFieldError(t, err, "content_type", "invalid")
}

func TestDecodeMultipartAbuseLimitsAndMemoryPressure(t *testing.T) {
	t.Run("oversized file fails closed without echoing payload", func(t *testing.T) {
		payload := bytes.Repeat([]byte("A"), 1024)
		req, size := newMultipartUploadRequest(t, nil, []testMultipartFile{{
			field:       "file",
			filename:    "large.bin",
			contentType: "application/octet-stream",
			payload:     payload,
		}})

		form, err := DecodeMultipart(req, Config{
			MaxRequestBytes: int64(size + 1),
			MaxFileBytes:    128,
			RequiredFiles:   []string{"file"},
		})
		assertUploadFieldError(t, err, "file", "too_large")
		if len(form.Files["file"]) != 1 || form.Files["file"][0].Size != int64(len(payload)) {
			t.Fatalf("expected oversized file metadata to remain available, got %#v", form.Files["file"])
		}
		problem := ValidationProblem(err)
		if strings.Contains(fmt.Sprintf("%#v", problem), strings.Repeat("A", 64)) {
			t.Fatalf("validation problem leaked file payload: %#v", problem)
		}
	})

	t.Run("many parts stay bounded by request limit", func(t *testing.T) {
		fields := map[string][]string{}
		for i := 0; i < 160; i++ {
			fields["tag"] = append(fields["tag"], fmt.Sprintf("value-%03d", i))
		}
		files := make([]testMultipartFile, 0, 24)
		for i := 0; i < 24; i++ {
			files = append(files, testMultipartFile{
				field:       "file",
				filename:    fmt.Sprintf("part-%02d.txt", i),
				contentType: "text/plain",
				payload:     []byte("ok"),
			})
		}
		req, size := newMultipartUploadRequest(t, fields, files)
		form, err := DecodeMultipart(req, Config{
			MaxRequestBytes:     int64(size + 1),
			MaxMemory:           64,
			MaxFileBytes:        8,
			RequiredFiles:       []string{"file"},
			AllowedContentTypes: AllowedContentTypes("text/plain"),
		})
		if err != nil {
			t.Fatalf("DecodeMultipart() many parts error = %v", err)
		}
		if req.MultipartForm != nil {
			t.Cleanup(func() { _ = req.MultipartForm.RemoveAll() })
		}
		if len(form.Values["tag"]) != 160 {
			t.Fatalf("tag value count = %d, want 160", len(form.Values["tag"]))
		}
		if len(form.Files["file"]) != 24 {
			t.Fatalf("file count = %d, want 24", len(form.Files["file"]))
		}
	})

	t.Run("low max memory still opens bounded file", func(t *testing.T) {
		payload := bytes.Repeat([]byte("z"), 64<<10)
		req, size := newMultipartUploadRequest(t, nil, []testMultipartFile{{
			field:       "file",
			filename:    "spill.txt",
			contentType: "text/plain",
			payload:     payload,
		}})
		form, err := DecodeMultipart(req, Config{
			MaxRequestBytes:     int64(size + 1),
			MaxMemory:           32,
			MaxFileBytes:        int64(len(payload) + 1),
			RequiredFiles:       []string{"file"},
			AllowedContentTypes: AllowedContentTypes("text/plain"),
		})
		if err != nil {
			t.Fatalf("DecodeMultipart() low memory error = %v", err)
		}
		if req.MultipartForm != nil {
			t.Cleanup(func() { _ = req.MultipartForm.RemoveAll() })
		}
		file, err := RequireFile(form, "file")
		if err != nil {
			t.Fatalf("RequireFile() error = %v", err)
		}
		opened, err := file.Open()
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		defer opened.Close()
		got, err := io.ReadAll(io.LimitReader(opened, int64(len(payload)+1)))
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("opened payload length = %d, want %d", len(got), len(payload))
		}
	})
}

func TestDecodeMultipartNormalizesWeirdFilenames(t *testing.T) {
	req, size := newMultipartUploadRequest(t, nil, []testMultipartFile{
		{
			field:       "file",
			filename:    `..\..\evil.txt`,
			contentType: "text/plain",
			payload:     []byte("one"),
		},
		{
			field:       "file",
			filename:    " /var/tmp/report.pdf ",
			contentType: "application/pdf",
			payload:     []byte("two"),
		},
	})
	form, err := DecodeMultipart(req, Config{
		MaxRequestBytes: int64(size + 1),
		MaxFileBytes:    8,
		RequiredFiles:   []string{"file"},
	})
	if err != nil {
		t.Fatalf("DecodeMultipart() error = %v", err)
	}
	got := []string{form.Files["file"][0].Filename, form.Files["file"][1].Filename}
	want := []string{"evil.txt", "report.pdf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filenames = %#v, want %#v", got, want)
	}
	for _, filename := range got {
		if strings.ContainsAny(filename, `/\`) || filename == "." || filename == ".." || strings.TrimSpace(filename) != filename {
			t.Fatalf("filename was not normalized: %q", filename)
		}
	}
}

func TestDecodeMultipartRepeatedValuesAndCloneIsolation(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("tag", "one")
	_ = writer.WriteField("tag", "two")
	part, err := writer.CreateFormFile("file", "avatar.txt")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	_, _ = part.Write([]byte("hello"))
	_ = writer.Close()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	form, err := DecodeMultipart(req, Config{RequiredFiles: []string{"file"}, AllowedContentTypes: AllowedContentTypes(" Application/Octet-Stream ")})
	if err != nil {
		t.Fatalf("DecodeMultipart() error = %v", err)
	}
	if !reflect.DeepEqual(form.Values["tag"], []string{"one", "two"}) {
		t.Fatalf("values = %#v", form.Values["tag"])
	}
	req.MultipartForm.Value["tag"][0] = "mutated"
	if form.Values["tag"][0] != "one" {
		t.Fatalf("form values were not cloned: %#v", form.Values["tag"])
	}
	file, err := RequireFile(form, "file")
	if err != nil {
		t.Fatalf("RequireFile() error = %v", err)
	}
	opened, err := file.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	_ = opened.Close()
}

func TestRequireFileAndValidationProblemFallbacks(t *testing.T) {
	if _, err := RequireFile(Form{}, "missing"); err == nil {
		t.Fatal("expected missing file error")
	}
	if _, err := (File{}).Open(); err == nil {
		t.Fatal("expected missing file header error")
	}
	problem := ValidationProblem(errors.New("SELECT * FROM uploads WHERE token='super-secret'"))
	if problem.Detail != "validation failed" {
		t.Fatalf("problem detail = %q", problem.Detail)
	}
	rec := httptest.NewRecorder()
	WriteValidationProblem(rec, unsafeFieldErrorProvider{message: "provider token=super-secret"})
	if rec.Code != http.StatusBadRequest || strings.Contains(rec.Body.String(), "super-secret") || !strings.Contains(rec.Body.String(), "validation failed") {
		t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
	}
}

type unsafeFieldErrorProvider struct {
	message string
}

func (e unsafeFieldErrorProvider) Error() string {
	return e.message
}

func (e unsafeFieldErrorProvider) FieldErrors() fielderrors.FieldErrors {
	return fielderrors.FieldErrors{{
		Field:   "token",
		Code:    "invalid",
		Message: e.message,
	}}
}

type testMultipartFile struct {
	field       string
	filename    string
	contentType string
	payload     []byte
}

func newMultipartUploadRequest(t *testing.T, fields map[string][]string, files []testMultipartFile) (*http.Request, int) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for field, values := range fields {
		for _, value := range values {
			if err := writer.WriteField(field, value); err != nil {
				t.Fatalf("write field %q: %v", field, err)
			}
		}
	}
	for _, file := range files {
		header := textproto.MIMEHeader{}
		disposition := mime.FormatMediaType("form-data", map[string]string{
			"name":     file.field,
			"filename": file.filename,
		})
		if disposition == "" {
			t.Fatalf("format content disposition for filename %q", file.filename)
		}
		header.Set("Content-Disposition", disposition)
		if file.contentType != "" {
			header.Set("Content-Type", file.contentType)
		}
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatalf("create file part %q: %v", file.filename, err)
		}
		if _, err := part.Write(file.payload); err != nil {
			t.Fatalf("write file part %q: %v", file.filename, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, body.Len()
}

func assertUploadFieldError(t *testing.T, err error, field, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected field error %s/%s, got nil", field, code)
	}
	var provider fielderrors.Provider
	if !errors.As(err, &provider) {
		t.Fatalf("expected field error provider, got %T: %v", err, err)
	}
	for _, fieldError := range provider.FieldErrors() {
		if fieldError.Field == field && fieldError.Code == code {
			return
		}
	}
	t.Fatalf("expected field error %s/%s, got %#v", field, code, provider.FieldErrors())
}
