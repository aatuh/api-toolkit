package objectstores3

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v4/objectstore"
	"github.com/aatuh/api-toolkit/contrib/v4/objectstore/objectstoretest"
)

func TestObjectStoreContract(t *testing.T) {
	t.Parallel()

	server := newContractS3Server(t)
	store := testStore(t, server.URL)

	objectstoretest.AssertStoreContract(t, func(testing.TB) objectstore.Store {
		return store
	})
	objectstoretest.AssertSignedURLerContract(t, store, objectstore.Ref{
		Bucket: "tenant-objects",
		Key:    "org_123/widgets/avatar.png",
	})
}

func TestPutSignsAndSendsObject(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath, gotBody, gotAuth, gotContentType, gotMeta string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotMeta = r.Header.Get("X-Amz-Meta-Owner")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := testStore(t, server.URL)
	err := store.Put(context.Background(), objectstore.Ref{Bucket: "tenant-objects", Key: "org_123/avatar.png"}, strings.NewReader("payload"), objectstore.PutOptions{
		Size:        7,
		ContentType: "image/png",
		Metadata:    map[string]string{"owner": "org_123"},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if gotMethod != http.MethodPut || gotPath != "/tenant-objects/org_123/avatar.png" || gotBody != "payload" {
		t.Fatalf("request = %s %s body %q", gotMethod, gotPath, gotBody)
	}
	if gotContentType != "image/png" || gotMeta != "org_123" {
		t.Fatalf("headers content-type=%q metadata=%q", gotContentType, gotMeta)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 ") {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if strings.Contains(gotAuth, "secret") {
		t.Fatalf("Authorization leaked secret: %q", gotAuth)
	}
}

func TestPutEnforcesPolicyBeforeNetwork(t *testing.T) {
	t.Parallel()

	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := testStore(t, server.URL)
	err := store.Put(context.Background(), objectstore.Ref{Bucket: "tenant-objects", Key: "org_123/avatar.svg"}, strings.NewReader("<svg/>"), objectstore.PutOptions{
		Size:        6,
		ContentType: "image/svg+xml",
	})
	if !errors.Is(err, objectstore.ErrContentTypeNotAllowed) {
		t.Fatalf("Put() error = %v, want %v", err, objectstore.ErrContentTypeNotAllowed)
	}
	if called {
		t.Fatal("network was called for policy failure")
	}
}

func TestGetMapsNotFoundAndReturnsMetadata(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "missing") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "2")
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("X-Amz-Meta-Owner", "org_123")
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	store := testStore(t, server.URL)
	result, err := store.Get(context.Background(), objectstore.Ref{Bucket: "tenant-objects", Key: "org_123/widget.json"})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	body, err := io.ReadAll(result.Body)
	if closeErr := result.Body.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("read body error = %v", err)
	}
	if string(body) != "{}" || result.ContentType != "application/json" || result.Size != 2 || result.ETag != `"abc"` || result.Metadata["owner"] != "org_123" {
		t.Fatalf("Get() result = %#v body %q", result, body)
	}
	if _, err := store.Get(context.Background(), objectstore.Ref{Bucket: "tenant-objects", Key: "org_123/missing.json"}); !errors.Is(err, objectstore.ErrObjectNotFound) {
		t.Fatalf("Get(missing) error = %v, want %v", err, objectstore.ErrObjectNotFound)
	}
}

func TestDeleteMapsNotFoundToSuccess(t *testing.T) {
	t.Parallel()

	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	store := testStore(t, server.URL)
	if err := store.Delete(context.Background(), objectstore.Ref{Bucket: "tenant-objects", Key: "org_123/widget.json"}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Fatalf("method = %q, want DELETE", gotMethod)
	}
}

func TestSignedURLBuildsSigV4PresignedURL(t *testing.T) {
	t.Parallel()

	store := testStore(t, "https://objects.example.com")
	signed, err := store.SignedURL(context.Background(), objectstore.Ref{Bucket: "tenant-objects", Key: "org_123/avatar.png"}, objectstore.SignedURLOptions{
		Method:      "PUT",
		Expires:     time.Minute,
		ContentType: "image/png",
	})
	if err != nil {
		t.Fatalf("SignedURL() error = %v", err)
	}
	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	values := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "objects.example.com" || parsed.EscapedPath() != "/tenant-objects/org_123/avatar.png" {
		t.Fatalf("signed URL target = %s", signed)
	}
	for _, name := range []string{"X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-Date", "X-Amz-Expires", "X-Amz-SignedHeaders", "X-Amz-Signature"} {
		if values.Get(name) == "" {
			t.Fatalf("signed URL missing %s: %s", name, signed)
		}
	}
	if values.Get("X-Amz-SignedHeaders") != "content-type;host" {
		t.Fatalf("signed headers = %q", values.Get("X-Amz-SignedHeaders"))
	}
	if strings.Contains(signed, "secret") {
		t.Fatalf("signed URL leaked secret: %s", signed)
	}
}

func TestPayloadHashUsesSHA256ForSmallKnownBodies(t *testing.T) {
	t.Parallel()

	sum := sha256.Sum256([]byte("payload"))
	if got := payloadHash([]byte("payload")); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("payloadHash() = %q", got)
	}
}

func testStore(t *testing.T, endpoint string) *Store {
	t.Helper()

	store, err := New(Options{
		Endpoint:            endpoint,
		Region:              "us-test-1",
		AccessKeyID:         "access",
		SecretAccessKey:     "secret",
		Clock:               func() time.Time { return time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC) },
		MaxObjectSize:       16,
		AllowedContentTypes: []string{"image/png", "application/json"},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return store
}

type contractObject struct {
	body        []byte
	contentType string
	metadata    map[string]string
}

func newContractS3Server(t *testing.T) *httptest.Server {
	t.Helper()

	var (
		mu      sync.Mutex
		objects = map[string]contractObject{}
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Fatalf("missing Authorization for %s %s", r.Method, r.URL.Path)
		}
		if strings.Contains(r.Header.Get("Authorization"), "secret") {
			t.Fatalf("Authorization leaked secret: %q", r.Header.Get("Authorization"))
		}
		key := r.URL.EscapedPath()
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			metadata := map[string]string{}
			for name, values := range r.Header {
				canonical := http.CanonicalHeaderKey(name)
				if !strings.HasPrefix(canonical, "X-Amz-Meta-") || len(values) == 0 {
					continue
				}
				metadata[strings.ToLower(strings.TrimPrefix(canonical, "X-Amz-Meta-"))] = values[0]
			}
			mu.Lock()
			objects[key] = contractObject{
				body:        append([]byte(nil), body...),
				contentType: r.Header.Get("Content-Type"),
				metadata:    metadata,
			}
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			mu.Lock()
			obj, ok := objects[key]
			mu.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", obj.contentType)
			w.Header().Set("Content-Length", strconv.Itoa(len(obj.body)))
			for name, value := range obj.metadata {
				w.Header().Set("X-Amz-Meta-"+name, value)
			}
			_, _ = w.Write(obj.body)
		case http.MethodDelete:
			mu.Lock()
			delete(objects, key)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	t.Cleanup(server.Close)
	return server
}
