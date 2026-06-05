package upload

import (
	"bytes"
	"context"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
)

func FuzzDecodeMultipartMetadata(f *testing.F) {
	for _, seed := range []struct {
		filename    string
		contentType string
		body        string
	}{
		{"avatar.txt", "text/plain", "hello"},
		{"photo.png", "application/octet-stream", "\x89PNG"},
		{" spaced name .txt ", " TEXT/PLAIN ", ""},
		{"bad\r\nname", "text/plain", "payload"},
	} {
		f.Add(seed.filename, seed.contentType, seed.body)
	}
	f.Fuzz(func(t *testing.T, filename, contentType, payload string) {
		filename = limitUploadFuzzString(filename, 512)
		payload = limitUploadFuzzString(payload, 2048)
		contentType = normalizeFuzzContentType(contentType)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		header := textproto.MIMEHeader{}
		disposition := mime.FormatMediaType("form-data", map[string]string{
			"name":     "file",
			"filename": filename,
		})
		if disposition == "" {
			disposition = `form-data; name="file"; filename="upload.bin"`
		}
		header.Set("Content-Disposition", disposition)
		header.Set("Content-Type", contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return
		}
		if _, err := part.Write([]byte(payload)); err != nil {
			return
		}
		if err := writer.Close(); err != nil {
			return
		}

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		form, err := DecodeMultipart(req, Config{
			MaxRequestBytes:     int64(body.Len() + 1),
			MaxFileBytes:        int64(len(payload) + 1),
			RequiredFiles:       []string{"file"},
			AllowedContentTypes: AllowedContentTypes(contentType),
		})
		if err != nil {
			return
		}
		file, err := RequireFile(form, "file")
		if err != nil {
			t.Fatalf("DecodeMultipart succeeded without required file: %v", err)
		}
		if file.Size != int64(len(payload)) {
			t.Fatalf("file size = %d, want %d", file.Size, len(payload))
		}
		if strings.TrimSpace(strings.ToLower(file.ContentType)) != strings.TrimSpace(strings.ToLower(contentType)) {
			t.Fatalf("content type = %q, want %q", file.ContentType, contentType)
		}
	})
}

func normalizeFuzzContentType(value string) string {
	value = limitUploadFuzzString(strings.TrimSpace(value), 512)
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "application/octet-stream"
	}
	return value
}

func limitUploadFuzzString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
