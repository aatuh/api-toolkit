package app

import (
	"context"
	"errors"
	"sort"
	"testing"
)

func TestObjectServiceEnforcesTenantRolesAndPolicy(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewObjectService(tenancy)
	obj, err := service.Put(ctx, "owner_1", org.ID, "readme.txt", "text/plain", []byte("hello"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if obj.Key != "readme.txt" || obj.Size != 5 {
		t.Fatalf("object = %#v", obj)
	}
	got, data, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt")
	if err != nil || !ok || got.Key != obj.Key || string(data) != "hello" {
		t.Fatalf("Get() object=%#v data=%q ok=%v err=%v", got, data, ok, err)
	}
	data[0] = 'x'
	_, again, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt")
	if err != nil || !ok || string(again) != "hello" {
		t.Fatalf("Get() after data mutation data=%q ok=%v err=%v", again, ok, err)
	}
	if _, err := service.List(ctx, "owner_1", org.ID); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if err := service.Delete(ctx, "owner_1", org.ID, "readme.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, _, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt"); err != nil || ok {
		t.Fatalf("Get() after delete ok=%v err=%v", ok, err)
	}
}

func TestObjectServiceRejectsUnsafeInputs(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	service := NewObjectService(tenancy)
	for _, key := range []string{"", "../secret", "nested/file", ".hidden"} {
		if _, err := service.Put(ctx, "owner_1", org.ID, key, "text/plain", []byte("hello")); !errors.Is(err, ErrValidation) {
			t.Fatalf("Put(%q) error = %v, want %v", key, err, ErrValidation)
		}
	}
	if _, err := service.Put(ctx, "owner_1", org.ID, "readme.txt", "application/x-secret", []byte("hello")); !errors.Is(err, ErrValidation) {
		t.Fatalf("unsafe content-type error = %v, want %v", err, ErrValidation)
	}
	if _, err := service.Put(ctx, "stranger_1", org.ID, "readme.txt", "text/plain", []byte("hello")); !errors.Is(err, ErrForbidden) {
		t.Fatalf("stranger Put() error = %v, want %v", err, ErrForbidden)
	}
}

func TestObjectServiceWithBlobStoreKeepsTenantPolicy(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	blobs := newRecordingObjectBlobStore()
	service := NewObjectServiceWithBlobStore(tenancy, blobs)
	if _, err := service.Put(ctx, "owner_1", org.ID, "readme.txt", "text/plain", []byte("hello")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if blobs.putRef.TenantID != org.ID || blobs.putRef.Key != "readme.txt" || string(blobs.data) != "hello" {
		t.Fatalf("blob put ref=%#v data=%q", blobs.putRef, blobs.data)
	}
	got, data, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt")
	if err != nil || !ok || got.Size != 5 || string(data) != "hello" {
		t.Fatalf("Get() object=%#v data=%q ok=%v err=%v", got, data, ok, err)
	}
	if _, _, _, err := service.Get(ctx, "other_user", org.ID, "readme.txt"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-actor Get() error = %v, want %v", err, ErrForbidden)
	}
	if err := service.Delete(ctx, "owner_1", org.ID, "readme.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !blobs.deleted {
		t.Fatalf("blob was not deleted")
	}
}

func TestObjectServiceWithMetadataStorePersistsListAndDelete(t *testing.T) {
	ctx := context.Background()
	tenancy := NewTenancyService()
	org, _, err := tenancy.CreateOrganization(ctx, "owner_1", "Acme")
	if err != nil {
		t.Fatalf("CreateOrganization() error = %v", err)
	}
	metadata := newRecordingObjectMetadataStore()
	blobs := newRecordingObjectBlobStore()
	service := NewObjectServiceWithStores(tenancy, metadata, blobs)
	obj, err := service.Put(ctx, "owner_1", org.ID, "readme.txt", "text/plain", []byte("hello"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if metadata.saved.Key != "readme.txt" || metadata.saved.Size != 5 {
		t.Fatalf("saved metadata = %#v", metadata.saved)
	}
	listed, err := service.List(ctx, "owner_1", org.ID)
	if err != nil || len(listed) != 1 || listed[0].Key != obj.Key {
		t.Fatalf("List() = %#v err=%v", listed, err)
	}
	got, data, ok, err := service.Get(ctx, "owner_1", org.ID, "readme.txt")
	if err != nil || !ok || got.CreatedAt != obj.CreatedAt || string(data) != "hello" {
		t.Fatalf("Get() object=%#v data=%q ok=%v err=%v", got, data, ok, err)
	}
	if err := service.Delete(ctx, "owner_1", org.ID, "readme.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !metadata.deleted || !blobs.deleted {
		t.Fatalf("delete did not update metadata/blob: metadata=%v blob=%v", metadata.deleted, blobs.deleted)
	}
}

type recordingObjectMetadataStore struct {
	objects map[string]Object
	saved   Object
	deleted bool
}

func newRecordingObjectMetadataStore() *recordingObjectMetadataStore {
	return &recordingObjectMetadataStore{objects: map[string]Object{}}
}

func (s *recordingObjectMetadataStore) SaveObjectMetadata(_ context.Context, object Object) error {
	s.saved = object
	s.objects[objectID(object.TenantID, object.Key)] = object
	return nil
}

func (s *recordingObjectMetadataStore) GetObjectMetadata(_ context.Context, tenantID, key string) (Object, bool, error) {
	object, ok := s.objects[objectID(tenantID, key)]
	return object, ok, nil
}

func (s *recordingObjectMetadataStore) ListObjectMetadata(_ context.Context, tenantID string) ([]Object, error) {
	out := make([]Object, 0, len(s.objects))
	for _, object := range s.objects {
		if object.TenantID == tenantID {
			out = append(out, object)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *recordingObjectMetadataStore) DeleteObjectMetadata(_ context.Context, tenantID, key string) (bool, error) {
	id := objectID(tenantID, key)
	if _, ok := s.objects[id]; !ok {
		return false, nil
	}
	delete(s.objects, id)
	s.deleted = true
	return true, nil
}

type recordingObjectBlobStore struct {
	putRef      ObjectRef
	contentType string
	data        []byte
	deleted     bool
}

func newRecordingObjectBlobStore() *recordingObjectBlobStore {
	return &recordingObjectBlobStore{}
}

func (s *recordingObjectBlobStore) PutObject(_ context.Context, ref ObjectRef, data []byte, contentType string) error {
	s.putRef = ref
	s.contentType = contentType
	s.data = append([]byte(nil), data...)
	return nil
}

func (s *recordingObjectBlobStore) GetObject(_ context.Context, ref ObjectRef) (ObjectBlob, bool, error) {
	if ref != s.putRef {
		return ObjectBlob{}, false, nil
	}
	return ObjectBlob{ContentType: s.contentType, Size: int64(len(s.data)), Data: append([]byte(nil), s.data...)}, true, nil
}

func (s *recordingObjectBlobStore) DeleteObject(_ context.Context, ref ObjectRef) (bool, error) {
	if ref != s.putRef {
		return false, nil
	}
	s.deleted = true
	return true, nil
}
