package objectstore

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	toolkitobjectstore "github.com/aatuh/api-toolkit/contrib/v3/objectstore"

	"example.com/reference-saas-api/internal/app"
)

func TestOpenS3BlobStoreRequiresConfig(t *testing.T) {
	if _, err := OpenS3BlobStore(S3Config{}); !errors.Is(err, ErrS3ConfigRequired) {
		t.Fatalf("OpenS3BlobStore() error = %v, want %v", err, ErrS3ConfigRequired)
	}
}

func TestS3BlobStorePutGetDelete(t *testing.T) {
	store := &recordingObjectStore{}
	blobs := NewS3BlobStore(store)
	ref := app.ObjectRef{TenantID: "org_1", Key: "readme.txt"}
	if err := blobs.PutObject(context.Background(), ref, []byte("hello"), "text/plain"); err != nil {
		t.Fatalf("PutObject() error = %v", err)
	}
	if store.ref.Key != "org_1/readme.txt" || store.contentType != "text/plain" || string(store.data) != "hello" {
		t.Fatalf("put ref=%#v contentType=%q data=%q", store.ref, store.contentType, store.data)
	}
	got, ok, err := blobs.GetObject(context.Background(), ref)
	if err != nil || !ok || got.ContentType != "text/plain" || string(got.Data) != "hello" {
		t.Fatalf("GetObject() blob=%#v ok=%v err=%v", got, ok, err)
	}
	ok, err = blobs.DeleteObject(context.Background(), ref)
	if err != nil || !ok || !store.deleted {
		t.Fatalf("DeleteObject() ok=%v deleted=%v err=%v", ok, store.deleted, err)
	}
}

func TestS3BlobStoreRejectsUnsafeRef(t *testing.T) {
	blobs := NewS3BlobStore(&recordingObjectStore{})
	if err := blobs.PutObject(context.Background(), app.ObjectRef{TenantID: "org_1", Key: "../secret"}, []byte("hello"), "text/plain"); !errors.Is(err, toolkitobjectstore.ErrInvalidRef) {
		t.Fatalf("PutObject() error = %v, want %v", err, toolkitobjectstore.ErrInvalidRef)
	}
}

type recordingObjectStore struct {
	ref         toolkitobjectstore.Ref
	contentType string
	data        []byte
	deleted     bool
}

func (s *recordingObjectStore) Put(_ context.Context, ref toolkitobjectstore.Ref, body io.Reader, opts toolkitobjectstore.PutOptions) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.ref = ref
	s.contentType = opts.ContentType
	s.data = data
	return nil
}

func (s *recordingObjectStore) Get(_ context.Context, ref toolkitobjectstore.Ref) (toolkitobjectstore.GetResult, error) {
	if ref != s.ref {
		return toolkitobjectstore.GetResult{}, toolkitobjectstore.ErrObjectNotFound
	}
	return toolkitobjectstore.GetResult{
		Body:        io.NopCloser(strings.NewReader(string(s.data))),
		ContentType: s.contentType,
		Size:        int64(len(s.data)),
	}, nil
}

func (s *recordingObjectStore) Delete(_ context.Context, ref toolkitobjectstore.Ref) error {
	if ref != s.ref {
		return toolkitobjectstore.ErrObjectNotFound
	}
	s.deleted = true
	return nil
}
