package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"example.com/reference-saas-api/internal/domain"
)

const maxObjectBytes = 1024 * 1024

var allowedObjectContentTypes = map[string]struct{}{
	"application/json": {},
	"application/pdf":  {},
	"image/jpeg":       {},
	"image/png":        {},
	"text/plain":       {},
}

type ObjectService struct {
	mu      sync.Mutex
	tenancy *TenancyService
	now     func() time.Time
	objects map[string]Object
	data    map[string][]byte
	store   ObjectMetadataStore
	blobs   ObjectBlobStore
}

type ObjectMetadataStore interface {
	SaveObjectMetadata(ctx context.Context, object Object) error
	GetObjectMetadata(ctx context.Context, tenantID, key string) (Object, bool, error)
	ListObjectMetadata(ctx context.Context, tenantID string) ([]Object, error)
	DeleteObjectMetadata(ctx context.Context, tenantID, key string) (bool, error)
}

type ObjectBlobStore interface {
	PutObject(ctx context.Context, ref ObjectRef, data []byte, contentType string) error
	GetObject(ctx context.Context, ref ObjectRef) (ObjectBlob, bool, error)
	DeleteObject(ctx context.Context, ref ObjectRef) (bool, error)
}

type ObjectRef struct {
	TenantID string
	Key      string
}

type ObjectBlob struct {
	ContentType string
	Size        int64
	Data        []byte
}

