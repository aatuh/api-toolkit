// Command file-upload shows a multipart upload endpoint with size limits.
package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/adapters/chi"
	"github.com/aatuh/api-toolkit/v3/httpx"
	maxbody "github.com/aatuh/api-toolkit/v3/middleware/maxbody"
)

const maxUploadBytes = 8 << 20

type uploadResult struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
}

func main() {
	r := chi.New()
	maxBodyMw, err := maxbody.New(maxbody.Options{MaxBytes: maxUploadBytes})
	if err != nil {
		log.Fatalf("init max body: %v", err)
	}
	r.Use(maxBodyMw.Middleware())
	r.Post("/upload", handleUpload)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	reader, err := r.MultipartReader()
	if err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
			Title:  http.StatusText(http.StatusBadRequest),
			Detail: "expected multipart form data",
		})
		return
	}

	var result *uploadResult
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			httpx.WriteProblem(w, http.StatusRequestEntityTooLarge, httpx.Problem{
				Title:  http.StatusText(http.StatusRequestEntityTooLarge),
				Detail: "upload too large",
			})
			return
		}
		if part.FormName() != "file" {
			_, _ = io.Copy(io.Discard, part)
			_ = part.Close()
			continue
		}
		size, err := io.Copy(io.Discard, part)
		_ = part.Close()
		if err != nil {
			httpx.WriteProblem(w, http.StatusRequestEntityTooLarge, httpx.Problem{
				Title:  http.StatusText(http.StatusRequestEntityTooLarge),
				Detail: "upload too large",
			})
			return
		}
		result = &uploadResult{
			Filename:    part.FileName(),
			ContentType: part.Header.Get("Content-Type"),
			Bytes:       size,
		}
	}
	if result == nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
			Title:  http.StatusText(http.StatusBadRequest),
			Detail: "missing file part",
		})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}
