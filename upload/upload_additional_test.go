package upload

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
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
	problem := ValidationProblem(errors.New("plain failure"))
	if problem.Detail != "plain failure" {
		t.Fatalf("problem detail = %q", problem.Detail)
	}
	rec := httptest.NewRecorder()
	WriteValidationProblem(rec, errors.New("plain failure"))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "plain failure") {
		t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
	}
}