type Object struct {
	TenantID    string
	Key         string
	ContentType string
	Size        int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (o Object) Public() map[string]any {
	return map[string]any{
		"tenant_id":    o.TenantID,
		"key":          o.Key,
		"content_type": o.ContentType,
		"size":         o.Size,
		"created_at":   o.CreatedAt,
		"updated_at":   o.UpdatedAt,
	}
}

func NewObjectService(tenancy *TenancyService) *ObjectService {
	return &ObjectService{tenancy: tenancy, now: time.Now, objects: map[string]Object{}, data: map[string][]byte{}}
}

func NewObjectServiceWithBlobStore(tenancy *TenancyService, blobs ObjectBlobStore) *ObjectService {
	return NewObjectServiceWithStores(tenancy, nil, blobs)
}

func NewObjectServiceWithStores(tenancy *TenancyService, store ObjectMetadataStore, blobs ObjectBlobStore) *ObjectService {
	service := NewObjectService(tenancy)
	service.store = store
	service.blobs = blobs
	return service
}

func (s *ObjectService) Put(ctx context.Context, actorID, tenantID, key, contentType string, data []byte) (Object, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, err
	}
	if s == nil || s.tenancy == nil {
		return Object{}, ErrValidation
	}
	if err := validateObjectKey(key); err != nil {
		return Object{}, err
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if _, ok := allowedObjectContentTypes[contentType]; !ok {
		return Object{}, ErrValidation
	}
	if len(data) == 0 || len(data) > maxObjectBytes {
		return Object{}, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleMember)
	if err != nil {
		return Object{}, err
	}
	if !ok {
		return Object{}, ErrForbidden
	}
	key = strings.TrimSpace(key)
	tenantID = strings.TrimSpace(tenantID)
	now := s.now().UTC()
	id := objectID(tenantID, key)
	obj := Object{TenantID: tenantID, Key: key, ContentType: contentType, Size: int64(len(data)), CreatedAt: now, UpdatedAt: now}
	if s.store != nil {
		existing, ok, err := s.store.GetObjectMetadata(ctx, tenantID, key)
		if err != nil {
			return Object{}, err
		}
		if ok {
			obj.CreatedAt = existing.CreatedAt
		}
		if s.blobs != nil {
			if err := s.blobs.PutObject(ctx, ObjectRef{TenantID: tenantID, Key: key}, append([]byte(nil), data...), contentType); err != nil {
				return Object{}, err
			}
		}
		if err := s.store.SaveObjectMetadata(ctx, obj); err != nil {
			if s.blobs != nil {
				_, _ = s.blobs.DeleteObject(ctx, ObjectRef{TenantID: tenantID, Key: key})
			}
			return Object{}, err
		}
		if s.blobs == nil {
			s.mu.Lock()
			s.data[id] = append([]byte(nil), data...)
			s.mu.Unlock()
		}
		return obj, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.objects[id]; ok {
		obj.CreatedAt = existing.CreatedAt
	}
	if s.blobs != nil {
		if err := s.blobs.PutObject(ctx, ObjectRef{TenantID: tenantID, Key: key}, append([]byte(nil), data...), contentType); err != nil {
			return Object{}, err
		}
	}
	s.objects[id] = obj
	s.data[id] = append([]byte(nil), data...)
	return obj, nil
}

func (s *ObjectService) Get(ctx context.Context, actorID, tenantID, key string) (Object, []byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return Object{}, nil, false, err
	}
	if s == nil || s.tenancy == nil {
		return Object{}, nil, false, ErrValidation
	}
	if err := validateObjectKey(key); err != nil {
		return Object{}, nil, false, err
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleViewer)
	if err != nil {
		return Object{}, nil, false, err
	}
	if !ok {
		return Object{}, nil, false, ErrForbidden
	}
	if s.store != nil {
		tenantID = strings.TrimSpace(tenantID)
		key = strings.TrimSpace(key)
		obj, ok, err := s.store.GetObjectMetadata(ctx, tenantID, key)
		if err != nil || !ok {
			return Object{}, nil, ok, err
		}
		if s.blobs != nil {
			blob, ok, err := s.blobs.GetObject(ctx, ObjectRef{TenantID: tenantID, Key: key})
			if err != nil || !ok {
				return Object{}, nil, ok, err
			}
			return obj, append([]byte(nil), blob.Data...), true, nil
		}
		id := objectID(tenantID, key)
		s.mu.Lock()
		defer s.mu.Unlock()
		data, ok := s.data[id]
		if !ok {
			return Object{}, nil, false, nil
		}
		return obj, append([]byte(nil), data...), true, nil
	}
	if s.blobs != nil {
		blob, ok, err := s.blobs.GetObject(ctx, ObjectRef{TenantID: strings.TrimSpace(tenantID), Key: strings.TrimSpace(key)})
		if err != nil || !ok {
			return Object{}, nil, ok, err
		}
		now := s.now().UTC()
		obj := Object{TenantID: strings.TrimSpace(tenantID), Key: strings.TrimSpace(key), ContentType: blob.ContentType, Size: blob.Size, CreatedAt: now, UpdatedAt: now}
		return obj, append([]byte(nil), blob.Data...), true, nil
	}
	id := objectID(tenantID, key)
	s.mu.Lock()
	defer s.mu.Unlock()
	obj, ok := s.objects[id]
	if !ok {
		return Object{}, nil, false, nil
	}
	return obj, append([]byte(nil), s.data[id]...), true, nil
}

func (s *ObjectService) List(ctx context.Context, actorID, tenantID string) ([]Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.tenancy == nil {
		return nil, ErrValidation
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleViewer)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrForbidden
	}
	tenantID = strings.TrimSpace(tenantID)
	if s.store != nil {
		return s.store.ListObjectMetadata(ctx, tenantID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Object, 0)
	for _, obj := range s.objects {
		if obj.TenantID == tenantID {
			out = append(out, obj)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (s *ObjectService) Delete(ctx context.Context, actorID, tenantID, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.tenancy == nil {
		return ErrValidation
	}
	if err := validateObjectKey(key); err != nil {
		return err
	}
	ok, err := s.tenancy.HasRole(ctx, tenantID, actorID, domain.RoleMember)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	if s.store != nil {
		tenantID = strings.TrimSpace(tenantID)
		key = strings.TrimSpace(key)
		if _, ok, err := s.store.GetObjectMetadata(ctx, tenantID, key); err != nil {
			return err
		} else if !ok {
			return ErrNotFound
		}
		if s.blobs != nil {
			if _, err := s.blobs.DeleteObject(ctx, ObjectRef{TenantID: tenantID, Key: key}); err != nil {
				return err
			}
		}
		deleted, err := s.store.DeleteObjectMetadata(ctx, tenantID, key)
		if err != nil {
			return err
		}
		if !deleted {
			return ErrNotFound
		}
		id := objectID(tenantID, key)
		s.mu.Lock()
		delete(s.data, id)
		s.mu.Unlock()
		return nil
	}
	if s.blobs != nil {
		ok, err := s.blobs.DeleteObject(ctx, ObjectRef{TenantID: strings.TrimSpace(tenantID), Key: strings.TrimSpace(key)})
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		id := objectID(tenantID, key)
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.objects, id)
		delete(s.data, id)
		return nil
	}
	id := objectID(tenantID, key)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[id]; !ok {
		return ErrNotFound
	}
	delete(s.objects, id)
	delete(s.data, id)
	return nil
}

func validateObjectKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 256 || strings.HasPrefix(key, ".") || strings.Contains(key, "..") || strings.ContainsAny(key, "/\\") {
		return ErrValidation
	}
	for _, r := range key {
		if unicode.IsControl(r) {
			return ErrValidation
		}
	}
	return nil
}

func objectID(tenantID, key string) string {
	return fmt.Sprintf("%s\x00%s", strings.TrimSpace(tenantID), strings.TrimSpace(key))
}
