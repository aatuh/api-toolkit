package objectstore

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestValidateRefRejectsUnsafeObjectNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ref  Ref
	}{
		{name: "missing bucket", ref: Ref{Key: "tenant/file.txt"}},
		{name: "missing key", ref: Ref{Bucket: "objects"}},
		{name: "absolute key", ref: Ref{Bucket: "objects", Key: "/tenant/file.txt"}},
		{name: "parent traversal", ref: Ref{Bucket: "objects", Key: "tenant/../file.txt"}},
		{name: "backslash", ref: Ref{Bucket: "objects", Key: `tenant\file.txt`}},
		{name: "control character", ref: Ref{Bucket: "objects", Key: "tenant/\x00/file.txt"}},
		{name: "invalid bucket", ref: Ref{Bucket: "Bad_Bucket", Key: "tenant/file.txt"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateRef(tt.ref); !errors.Is(err, ErrInvalidRef) {
				t.Fatalf("ValidateRef() error = %v, want %v", err, ErrInvalidRef)
			}
		})
	}
}

func TestValidateRefAcceptsTenantScopedKeys(t *testing.T) {
	t.Parallel()

	ref := Ref{Bucket: "tenant-objects", Key: "org_123/widgets/avatar.png"}
	if err := ValidateRef(ref); err != nil {
		t.Fatalf("ValidateRef() error = %v", err)
	}
}

func TestPolicyValidatesSizeAndContentType(t *testing.T) {
	t.Parallel()

	policy := Policy{MaxObjectSize: 8, AllowedContentTypes: []string{"image/png", "application/json"}}
	if err := policy.ValidatePut(PutOptions{Size: 9, ContentType: "image/png"}); !errors.Is(err, ErrObjectTooLarge) {
		t.Fatalf("oversized error = %v, want %v", err, ErrObjectTooLarge)
	}
	if err := policy.ValidatePut(PutOptions{Size: 4, ContentType: "text/html"}); !errors.Is(err, ErrContentTypeNotAllowed) {
		t.Fatalf("content-type error = %v, want %v", err, ErrContentTypeNotAllowed)
	}
	if err := policy.ValidatePut(PutOptions{Size: 4, ContentType: " Application/JSON ; charset=utf-8 "}); err != nil {
		t.Fatalf("ValidatePut() error = %v", err)
	}
}

func TestCopyMetadataRejectsSecretShapedMetadata(t *testing.T) {
	t.Parallel()

	if _, err := SafeMetadata(map[string]string{"api_key": "redacted"}); !errors.Is(err, ErrUnsafeMetadata) {
		t.Fatalf("SafeMetadata() error = %v, want %v", err, ErrUnsafeMetadata)
	}
	metadata, err := SafeMetadata(map[string]string{"owner": "org_123"})
	if err != nil {
		t.Fatalf("SafeMetadata() error = %v", err)
	}
	metadata["owner"] = "changed"
	again, err := SafeMetadata(map[string]string{"owner": "org_123"})
	if err != nil {
		t.Fatalf("SafeMetadata() second error = %v", err)
	}
	if again["owner"] != "org_123" {
		t.Fatalf("metadata was not cloned")
	}
}

func TestReadAtMostEnforcesUnknownBodyLimit(t *testing.T) {
	t.Parallel()

	body, size, err := ReadAtMost(strings.NewReader("small"), 8)
	if err != nil {
		t.Fatalf("ReadAtMost() error = %v", err)
	}
	if size != 5 || string(body) != "small" {
		t.Fatalf("ReadAtMost() = (%q, %d), want small 5", body, size)
	}
	if _, _, err := ReadAtMost(strings.NewReader("too-large"), 4); !errors.Is(err, ErrObjectTooLarge) {
		t.Fatalf("oversized ReadAtMost() error = %v, want %v", err, ErrObjectTooLarge)
	}
	if _, _, err := ReadAtMost(errReader{}, 4); err == nil {
		t.Fatal("ReadAtMost() error = nil, want reader error")
	}
}

func TestSignedURLOptionsDefaults(t *testing.T) {
	t.Parallel()

	opts, err := NormalizeSignedURLOptions(SignedURLOptions{})
	if err != nil {
		t.Fatalf("NormalizeSignedURLOptions() error = %v", err)
	}
	if opts.Method != "GET" || opts.Expires != 15*time.Minute {
		t.Fatalf("NormalizeSignedURLOptions() = %#v", opts)
	}
	if _, err := NormalizeSignedURLOptions(SignedURLOptions{Method: "POST"}); !errors.Is(err, ErrInvalidSignedURL) {
		t.Fatalf("invalid method error = %v, want %v", err, ErrInvalidSignedURL)
	}
	if _, err := NormalizeSignedURLOptions(SignedURLOptions{Expires: 8 * 24 * time.Hour}); !errors.Is(err, ErrInvalidSignedURL) {
		t.Fatalf("invalid expiry error = %v, want %v", err, ErrInvalidSignedURL)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
