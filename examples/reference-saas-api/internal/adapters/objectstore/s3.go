package objectstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aatuh/api-toolkit/contrib/v4/adapters/objectstores3"
	toolkitobjectstore "github.com/aatuh/api-toolkit/contrib/v4/objectstore"

	"example.com/reference-saas-api/internal/app"
)

const maxS3ObjectBytes = 1024 * 1024

var ErrS3ConfigRequired = errors.New("s3 object store configuration is required")

type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

type S3BlobStore struct {
	store toolkitobjectstore.Store
}

func OpenS3BlobStore(cfg S3Config) (*S3BlobStore, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" ||
		strings.TrimSpace(cfg.Region) == "" ||
		strings.TrimSpace(cfg.Bucket) == "" ||
		strings.TrimSpace(cfg.AccessKeyID) == "" ||
		strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, ErrS3ConfigRequired
	}
	store, err := objectstores3.New(objectstores3.Options{
		Endpoint:            cfg.Endpoint,
		Region:              cfg.Region,
		Bucket:              cfg.Bucket,
		AccessKeyID:         cfg.AccessKeyID,
		SecretAccessKey:     cfg.SecretAccessKey,
		MaxObjectSize:       maxS3ObjectBytes,
		AllowedContentTypes: []string{"application/json", "application/pdf", "image/jpeg", "image/png", "text/plain"},
	})
	if err != nil {
		return nil, err
	}
	return &S3BlobStore{store: store}, nil
}

func NewS3BlobStore(store toolkitobjectstore.Store) *S3BlobStore {
	return &S3BlobStore{store: store}
}

func (s *S3BlobStore) PutObject(ctx context.Context, ref app.ObjectRef, data []byte, contentType string) error {
	if s == nil || s.store == nil {
		return ErrS3ConfigRequired
	}
	objectRef, err := toObjectRef(ref)
	if err != nil {
		return err
	}
	return s.store.Put(ctx, objectRef, bytes.NewReader(data), toolkitobjectstore.PutOptions{
		Size:        int64(len(data)),
		ContentType: strings.TrimSpace(contentType),
		Metadata:    map[string]string{"tenant_id": strings.TrimSpace(ref.TenantID)},
	})
}

func (s *S3BlobStore) GetObject(ctx context.Context, ref app.ObjectRef) (app.ObjectBlob, bool, error) {
	if s == nil || s.store == nil {
		return app.ObjectBlob{}, false, ErrS3ConfigRequired
	}
	objectRef, err := toObjectRef(ref)
	if err != nil {
		return app.ObjectBlob{}, false, err
	}
	result, err := s.store.Get(ctx, objectRef)
	if errors.Is(err, toolkitobjectstore.ErrObjectNotFound) {
		return app.ObjectBlob{}, false, nil
	}
	if err != nil {
		return app.ObjectBlob{}, false, err
	}
	defer result.Body.Close()
	data, err := io.ReadAll(io.LimitReader(result.Body, maxS3ObjectBytes+1))
	if err != nil {
		return app.ObjectBlob{}, false, err
	}
	if len(data) > maxS3ObjectBytes {
		return app.ObjectBlob{}, false, toolkitobjectstore.ErrObjectTooLarge
	}
	return app.ObjectBlob{ContentType: result.ContentType, Size: int64(len(data)), Data: data}, true, nil
}

func (s *S3BlobStore) DeleteObject(ctx context.Context, ref app.ObjectRef) (bool, error) {
	if s == nil || s.store == nil {
		return false, ErrS3ConfigRequired
	}
	objectRef, err := toObjectRef(ref)
	if err != nil {
		return false, err
	}
	if err := s.store.Delete(ctx, objectRef); errors.Is(err, toolkitobjectstore.ErrObjectNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func toObjectRef(ref app.ObjectRef) (toolkitobjectstore.Ref, error) {
	tenantID := strings.TrimSpace(ref.TenantID)
	key := strings.TrimSpace(ref.Key)
	if tenantID == "" || key == "" {
		return toolkitobjectstore.Ref{}, toolkitobjectstore.ErrInvalidRef
	}
	objectRef := toolkitobjectstore.Ref{Key: fmt.Sprintf("%s/%s", tenantID, key)}
	if err := toolkitobjectstore.ValidateRef(toolkitobjectstore.Ref{Bucket: "api-objects", Key: objectRef.Key}); err != nil {
		return toolkitobjectstore.Ref{}, err
	}
	return objectRef, nil
}
