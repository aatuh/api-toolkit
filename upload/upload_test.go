package upload

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecodeMultipart(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("name", "avatar")
	part, err := writer.CreateFormFile("file", "avatar.txt")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	_, _ = part.Write([]byte("hello"))
	_ = writer.Close()

	request := httptest.NewRequest(http.MethodPost, "/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	form, err := DecodeMultipart(request, Config{RequiredFiles: []string{"file"}, MaxFileBytes: MaxFileBytes(10)})
	if err != nil {
		t.Fatalf("DecodeMultipart() error = %v", err)
	}
	if form.Values["name"][0] != "avatar" || len(form.Files["file"]) != 1 {
		t.Fatalf("form = %#v", form)
	}
	if _, err := RequireFile(form, "file"); err != nil {
		t.Fatalf("RequireFile() error = %v", err)
	}
}

func TestDecodeMultipartValidationErrors(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "avatar.txt")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	_, _ = part.Write([]byte("hello"))
	_ = writer.Close()

	request := httptest.NewRequest(http.MethodPost, "/upload", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	_, err = DecodeMultipart(request, Config{RequiredFiles: []string{"avatar"}, MaxFileBytes: 1, AllowedContentTypes: AllowedContentTypes("image/png")})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	problem := ValidationProblem(err)
	if problem.Ext["validation"] == nil {
		t.Fatalf("validation problem missing field errors: %#v", problem)
	}
}
