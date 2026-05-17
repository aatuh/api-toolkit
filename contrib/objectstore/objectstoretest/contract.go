// Package objectstoretest contains reusable object store contract tests.
package objectstoretest

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/objectstore"
)

// StoreFactory builds a fresh object store for one contract test run.
type StoreFactory func(testing.TB) objectstore.Store

// AssertStoreContract verifies behavior shared by object store implementations.
func AssertStoreContract(t testing.TB, newStore StoreFactory) {
	t.Helper()

	store := newStore(t)
	if store == nil {
		t.Fatal("newStore returned nil")
	}
	ctx := context.Background()
	ref := objectstore.Ref{Bucket: "tenant-objects", Key: "org_123/widgets/avatar.png"}
	if err := store.Put(ctx, objectstore.Ref{}, strings.NewReader("payload"), objectstore.PutOptions{
		Size:        7,
		ContentType: "application/json",
	}); !errors.Is(err, objectstore.ErrInvalidRef) {
		t.Fatalf("Put(invalid ref) error = %v, want %v", err, objectstore.ErrInvalidRef)
	}
	if err := store.Put(ctx, ref, strings.NewReader("payload"), objectstore.PutOptions{
		Size:        7,
		ContentType: "application/json",
		Metadata:    map[string]string{"owner": "org_123"},
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	body, err := io.ReadAll(got.Body)
	if closeErr := got.Body.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("ReadAll(Get().Body) error = %v", err)
	}
	if string(body) != "payload" || got.ContentType != "application/json" || got.Size != 7 {
		t.Fatalf("Get() = body %q content type %q size %d", body, got.ContentType, got.Size)
	}
	got.Metadata["owner"] = "changed"
	gotAgain, err := store.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get() second error = %v", err)
	}
	_ = gotAgain.Body.Close()
	if gotAgain.Metadata["owner"] != "org_123" {
		t.Fatalf("metadata was not defensively copied")
	}
	if err := store.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(ctx, ref); !errors.Is(err, objectstore.ErrObjectNotFound) {
		t.Fatalf("Get() after delete error = %v, want %v", err, objectstore.ErrObjectNotFound)
	}
}

// AssertSignedURLerContract verifies common signed URL behavior.
func AssertSignedURLerContract(t testing.TB, signer objectstore.SignedURLer, ref objectstore.Ref) {
	t.Helper()

	if signer == nil {
		t.Fatal("signer is nil")
	}
	url, err := signer.SignedURL(context.Background(), ref, objectstore.SignedURLOptions{
		Method:  "GET",
		Expires: time.Minute,
	})
	if err != nil {
		t.Fatalf("SignedURL() error = %v", err)
	}
	if strings.TrimSpace(url) == "" {
		t.Fatal("SignedURL() returned empty URL")
	}
	if _, err := signer.SignedURL(context.Background(), objectstore.Ref{}, objectstore.SignedURLOptions{}); !errors.Is(err, objectstore.ErrInvalidRef) {
		t.Fatalf("SignedURL(invalid ref) error = %v, want %v", err, objectstore.ErrInvalidRef)
	}
}
